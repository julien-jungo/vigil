package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	URL        string           `yaml:"url"`
	Specs      string           `yaml:"specs"`
	LLM        LLMConfig        `yaml:"llm"`
	Playwright PlaywrightConfig `yaml:"playwright"`
	Reports    ReportsConfig    `yaml:"reports"`
	Explore    ExploreConfig    `yaml:"explore"`
}

type LLMConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
}

type PlaywrightConfig struct {
	Headless bool           `yaml:"headless"`
	Viewport ViewportConfig `yaml:"viewport"`
}

type ViewportConfig struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

type ReportsConfig struct {
	JUnit string `yaml:"junit"`
	HTML  string `yaml:"html"`
}

type ExploreConfig struct {
	MaxSteps int `yaml:"max_steps"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validate(cfg Config) error {
	var missing []string

	if cfg.URL == "" {
		missing = append(missing, "url")
	}
	if cfg.LLM.Provider == "" {
		missing = append(missing, "llm.provider")
	}
	if cfg.LLM.Model == "" {
		missing = append(missing, "llm.model")
	}
	if cfg.LLM.APIKey == "" {
		missing = append(missing, "llm.api_key")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required config fields: %s", strings.Join(missing, ", "))
	}

	return nil
}
