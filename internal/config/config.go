package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Layout     string `yaml:"layout"`
	BaseBranch string `yaml:"baseBranch"`
	Editor     string `yaml:"editor"`
	Agent      string `yaml:"agent"`
}

type fileConfig struct {
	Layout     *string `yaml:"layout"`
	BaseBranch *string `yaml:"baseBranch"`
	Editor     *string `yaml:"editor"`
	Agent      *string `yaml:"agent"`
}

func Load(start string) (Config, error) {
	defaults := Config{Layout: "sibling", BaseBranch: "main", Editor: "code", Agent: "claude"}
	for _, path := range configPaths(start) {
		data, err := os.ReadFile(path) // #nosec G304 -- config paths are fixed to the working directory, its parents, or the user config directory.
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Config{}, fmt.Errorf("read config %s: %w", path, err)
		}
		var raw fileConfig
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&raw); errors.Is(err, io.EOF) {
			return defaults, nil
		} else if err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", path, err)
		}
		if err := rejectNullFields(data); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", path, err)
		}
		var extra fileConfig
		if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
			if err != nil {
				return Config{}, fmt.Errorf("parse config %s: %w", path, err)
			}
			return Config{}, fmt.Errorf("parse config %s: multiple YAML documents", path)
		}
		cfg, err := apply(raw, defaults)
		if err != nil {
			return Config{}, fmt.Errorf("validate config %s: %w", path, err)
		}
		return cfg, nil
	}
	return defaults, nil
}

func rejectNullFields(data []byte) error {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return err
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		if len(document.Content) > 0 && document.Content[0].Tag == "!!null" {
			return errors.New("config cannot be null")
		}
		return nil
	}
	for i := 0; i < len(document.Content[0].Content); i += 2 {
		key, value := document.Content[0].Content[i], document.Content[0].Content[i+1]
		if (key.Value == "layout" || key.Value == "baseBranch" || key.Value == "editor" || key.Value == "agent") && value.Tag == "!!null" {
			return fmt.Errorf("%s cannot be null", key.Value)
		}
	}
	return nil
}

func configPaths(start string) []string {
	home, homeErr := os.UserHomeDir()
	paths := localConfigPaths(start, home)
	if dir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(dir, "gwt", "config.yml"))
	}
	if homeErr == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "gwt", "config.yml"),
			filepath.Join(home, "gwt.yml"),
		)
	}
	return paths
}

// localConfigPaths lists gwt.yml from start up to the filesystem root, nearest
// first. The walk stops below home so ~/gwt.yml keeps its place as the last
// fallback, unless start is home itself.
func localConfigPaths(start, home string) []string {
	var paths []string
	for dir := filepath.Clean(start); ; {
		paths = append(paths, filepath.Join(dir, "gwt.yml"))
		parent := filepath.Dir(dir)
		if parent == dir || (home != "" && parent == filepath.Clean(home)) {
			return paths
		}
		dir = parent
	}
}

func apply(raw fileConfig, defaults Config) (Config, error) {
	config := defaults
	if raw.Layout != nil {
		if *raw.Layout != "sibling" && *raw.Layout != "grouped" && *raw.Layout != "inside" && *raw.Layout != "branch" {
			return Config{}, fmt.Errorf("invalid layout %q", *raw.Layout)
		}
		config.Layout = *raw.Layout
	}
	if raw.BaseBranch != nil {
		if *raw.BaseBranch == "" {
			return Config{}, errors.New("baseBranch cannot be empty")
		}
		if err := exec.Command("git", "check-ref-format", "--branch", *raw.BaseBranch).Run(); err != nil { // #nosec G204 -- git and arguments are fixed except the validated branch candidate.
			return Config{}, fmt.Errorf("invalid baseBranch %q", *raw.BaseBranch)
		}
		config.BaseBranch = *raw.BaseBranch
	}
	if raw.Editor != nil {
		config.Editor = *raw.Editor
	}
	if raw.Agent != nil {
		config.Agent = *raw.Agent
	}
	return config, nil
}
