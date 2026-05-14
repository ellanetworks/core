package integration_test

import (
	"context"
	"os"
	"testing"
)

type apiMatrixHARunner func(ctx context.Context, t *testing.T, h *haMatrixEnv)

// apiMatrixHAResources mirrors apiMatrixResources but each runner sees
// the full 3-node cluster and is responsible for distributing writes
// across nodes plus asserting the cross-node invariant (replication for
// shared resources, locality for per-node resources).
var apiMatrixHAResources = map[string]apiMatrixHARunner{
	"subscribers": runSubscribersHAMatrix,
}

func TestAPIMatrixHA(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()
	h := setupHAMatrixEnv(ctx, t)

	for name, run := range apiMatrixHAResources {
		name, run := name, run
		t.Run(name, func(t *testing.T) {
			run(ctx, t, h)
		})
	}
}
