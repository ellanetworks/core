package config_test

import (
	"testing"

	"github.com/yeastengine/ella/internal/config"
)

func TestGivenBadYamlFormattingWhenParseThenReturnError(t *testing.T) {
	_, err := config.Parse("bad_config.yaml")
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestGivenBadConfigWhenValidateThenError(t *testing.T) {
	config := config.Config{
		DB: &config.DBConfig{
			Url: "", // empty url
		},
		UPF: &config.UPFConfig{
			Interfaces: []string{"lo"},
			N3Address:  "127.0.0.1",
		},
	}

	err := config.Validate()
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestGivenCorrectConfigFileWhenValidateThenNoError(t *testing.T) {
	config := config.Config{
		DB: &config.DBConfig{
			Url: "mongodb://localhost:27017",
		},
		UPF: &config.UPFConfig{
			Interfaces: []string{"lo"},
			N3Address:  "127.0.0.1",
		},
	}
	err := config.Validate()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
