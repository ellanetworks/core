package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runOperatorCodeMatrix exercises PUT /api/v1/operator/code. The
// endpoint is PUT-only — there is no GET counterpart — so this is a
// set-only matrix that verifies successful updates and the documented
// validation errors (api_operator.go:364-415):
//
//   - 32-character hex string required
//   - rejected when any subscribers exist
//
// The runner restores a known-good code on cleanup so the operator is
// in a consistent state for any subsequent test or run. Since the
// operator code can't be updated while subscribers exist, this runner
// is sensitive to leaked subscriber state from prior runs; the matrix
// driver runs subtests sequentially so as long as no other subtest
// leaks, the constraint is satisfied here.
func runOperatorCodeMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	const (
		// Two valid 32-char hex codes — well-known test vectors, no
		// security significance.
		codeA = "0123456789abcdef0123456789abcdef"
		codeB = "fedcba9876543210fedcba9876543210"
	)

	if err := c.UpdateOperatorCode(ctx, &client.UpdateOperatorCodeOptions{OperatorCode: codeA}); err != nil {
		t.Fatalf("set operator code baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := c.UpdateOperatorCode(ctx, &client.UpdateOperatorCodeOptions{OperatorCode: codeA}); err != nil {
			t.Logf("cleanup: restore operator code: %v", err)
		}
	})

	t.Run("update_to_alternate_code", func(t *testing.T) {
		if err := c.UpdateOperatorCode(ctx, &client.UpdateOperatorCodeOptions{OperatorCode: codeB}); err != nil {
			t.Fatalf("update operator code to alternate: %v", err)
		}
	})

	// Validation negatives — each must fail with a 4xx server error.
	negatives := []struct {
		name string
		code string
		want string
	}{
		{
			name: "empty",
			code: "",
			want: "operator code is missing",
		},
		{
			name: "wrong_length_31",
			code: "0123456789abcdef0123456789abcde",
			want: "32-character hexadecimal string",
		},
		{
			name: "wrong_length_33",
			code: "0123456789abcdef0123456789abcdef0",
			want: "32-character hexadecimal string",
		},
		{
			name: "not_hex",
			code: "0123456789abcdef0123456789abcdez",
			want: "32-character hexadecimal string",
		},
	}

	for _, n := range negatives {
		n := n
		t.Run("negative_"+n.name, func(t *testing.T) {
			err := c.UpdateOperatorCode(ctx, &client.UpdateOperatorCodeOptions{OperatorCode: n.code})
			if err == nil {
				t.Fatalf("expected error, got none")
			}

			msg := err.Error()
			if !strings.Contains(msg, n.want) {
				t.Fatalf("error message: got %q, want substring %q", msg, n.want)
			}

			if !strings.Contains(msg, "400") {
				t.Fatalf("expected 400 status, got %q", msg)
			}
		})
	}
}
