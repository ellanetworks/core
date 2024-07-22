package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	MongoDBBinariesPath string `yaml:"mongoDBBinariesPath"`
	DbPath              string `yaml:"dbPath"`
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

func (cfg *Config) Validate() error {
	if cfg.MongoDBBinariesPath == "" {
		return fmt.Errorf("mongoDBBinariesPath is required")
	}
	if _, err := os.Stat(cfg.MongoDBBinariesPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", cfg.MongoDBBinariesPath)
	}
	if cfg.DbPath == "" {
		return fmt.Errorf("dbPath is required")
	}
	if _, err := os.Stat(cfg.DbPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", cfg.DbPath)
	}
	return nil
}
