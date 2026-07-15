// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
)

func TestSubscriberUsageLimitParam(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	cases := []struct {
		name   string
		url    string
		status int
	}{
		{"absent limit is allowed", "/api/v1/subscriber-usage?group_by=subscriber", http.StatusOK},
		{"valid limit", "/api/v1/subscriber-usage?group_by=subscriber&limit=10", http.StatusOK},
		{"limit at cap", "/api/v1/subscriber-usage?group_by=subscriber&limit=1000", http.StatusOK},
		{"limit above cap rejected", "/api/v1/subscriber-usage?group_by=subscriber&limit=1001", http.StatusBadRequest},
		{"zero limit rejected", "/api/v1/subscriber-usage?group_by=subscriber&limit=0", http.StatusBadRequest},
		{"negative limit rejected", "/api/v1/subscriber-usage?group_by=subscriber&limit=-1", http.StatusBadRequest},
		{"non-numeric limit rejected", "/api/v1/subscriber-usage?group_by=subscriber&limit=abc", http.StatusBadRequest},
		{"limit with group_by=day rejected", "/api/v1/subscriber-usage?group_by=day&limit=10", http.StatusBadRequest},
		{"group_by=day without limit still works", "/api/v1/subscriber-usage?group_by=day", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(), "GET", env.Server.URL+tc.url, nil)
			if err != nil {
				t.Fatalf("req: %s", err)
			}

			req.Header.Set("Authorization", "Bearer "+token)

			res, err := client.Do(req)
			if err != nil {
				t.Fatalf("do: %s", err)
			}

			defer func() {
				if err := res.Body.Close(); err != nil {
					t.Fatalf("close: %s", err)
				}
			}()

			if res.StatusCode != tc.status {
				t.Errorf("expected %d, got %d", tc.status, res.StatusCode)
			}
		})
	}
}
