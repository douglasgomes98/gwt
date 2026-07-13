package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Layout     string `yaml:"layout"`
	BaseBranch string `yaml:"baseBranch"`
	Editor     string `yaml:"editor"`
	Agent      string `yaml:"agent"`
}

func Load(start string) Config {
	c := Config{Layout: "sibling", BaseBranch: "main", Editor: "code"}
	paths := []string{filepath.Join(start, "gwt.yml")}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config/gwt/config.yml"))
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		_ = yaml.Unmarshal(b, &c)
		break
	}
	if c.Layout == "" {
		c.Layout = "sibling"
	}
	if c.BaseBranch == "" {
		c.BaseBranch = "main"
	}
	return c
}
