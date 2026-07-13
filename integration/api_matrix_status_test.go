// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runStatusMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	got, err := c.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}

	if !got.Initialized {
		t.Fatalf("Initialized: got false, want true")
	}

	if got.Version == "" {
		t.Fatalf("Version: got empty, want non-empty")
	}
}
