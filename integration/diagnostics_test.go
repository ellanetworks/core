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

const metricsScrapeTimeout = 10 * time.Second

func captureMetrics(t *testing.T, baseURL, name string) {
	t.Helper()

	root := os.Getenv("INTEGRATION_LOG_DIR")
	if root == "" || !t.Failed() {
		return
	}

	body, err := scrapeMetrics(baseURL)
	if err != nil {
		t.Logf("captureMetrics: scrape %s: %v", baseURL, err)
		return
	}

	dir := filepath.Join(root, sanitizeTestName(t.Name()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("captureMetrics: mkdir %s: %v", dir, err)
		return
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Logf("captureMetrics: write %s: %v", path, err)
	}
}

func scrapeMetrics(baseURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metricsScrapeTimeout)
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

const composeLogsTimeout = 2 * time.Minute

func captureServiceLogs(t *testing.T, dc *DockerClient, composeDir string, services []string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), composeLogsTimeout)
	defer cancel()

	var diskDir string

	if root := os.Getenv("INTEGRATION_LOG_DIR"); root != "" {
		diskDir = filepath.Join(root, sanitizeTestName(t.Name()))
		if err := os.MkdirAll(diskDir, 0o755); err != nil {
			t.Logf("captureServiceLogs: mkdir %s: %v", diskDir, err)

			diskDir = ""
		}
	}

	for _, svc := range services {
		logs, err := dc.ComposeLogs(ctx, composeDir, svc)
		if err != nil {
			if t.Failed() {
				t.Logf("=== %s logs: collection failed: %v ===", svc, err)
			}

			continue
		}

		if t.Failed() {
			t.Logf("=== %s logs ===\n%s", svc, logs)
		}

		if diskDir == "" {
			continue
		}

		path := filepath.Join(diskDir, svc+".log")
		if err := os.WriteFile(path, []byte(logs), 0o644); err != nil {
			t.Logf("captureServiceLogs: write %s: %v", path, err)
		}
	}
}
