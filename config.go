package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

var ConfigPath = "config.yaml"

type Config struct {
	Repositories []Repository `yaml:"repositories"`
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
