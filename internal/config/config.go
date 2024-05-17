package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Database struct {
	Name           string `yaml:"name"`
	Url            string `yaml:"url"`
	AuthKeysDbName string `yaml:"authKeysDbName"`
	AuthUrl        string `yaml:"authUrl"`
}

type Config struct {
	Database Database `yaml:"database"`
}

func Parse(configPath string) (Config, error) {

	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}
