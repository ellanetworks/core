// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runSupportBundleMatrix exercises the support bundle endpoint. The server
// streams a gzipped tar, which the client writes to the given path.
func runSupportBundleMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	bundlePath := filepath.Join(t.TempDir(), "apimat-support-bundle.tar.gz")

	if err := c.GenerateSupportBundle(ctx, &client.GenerateSupportBundleParams{Path: bundlePath}); err != nil {
		t.Fatalf("generate support bundle: %v", err)
	}

	info, err := os.Stat(bundlePath)
	if err != nil {
		t.Fatalf("stat support bundle file: %v", err)
	}

	if info.Size() == 0 {
		t.Fatalf("support bundle file is empty")
	}
}
