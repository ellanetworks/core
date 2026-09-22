// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"path/filepath"
	"testing"
)

type clusterHealthResult struct {
	State            string `json:"state"`
	TotalVoters      int    `json:"totalVoters"`
	HealthyVoters    *int   `json:"healthyVoters,omitempty"`
	FailureTolerance *int   `json:"failureTolerance,omitempty"`
}

type clusterHealthResponse struct {
	Error  string              `json:"error,omitempty"`
	Result clusterHealthResult `json:"result"`
}

func getClusterHealth(url string, client *http.Client, token string) (int, *clusterHealthResponse, error) {
	return apiDo[clusterHealthResponse](client, "GET", url+"/api/v1/cluster/health", token, nil)
}

func TestGetClusterHealth_SingleServer(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	status, body, err := getClusterHealth(env.Server.URL, client, token)
	if err != nil {
		t.Fatalf("couldn't get cluster health: %s", err)
	}

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %+v", status, body)
	}

	if body.Result.State != "healthy" {
		t.Errorf("expected state=healthy, got %q", body.Result.State)
	}

	if body.Result.TotalVoters != 1 {
		t.Errorf("expected totalVoters=1, got %d", body.Result.TotalVoters)
	}

	if body.Result.HealthyVoters == nil || *body.Result.HealthyVoters != 1 {
		t.Errorf("expected healthyVoters=1, got %v", body.Result.HealthyVoters)
	}

	if body.Result.FailureTolerance == nil || *body.Result.FailureTolerance != 0 {
		t.Errorf("expected failureTolerance=0, got %v", body.Result.FailureTolerance)
	}
}

func TestGetClusterHealth_Unauthorized(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	if _, err := initializeAndRefresh(env.Server.URL, client); err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	status, _, err := getClusterHealth(env.Server.URL, client, "")
	if err != nil {
		t.Fatalf("request failed: %s", err)
	}

	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", status)
	}
}

func TestGetClusterHealth_ReadableByEveryRole(t *testing.T) {
	env, _, adminToken := newAuthedTestEnv(t)

	for _, tc := range []struct {
		name   string
		email  string
		roleID RoleID
	}{
		{name: "read only", email: "health-readonly@ellanetworks.com", roleID: RoleReadOnly},
		{name: "network manager", email: "health-netmanager@ellanetworks.com", roleID: RoleNetworkManager},
	} {
		t.Run(tc.name, func(t *testing.T) {
			roleClient := newTestClient(env.Server)

			roleToken, err := createUserAndLogin(env.Server.URL, adminToken, tc.email, tc.roleID, roleClient)
			if err != nil {
				t.Fatalf("couldn't create %s user: %s", tc.name, err)
			}

			status, body, err := getClusterHealth(env.Server.URL, roleClient, roleToken)
			if err != nil {
				t.Fatalf("couldn't get cluster health: %s", err)
			}

			if status != http.StatusOK {
				t.Errorf("expected 200 for a %s user, got %d: %+v", tc.name, status, body)
			}

			statusCode, _, err := getAutopilotState(env.Server.URL, roleClient, roleToken)
			if err != nil {
				t.Fatalf("couldn't get autopilot state: %s", err)
			}

			if statusCode != http.StatusForbidden {
				t.Errorf("expected 403 on autopilot for a %s user, got %d", tc.name, statusCode)
			}
		})
	}
}
