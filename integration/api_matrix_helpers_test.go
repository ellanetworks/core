package integration_test

import (
	"strings"
	"testing"
)

// apiMatrixName prefixes resource names so they're identifiable as matrix
// state and so a collision from a leaked prior run surfaces immediately.
func apiMatrixName(resource string) string {
	return "apimat-" + resource
}

func assertNotFound(t *testing.T, err error, what string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %s lookup to fail with not-found, got nil", what)
	}

	msg := err.Error()
	if !strings.Contains(msg, "404") && !strings.Contains(strings.ToLower(msg), "not found") {
		t.Fatalf("expected %s lookup to fail with not-found, got: %v", what, err)
	}
}
