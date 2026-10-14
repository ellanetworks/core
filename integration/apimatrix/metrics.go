// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package apimatrix

import (
	"context"
	"testing"
)

func runMetricsMatrix(ctx context.Context, t *testing.T, env *Env) {
	c := env.Client

	got, err := c.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}

	if len(got) == 0 {
		t.Fatalf("metrics: got 0 series, want at least one")
	}
}
