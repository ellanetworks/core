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
)

func TestCreateAuthDataBadSuci(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	testdb, err := db.NewDatabase(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	ausfInst := ausf.NewAUSF(testdb)

	authInfoResult, err := ausfInst.CreateAuthData(context.Background(), "", nil, "123")
	if err == nil {
		t.Fatalf("failed to create auth data: %v", err)
	}

	if authInfoResult != nil {
		t.Fatalf("auth data should be nil")
	}
}
