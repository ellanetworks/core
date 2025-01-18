// Copyright 2024 Ella Networks

package config_test

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/config"
)

func TestGoodConfigSuccess(t *testing.T) {
	// Create temporary cert and key files
	tempCertFile, err := os.CreateTemp("", "ella_cert_*.crt")
	if err != nil {
		t.Fatalf("Failed to create temp cert file: %s", err)
	}
	defer func() {
		if err := os.Remove(tempCertFile.Name()); err != nil {
			log.Fatalf("Failed to remove temp key file: %v", err)
		}
	}()

	tempKeyFile, err := os.CreateTemp("", "ella_key_*.key")
	if err != nil {
		t.Fatalf("Failed to create temp key file: %s", err)
	}
	defer func() {
		if err := os.Remove(tempKeyFile.Name()); err != nil {
			log.Fatalf("Failed to remove temp key file: %v", err)
		}
	}()

	if _, err := tempCertFile.WriteString("dummy cert data"); err != nil {
		t.Fatalf("Failed to write to temp cert file: %s", err)
	}
	if _, err := tempKeyFile.WriteString("dummy key data"); err != nil {
		t.Fatalf("Failed to write to temp key file: %s", err)
	}

	defer func() {
		if err := tempCertFile.Close(); err != nil {
			log.Fatalf("Faile to close temp cert file: %v", err)
		}
	}()
	defer func() {
		if err := tempKeyFile.Close(); err != nil {
			log.Fatalf("Faile to close temp key file: %v", err)
		}
	}()

	config.CheckInterfaceExistsWithAddress = func(name string, address string) (bool, error) {
		return true, nil
	}

	// Update the config file to use the temporary cert and key paths
	confFilePath := "testdata/valid.yaml"
	originalContent, err := os.ReadFile(confFilePath)
	if err != nil {
		t.Fatalf("Failed to read config file: %s", err)
	}

	fmt.Println("Temp file name: ", tempCertFile.Name())

	updatedContent := strings.ReplaceAll(string(originalContent), "/etc/ssl/certs/ella.crt", tempCertFile.Name())
	updatedContent = strings.ReplaceAll(updatedContent, "/etc/ssl/private/ella.key", tempKeyFile.Name())

	err = os.WriteFile(confFilePath, []byte(updatedContent), os.FileMode(0o644))
	if err != nil {
		t.Fatalf("Failed to update config file: %s", err)
	}
	defer func() {
		if err := os.WriteFile(confFilePath, originalContent, os.FileMode(0o644)); err != nil {
			log.Fatalf("Failed to close database: %v", err)
		}
	}()

	// Run the validation
	conf, err := config.Validate(confFilePath)
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

	if conf.Interfaces.API.Port != 5002 {
		t.Fatalf("API port was not configured correctly")
	}

	if conf.Interfaces.API.TLS.Cert != tempCertFile.Name() {
		t.Fatalf("TLS cert was not configured correctly")
	}

	if conf.Interfaces.API.TLS.Key != tempKeyFile.Name() {
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
