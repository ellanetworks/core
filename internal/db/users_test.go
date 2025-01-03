package db_test

import (
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDBUsersEndToEnd(t *testing.T) {
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

	res, err := database.ListUsers()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if len(res) != 0 {
		t.Fatalf("One or more users were found in DB")
	}

	user := &db.User{
		Email:          "my.user123@ellanetworks.com",
		HashedPassword: "my-hashed-password",
	}
	err = database.CreateUser(user)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, err = database.ListUsers()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}
	if len(res) != 1 {
		t.Fatalf("One or more users weren't found in DB")
	}

	retrievedUser, err := database.GetUser(user.Email)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedUser.Email != user.Email {
		t.Fatalf("The user from the database doesn't match the user that was given")
	}

	if err = database.DeleteUser(user.Email); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, _ = database.ListUsers()
	if len(res) != 0 {
		t.Fatalf("Users weren't deleted from the DB properly")
	}
}
