// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

const datapathRevisionTimeout = 30 * time.Second

func awaitDatapath(ctx context.Context, t *testing.T, clients ...*client.Client) {
	t.Helper()

	if err := waitForDatapath(ctx, clients...); err != nil {
		t.Fatalf("%v", err)
	}
}

func waitForDatapath(ctx context.Context, clients ...*client.Client) error {
	var target uint64

	for _, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil {
			return fmt.Errorf("get status: %w", err)
		}

		target = max(target, status.Cluster.AppliedIndex)
	}

	deadline := time.Now().Add(datapathRevisionTimeout)

	for {
		behind := 0

		var lastErr error

		for _, c := range clients {
			status, err := c.GetStatus(ctx)
			if err != nil {
				lastErr = err
				behind++

				continue
			}

			if !status.Ready || status.DatapathRevision < target {
				behind++
			}
		}

		if behind == 0 {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%d of %d nodes did not apply revision %d to their datapath within %v (last error: %v)", behind, len(clients), target, datapathRevisionTimeout, lastErr)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
