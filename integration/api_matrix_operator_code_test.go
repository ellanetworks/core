package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runOperatorCodeMatrix sets the operator code via the set-only PUT
// endpoint. The server rejects the update when any subscribers exist,
// so this runner must not run concurrently with the subscribers matrix.
func runOperatorCodeMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	const (
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
