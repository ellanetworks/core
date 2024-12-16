package config_test

import (
	"strings"
	"testing"

	"github.com/yeastengine/ella/internal/config"
)

func TestGoodConfigSuccess(t *testing.T) {
	conf, err := config.Validate("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Error occurred: %s", err)
	}

	if conf.Interfaces.N3.Name != "enp3s0" {
		t.Fatalf("N3 interface was not configured correctly")
	}

	if conf.Interfaces.N3.Address != "127.0.0.1" {
		t.Fatalf("N3 interface address was not configured correctly")
	}

	if conf.Interfaces.N6.Name != "enp6s0" {
		t.Fatalf("N6 interface was not configured correctly")
	}

	if conf.Interfaces.API.Name != "enp0s8" {
		t.Fatalf("API interface was not configured correctly")
	}

	if conf.Interfaces.API.Port != 5000 {
		t.Fatalf("API port was not configured correctly")
	}

	if conf.Interfaces.API.TLS.Cert != "/etc/ssl/certs/ella.crt" {
		t.Fatalf("TLS cert was not configured correctly")
	}

	if conf.Interfaces.API.TLS.Key != "/etc/ssl/private/ella.key" {
		t.Fatalf("TLS key was not configured correctly")
	}

	if conf.DB.Path != "test" {
		t.Fatalf("Database path was not configured correctly")
	}
}

func TestBadConfigFail(t *testing.T) {
	cases := []struct {
		Name               string
		ConfigYAMLFilePath string
		ExpectedError      string
	}{
		{"no db", "testdata/invalid_no_db.yaml", "db is empty"},
		{"invalid yaml", "testdata/invalid_yaml.yaml", "cannot unmarshal config file"},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := config.Validate(tc.ConfigYAMLFilePath)
			if err == nil {
				t.Fatalf("Expected error, got nil")
			}

			if !strings.Contains(err.Error(), tc.ExpectedError) {
				t.Errorf("Expected error: %s, got: %s", tc.ExpectedError, err)
			}
		})
	}
}
