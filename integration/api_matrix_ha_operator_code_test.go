package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runOperatorCodeHAMatrix exercises the operator-code setter across all
// nodes. The server rejects the update when any subscribers exist, so
// this runner must not interleave with the subscribers HA matrix; the
// driver runs subtests sequentially and each cleans up before the next
// starts, which is sufficient.
func runOperatorCodeHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	const (
		codeA = "0123456789abcdef0123456789abcdef"
		codeB = "fedcba9876543210fedcba9876543210"
	)

	if err := h.Leader.UpdateOperatorCode(ctx, &client.UpdateOperatorCodeOptions{OperatorCode: codeA}); err != nil {
		t.Fatalf("set operator code baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := h.Leader.UpdateOperatorCode(ctx, &client.UpdateOperatorCodeOptions{OperatorCode: codeA}); err != nil {
			t.Logf("cleanup: restore operator code: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	t.Run("update_to_alternate_code", func(t *testing.T) {
		if err := nodes[1].UpdateOperatorCode(ctx, &client.UpdateOperatorCodeOptions{OperatorCode: codeB}); err != nil {
			t.Fatalf("update operator code on node 2: %v", err)
		}

		awaitConvergence(ctx, t, h)
	})

	negatives := []struct {
		name   string
		writer int
		code   string
		want   string
	}{
		{name: "empty", writer: 0, code: "", want: "operator code is missing"},
		{name: "wrong_length_31", writer: 1, code: "0123456789abcdef0123456789abcde", want: "32-character hexadecimal string"},
		{name: "wrong_length_33", writer: 2, code: "0123456789abcdef0123456789abcdef0", want: "32-character hexadecimal string"},
		{name: "not_hex", writer: 0, code: "0123456789abcdef0123456789abcdez", want: "32-character hexadecimal string"},
	}

	for _, n := range negatives {
		n := n
		t.Run("negative_"+n.name, func(t *testing.T) {
			err := nodes[n.writer].UpdateOperatorCode(ctx, &client.UpdateOperatorCodeOptions{OperatorCode: n.code})
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
