// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const datapathMetricsTimeout = 10 * time.Second

// captureDatapathMetrics writes the node's Prometheus metrics beside the
// container logs, so a failed run records what the data plane did with every
// frame. app_upf_datapath_forward_total and app_upf_datapath_drop_total count
// each frame exactly once between them, so they distinguish a packet the data
// plane dropped (and why) from one that never reached it at all.
//
// Best-effort: a scrape failure is logged and never fails the test. Must run
// before the stack is torn down.
func captureDatapathMetrics(t *testing.T, baseURL, name string) {
	t.Helper()

	root := os.Getenv("INTEGRATION_LOG_DIR")
	if root == "" || !t.Failed() {
		return
	}

	body, err := scrapeMetrics(baseURL)
	if err != nil {
		t.Logf("captureDatapathMetrics: scrape %s: %v", baseURL, err)
		return
	}

	dir := filepath.Join(root, sanitizeTestName(t.Name()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("captureDatapathMetrics: mkdir %s: %v", dir, err)
		return
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Logf("captureDatapathMetrics: write %s: %v", path, err)
	}
}

func scrapeMetrics(baseURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), datapathMetricsTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/metrics", nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}
