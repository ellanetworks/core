// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDBAPITokensEndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			panic(err)
		}
	}()

	user := &db.User{
		Email:  "abc@ellanetworks.com",
		RoleID: db.RoleAdmin,
	}
	err = database.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("Couldn't create user: %s", err)
	}

	res, total, err := database.ListUsersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't list users: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1, but got %d", total)
	}

	if len(res) != 1 {
		t.Fatalf("One or more users weren't found in DB")
	}

	userID := res[0].ID

	resList, total, err := database.ListAPITokensPage(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("Couldn't list API tokens: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(resList) != 0 {
		t.Fatalf("One or more users were found in DB")
	}

	token := &db.APIToken{
		Name:   "whatever token",
		UserID: userID,
	}

	err = database.CreateAPIToken(context.Background(), token)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	resList, total, err = database.ListAPITokensPage(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("Couldn't list API tokens: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1, but got %d", total)
	}

	if len(resList) != 1 {
		t.Fatalf("One or more API tokens weren't found in DB")
	}

	if resList[0].Name != token.Name {
		t.Fatalf("The API token from the database doesn't match the API token that was given")
	}

	err = database.DeleteAPIToken(context.Background(), resList[0].ID)
	if err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}

	resList, total, err = database.ListAPITokensPage(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("Couldn't list API tokens: %s", err)
	}

	if total != 0 {
		t.Fatalf("API tokens weren't deleted from the DB properly")
	}

	if len(resList) != 0 {
		t.Fatalf("API tokens weren't deleted from the DB properly")
	}
}
