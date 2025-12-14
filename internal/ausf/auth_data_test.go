// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package ausf_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
)

func TestCreateAuthDataBadSuci(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	testdb, err := db.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	_ = ausf.Start(testdb)
	authInfoRequest := models.AuthenticationInfoRequest{}
	ueSuci := "123"
	authInfoResult, err := ausf.CreateAuthData(context.Background(), authInfoRequest, ueSuci)
	if err == nil {
		t.Fatalf("failed to create auth data: %v", err)
	}
	if authInfoResult != nil {
		t.Fatalf("auth data should be nil")
	}
}
