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
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Config{}, fmt.Errorf("read config %s: %w", path, err)
		}
		var raw fileConfig
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&raw); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", path, err)
		}
		if err := rejectNullFields(data); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", path, err)
		}
		var extra fileConfig
		if err := dec.Decode(&extra); err != io.EOF {
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
	paths := []string{filepath.Join(start, "gwt.yml")}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config/gwt/config.yml"))
	}
	return paths
}

func apply(raw fileConfig, defaults Config) (Config, error) {
	config := defaults
	if raw.Layout != nil {
		if *raw.Layout != "sibling" && *raw.Layout != "grouped" && *raw.Layout != "inside" {
			return Config{}, fmt.Errorf("invalid layout %q", *raw.Layout)
		}
		config.Layout = *raw.Layout
	}
	if raw.BaseBranch != nil {
		if *raw.BaseBranch == "" {
			return Config{}, errors.New("baseBranch cannot be empty")
		}
		if err := exec.Command("git", "check-ref-format", "--branch", *raw.BaseBranch).Run(); err != nil {
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
