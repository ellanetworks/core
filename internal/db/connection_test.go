package db_test

import (
	"testing"

	"github.com/yeastengine/ella/internal/db"
)

func TestGivenNoDBWhenTestConnectionThenReturnError(t *testing.T) {
	err := db.TestConnection("mongodb://1.2.3.4:12345")
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}
