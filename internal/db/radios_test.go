package db_test

import (
	"path/filepath"
	"testing"

	"github.com/yeastengine/ella/internal/db"
)

func TestDBRadiosEndToEnd(t *testing.T) {
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

	res, err := database.ListRadios()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if len(res) != 0 {
		t.Fatalf("One or more radios were found in DB")
	}

	radio := &db.Radio{
		Name: "my-radio",
		Tac:  "001",
	}
	err = database.CreateRadio(radio)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, err = database.ListRadios()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}
	if len(res) != 1 {
		t.Fatalf("One or more radios weren't found in DB")
	}

	retrievedRadio, err := database.GetRadio(radio.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedRadio.Name != radio.Name {
		t.Fatalf("The radio from the database doesn't match the radio that was given")
	}

	if err = database.DeleteRadio(radio.Name); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, _ = database.ListRadios()
	if len(res) != 0 {
		t.Fatalf("Radios weren't deleted from the DB properly")
	}
}
