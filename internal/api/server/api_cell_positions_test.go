// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func createCellPosition(url string, client *http.Client, token, body string) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/beta/cell-positions", strings.NewReader(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	defer func() { _ = res.Body.Close() }()

	return res.StatusCode, nil
}

// TestCreateCellPositionAuditLog verifies that provisioning a cell position
// records an audit-log entry identifying the actor and the cell.
func TestCreateCellPositionAuditLog(t *testing.T) {
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
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	body := `{"rat":"nr","mcc":"001","mnc":"01","cell_identity":"00066c","latitude":45.62,"longitude":-73.73}`

	status, err := createCellPosition(env.Server.URL, client, token, body)
	if err != nil {
		t.Fatalf("couldn't create cell position: %s", err)
	}

	if status != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, status)
	}

	_, auditResp, err := listAuditLogs(env.Server.URL, client, token, 1, 100)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	var found bool

	for _, entry := range auditResp.Result.Items {
		if entry.Action != "create_cell_position" {
			continue
		}

		found = true

		if !strings.Contains(entry.Details, "nr 001/01 00066c") {
			t.Errorf("audit details missing cell descriptor: %q", entry.Details)
		}

		break
	}

	if !found {
		t.Fatal("no create_cell_position audit entry found")
	}
}
