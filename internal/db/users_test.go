// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDBUsersEndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			panic(err)
		}
	}()

	res, total, err := database.ListUsersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("One or more users were found in DB")
	}

	user := &db.User{
		Email:          "my.user123@ellanetworks.com",
		HashedPassword: "my-hashed-password",
	}

	userID, err := database.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	if userID == 0 {
		t.Fatalf("Expected user ID to be non-zero after creation")
	}

	res, total, err = database.ListUsersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1, but got %d", total)
	}

	if len(res) != 1 {
		t.Fatalf("One or more users weren't found in DB")
	}

	retrievedUser, err := database.GetUser(context.Background(), user.Email)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedUser.Email != user.Email {
		t.Fatalf("The user from the database doesn't match the user that was given")
	}

	if err = database.DeleteUser(context.Background(), user.Email); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, total, _ = database.ListUsersPage(context.Background(), 1, 10)

	if total != 0 {
		t.Fatalf("Users weren't deleted from the DB properly")
	}

	if len(res) != 0 {
		t.Fatalf("Users weren't deleted from the DB properly")
	}
}
