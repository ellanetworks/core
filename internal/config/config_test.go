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

	if conf.UPF.Interfaces == nil {
		t.Fatalf("Interfaces was not configured correctly")
	}

	if conf.UPF.Interfaces[0] != "eth0" {
		t.Fatalf("Interfaces was not configured correctly")
	}

	if conf.DB.Mongo.Name != "test" {
		t.Fatalf("Database name was not configured correctly")
	}

	if conf.DB.Sql.Path != "testdata/sqlite.db" {
		t.Fatalf("Database path was not configured correctly")
	}

	if conf.TLS.Cert == nil {
		t.Fatalf("TLS cert was not configured correctly")
	}

	if conf.TLS.Key == nil {
		t.Fatalf("TLS key was not configured correctly")
	}
}

func TestBadConfigFail(t *testing.T) {
	cases := []struct {
		Name               string
		ConfigYAMLFilePath string
		ExpectedError      string
	}{
		{"no db", "testdata/invalid_no_db.yaml", "db is empty"},
		{"invalid yaml", "testdata/invalid_yaml.yaml", "unmarshal errors"},
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
