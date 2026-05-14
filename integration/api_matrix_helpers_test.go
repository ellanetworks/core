package integration_test

import (
	"strings"
	"testing"
)

// apiMatrixName returns a deterministic resource name scoped to the matrix
// suite. Strict-create semantics (matching integration/fixture/apply.go) mean
// a collision from a prior crashed run surfaces as a test failure rather than
// silent mutation.
func apiMatrixName(resource string) string {
	return "apimat-" + resource
}

// assertNotFound fails the test if err is nil, or if err does not look like
// the server's not-found response. The Go SDK surfaces server errors as
// `server error <code>: <message>` (client/client.go:243-249), so we match
// on both the 404 status and the "not found" text.
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
