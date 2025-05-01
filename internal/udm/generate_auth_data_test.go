package udm_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
)

func TestCreateAuthDataBadSuci(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	initialOperator := db.Operator{}
	testdb, err := db.NewDatabase(dbPath, initialOperator)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	udm.SetDBInstance(testdb)
	authInfoRequest := models.AuthenticationInfoRequest{}
	ueSuci := "123"
	authInfoResult, err := udm.CreateAuthData(authInfoRequest, ueSuci, context.Background())
	if err == nil {
		t.Fatalf("failed to create auth data: %v", err)
	}
	if authInfoResult != nil {
		t.Fatalf("auth data should be nil")
	}
}
