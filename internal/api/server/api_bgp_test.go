package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
)

// BGP Settings response types

type GetBGPSettingsResponseResult struct {
	Enabled       bool   `json:"enabled"`
	LocalAS       int    `json:"localAS"`
	RouterID      string `json:"routerID"`
	ListenAddress string `json:"listenAddress"`
}

type GetBGPSettingsResponse struct {
	Result GetBGPSettingsResponseResult `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

type UpdateBGPSettingsParams struct {
	Enabled       bool   `json:"enabled"`
	LocalAS       int    `json:"localAS"`
	RouterID      string `json:"routerID"`
	ListenAddress string `json:"listenAddress,omitempty"`
}

type UpdateBGPSettingsResponseResult struct {
	Message string `json:"message"`
}

type UpdateBGPSettingsResponse struct {
	Result UpdateBGPSettingsResponseResult `json:"result"`
	Error  string                          `json:"error,omitempty"`
}

// BGP Peers response types

type BGPImportPrefixResult struct {
	Prefix    string `json:"prefix"`
	MaxLength int    `json:"maxLength"`
}

type BGPPeerResult struct {
	ID               int                     `json:"id"`
	Address          string                  `json:"address"`
	RemoteAS         int                     `json:"remoteAS"`
	HoldTime         int                     `json:"holdTime"`
	Password         string                  `json:"password"`
	Description      string                  `json:"description"`
	ImportPrefixes   []BGPImportPrefixResult `json:"importPrefixes"`
	State            string                  `json:"state,omitempty"`
	Uptime           string                  `json:"uptime,omitempty"`
	PrefixesSent     int                     `json:"prefixesSent,omitempty"`
	PrefixesReceived int                     `json:"prefixesReceived,omitempty"`
	PrefixesAccepted int                     `json:"prefixesAccepted,omitempty"`
}

type ListBGPPeersResponseResult struct {
	Items      []BGPPeerResult `json:"items"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	TotalCount int             `json:"total_count"`
}

type ListBGPPeersResponse struct {
	Result ListBGPPeersResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

type GetBGPPeerResponse struct {
	Result BGPPeerResult `json:"result"`
	Error  string        `json:"error,omitempty"`
}

type CreateBGPPeerParams struct {
	Address        string                  `json:"address"`
	RemoteAS       int                     `json:"remoteAS"`
	HoldTime       int                     `json:"holdTime"`
	Password       string                  `json:"password,omitempty"`
	Description    string                  `json:"description"`
	ImportPrefixes []BGPImportPrefixResult `json:"importPrefixes,omitempty"`
}

type CreateBGPPeerResponseResult struct {
	Message string `json:"message"`
}

type CreateBGPPeerResponse struct {
	Result CreateBGPPeerResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

type UpdateBGPPeerTestParams struct {
	Address        string                  `json:"address"`
	RemoteAS       int                     `json:"remoteAS"`
	HoldTime       int                     `json:"holdTime"`
	Password       string                  `json:"password,omitempty"`
	Description    string                  `json:"description"`
	ImportPrefixes []BGPImportPrefixResult `json:"importPrefixes,omitempty"`
}

type UpdateBGPPeerResponseResult struct {
	Message string `json:"message"`
}

type UpdateBGPPeerResponse struct {
	Result UpdateBGPPeerResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

type DeleteBGPPeerResponseResult struct {
	Message string `json:"message"`
}

type DeleteBGPPeerResponse struct {
	Result DeleteBGPPeerResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

// Helper functions

func getBGPSettings(url string, client *http.Client, token string) (int, *GetBGPSettingsResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/bgp", nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var resp GetBGPSettingsResponse

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func updateBGPSettings(url string, client *http.Client, token string, params *UpdateBGPSettingsParams) (int, *UpdateBGPSettingsResponse, error) {
	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/networking/bgp", bytes.NewReader(payloadBytes))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var resp UpdateBGPSettingsResponse

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func listBGPPeers(url string, client *http.Client, token string) (int, *ListBGPPeersResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/bgp/peers", nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var resp ListBGPPeersResponse

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func getBGPPeerByID(url string, client *http.Client, token string, id int) (int, *GetBGPPeerResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/bgp/peers/"+strconv.Itoa(id), nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var resp GetBGPPeerResponse

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func createBGPPeer(url string, client *http.Client, token string, params *CreateBGPPeerParams) (int, *CreateBGPPeerResponse, error) {
	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/networking/bgp/peers", bytes.NewReader(payloadBytes))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var resp CreateBGPPeerResponse

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func updateBGPPeerByID(url string, client *http.Client, token string, id int, params *UpdateBGPPeerTestParams) (int, *UpdateBGPPeerResponse, error) {
	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/networking/bgp/peers/"+strconv.Itoa(id), bytes.NewReader(payloadBytes))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var resp UpdateBGPPeerResponse

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func deleteBGPPeerByID(url string, client *http.Client, token string, id int) (int, *DeleteBGPPeerResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/networking/bgp/peers/"+strconv.Itoa(id), nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var resp DeleteBGPPeerResponse

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

// BGP Routes response types

type BGPRouteResult struct {
	Subscriber string `json:"subscriber"`
	Prefix     string `json:"prefix"`
	NextHop    string `json:"nextHop"`
}

type BGPAdvertisedRoutesResponseResult struct {
	Routes []BGPRouteResult `json:"routes"`
}

type GetBGPAdvertisedRoutesResponse struct {
	Result BGPAdvertisedRoutesResponseResult `json:"result"`
	Error  string                            `json:"error,omitempty"`
}

type BGPLearnedRouteResult struct {
	Prefix  string `json:"prefix"`
	NextHop string `json:"nextHop"`
	Peer    string `json:"peer"`
}

type BGPLearnedRoutesResponseResult struct {
	Routes []BGPLearnedRouteResult `json:"routes"`
}

type GetBGPLearnedRoutesResponse struct {
	Result BGPLearnedRoutesResponseResult `json:"result"`
	Error  string                         `json:"error,omitempty"`
}

func getBGPAdvertisedRoutes(url string, client *http.Client, token string) (int, *GetBGPAdvertisedRoutesResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/bgp/advertised-routes", nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var resp GetBGPAdvertisedRoutesResponse

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func getBGPLearnedRoutes(url string, client *http.Client, token string) (int, *GetBGPLearnedRoutesResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/bgp/learned-routes", nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var resp GetBGPLearnedRoutesResponse

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

// Tests

func TestApiBGPSettingsEndToEnd(t *testing.T) {
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

	t.Run("1. Get BGP settings (default)", func(t *testing.T) {
		statusCode, resp, err := getBGPSettings(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get BGP settings: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}

		if resp.Result.Enabled {
			t.Fatalf("expected BGP to be disabled by default")
		}

		if resp.Result.LocalAS != 64512 {
			t.Fatalf("expected default localAS 64512, got %d", resp.Result.LocalAS)
		}

		if resp.Result.RouterID != "" {
			t.Fatalf("expected empty default routerID, got %s", resp.Result.RouterID)
		}
	})

	t.Run("2. Update BGP settings to enable", func(t *testing.T) {
		params := &UpdateBGPSettingsParams{
			Enabled:  true,
			LocalAS:  64513,
			RouterID: "192.168.5.10",
		}

		statusCode, resp, err := updateBGPSettings(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't update BGP settings: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}

		if resp.Result.Message != "BGP settings updated successfully" {
			t.Fatalf("unexpected message: %s", resp.Result.Message)
		}
	})

	t.Run("3. Get BGP settings (enabled)", func(t *testing.T) {
		statusCode, resp, err := getBGPSettings(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get BGP settings: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if !resp.Result.Enabled {
			t.Fatalf("expected BGP to be enabled")
		}

		if resp.Result.LocalAS != 64513 {
			t.Fatalf("expected localAS 64513, got %d", resp.Result.LocalAS)
		}

		if resp.Result.RouterID != "192.168.5.10" {
			t.Fatalf("expected routerID 192.168.5.10, got %s", resp.Result.RouterID)
		}
	})

	t.Run("5. Enable NAT while BGP is enabled should succeed (coexistence)", func(t *testing.T) {
		statusCode, _, err := updateNATInfo(env.Server.URL, client, token, true)
		if err != nil {
			t.Fatalf("couldn't enable NAT: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
	})

	t.Run("6. Disable BGP while NAT is enabled", func(t *testing.T) {
		params := &UpdateBGPSettingsParams{
			Enabled:  false,
			LocalAS:  64513,
			RouterID: "192.168.5.10",
		}

		statusCode, resp, err := updateBGPSettings(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't update BGP settings: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}
	})

	t.Run("7. Re-enable BGP while NAT is still enabled should succeed (coexistence)", func(t *testing.T) {
		params := &UpdateBGPSettingsParams{
			Enabled:  true,
			LocalAS:  64513,
			RouterID: "192.168.5.10",
		}

		statusCode, resp, err := updateBGPSettings(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't enable BGP: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}
	})
}

func TestApiBGPSettingsToggleCycling(t *testing.T) {
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

	// Cycle BGP on/off/on/off/on rapidly
	for i := 0; i < 5; i++ {
		enabled := i%2 == 0

		params := &UpdateBGPSettingsParams{
			Enabled:  enabled,
			LocalAS:  64513,
			RouterID: "10.0.0.1",
		}

		statusCode, resp, err := updateBGPSettings(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("cycle %d: couldn't update BGP settings: %s", i, err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("cycle %d: expected status %d, got %d", i, http.StatusOK, statusCode)
		}

		if resp.Result.Message != "BGP settings updated successfully" {
			t.Fatalf("cycle %d: unexpected message: %s", i, resp.Result.Message)
		}

		// Verify the state is correct after each toggle
		getStatus, getResp, err := getBGPSettings(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("cycle %d: couldn't get BGP settings: %s", i, err)
		}

		if getStatus != http.StatusOK {
			t.Fatalf("cycle %d: expected status %d, got %d", i, http.StatusOK, getStatus)
		}

		if getResp.Result.Enabled != enabled {
			t.Fatalf("cycle %d: expected enabled=%t, got %t", i, enabled, getResp.Result.Enabled)
		}

		if getResp.Result.LocalAS != 64513 {
			t.Fatalf("cycle %d: expected localAS 64513, got %d", i, getResp.Result.LocalAS)
		}

		if getResp.Result.RouterID != "10.0.0.1" {
			t.Fatalf("cycle %d: expected routerID 10.0.0.1, got %s", i, getResp.Result.RouterID)
		}
	}
}

func TestApiBGPSettingsValidation(t *testing.T) {
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

	t.Run("Invalid localAS (0)", func(t *testing.T) {
		params := &UpdateBGPSettingsParams{
			Enabled: false,
			LocalAS: 0,
		}

		statusCode, _, err := updateBGPSettings(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't update BGP settings: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("Invalid routerID", func(t *testing.T) {
		params := &UpdateBGPSettingsParams{
			Enabled:  false,
			LocalAS:  64512,
			RouterID: "not-an-ip",
		}

		statusCode, _, err := updateBGPSettings(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't update BGP settings: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})
}

func TestApiBGPPeersEndToEnd(t *testing.T) {
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

	t.Run("1. List peers (empty)", func(t *testing.T) {
		statusCode, resp, err := listBGPPeers(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list BGP peers: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Result.TotalCount != 0 {
			t.Fatalf("expected 0 peers, got %d", resp.Result.TotalCount)
		}

		if len(resp.Result.Items) != 0 {
			t.Fatalf("expected empty items, got %d", len(resp.Result.Items))
		}
	})

	t.Run("2. Create a peer with import prefixes", func(t *testing.T) {
		params := &CreateBGPPeerParams{
			Address:     "192.168.5.1",
			RemoteAS:    64512,
			HoldTime:    90,
			Description: "core-router",
			ImportPrefixes: []BGPImportPrefixResult{
				{Prefix: "0.0.0.0/0", MaxLength: 0},
				{Prefix: "10.100.0.0/16", MaxLength: 24},
			},
		}

		statusCode, resp, err := createBGPPeer(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't create BGP peer: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}

		if resp.Result.Message != "BGP peer created successfully" {
			t.Fatalf("unexpected message: %s", resp.Result.Message)
		}
	})

	t.Run("3. List peers (1 peer with import prefixes)", func(t *testing.T) {
		statusCode, resp, err := listBGPPeers(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list BGP peers: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Result.TotalCount != 1 {
			t.Fatalf("expected 1 peer, got %d", resp.Result.TotalCount)
		}

		peer := resp.Result.Items[0]
		if peer.Address != "192.168.5.1" {
			t.Fatalf("expected address 192.168.5.1, got %s", peer.Address)
		}

		if peer.RemoteAS != 64512 {
			t.Fatalf("expected remoteAS 64512, got %d", peer.RemoteAS)
		}

		if peer.Description != "core-router" {
			t.Fatalf("expected description core-router, got %s", peer.Description)
		}

		if len(peer.ImportPrefixes) != 2 {
			t.Fatalf("expected 2 import prefixes, got %d", len(peer.ImportPrefixes))
		}

		if peer.ImportPrefixes[0].Prefix != "0.0.0.0/0" {
			t.Fatalf("expected first prefix 0.0.0.0/0, got %s", peer.ImportPrefixes[0].Prefix)
		}
	})

	t.Run("4. Get single peer by ID", func(t *testing.T) {
		_, listResp, _ := listBGPPeers(env.Server.URL, client, token)
		peerID := listResp.Result.Items[0].ID

		statusCode, resp, err := getBGPPeerByID(env.Server.URL, client, token, peerID)
		if err != nil {
			t.Fatalf("couldn't get BGP peer: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Result.Address != "192.168.5.1" {
			t.Fatalf("expected address 192.168.5.1, got %s", resp.Result.Address)
		}

		if len(resp.Result.ImportPrefixes) != 2 {
			t.Fatalf("expected 2 import prefixes, got %d", len(resp.Result.ImportPrefixes))
		}
	})

	t.Run("5. Get non-existent peer returns 404", func(t *testing.T) {
		statusCode, _, err := getBGPPeerByID(env.Server.URL, client, token, 9999)
		if err != nil {
			t.Fatalf("couldn't attempt to get non-existent peer: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})

	t.Run("6. Update peer", func(t *testing.T) {
		_, listResp, _ := listBGPPeers(env.Server.URL, client, token)
		peerID := listResp.Result.Items[0].ID

		params := &UpdateBGPPeerTestParams{
			Address:     "192.168.5.1",
			RemoteAS:    64513,
			HoldTime:    120,
			Description: "updated-router",
			ImportPrefixes: []BGPImportPrefixResult{
				{Prefix: "10.0.0.0/8", MaxLength: 24},
			},
		}

		statusCode, resp, err := updateBGPPeerByID(env.Server.URL, client, token, peerID, params)
		if err != nil {
			t.Fatalf("couldn't update BGP peer: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Result.Message != "BGP peer updated successfully" {
			t.Fatalf("unexpected message: %s", resp.Result.Message)
		}

		// Verify the update
		_, getResp, _ := getBGPPeerByID(env.Server.URL, client, token, peerID)
		if getResp.Result.RemoteAS != 64513 {
			t.Fatalf("expected remoteAS 64513, got %d", getResp.Result.RemoteAS)
		}

		if getResp.Result.Description != "updated-router" {
			t.Fatalf("expected description updated-router, got %s", getResp.Result.Description)
		}

		if len(getResp.Result.ImportPrefixes) != 1 {
			t.Fatalf("expected 1 import prefix, got %d", len(getResp.Result.ImportPrefixes))
		}

		if getResp.Result.ImportPrefixes[0].Prefix != "10.0.0.0/8" {
			t.Fatalf("expected prefix 10.0.0.0/8, got %s", getResp.Result.ImportPrefixes[0].Prefix)
		}
	})

	t.Run("7. Update non-existent peer returns 404", func(t *testing.T) {
		params := &UpdateBGPPeerTestParams{
			Address:  "10.0.0.1",
			RemoteAS: 64512,
		}

		statusCode, _, err := updateBGPPeerByID(env.Server.URL, client, token, 9999, params)
		if err != nil {
			t.Fatalf("couldn't attempt to update non-existent peer: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})

	t.Run("8. Create duplicate peer should fail (409)", func(t *testing.T) {
		params := &CreateBGPPeerParams{
			Address:  "192.168.5.1",
			RemoteAS: 64513,
		}

		statusCode, _, err := createBGPPeer(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't attempt to create duplicate peer: %s", err)
		}

		if statusCode != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, statusCode)
		}
	})

	t.Run("9. Delete peer", func(t *testing.T) {
		_, listResp, err := listBGPPeers(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list peers: %s", err)
		}

		peerID := listResp.Result.Items[0].ID

		statusCode, resp, err := deleteBGPPeerByID(env.Server.URL, client, token, peerID)
		if err != nil {
			t.Fatalf("couldn't delete BGP peer: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}
	})

	t.Run("10. List peers (empty after delete)", func(t *testing.T) {
		statusCode, resp, err := listBGPPeers(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list BGP peers: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Result.TotalCount != 0 {
			t.Fatalf("expected 0 peers, got %d", resp.Result.TotalCount)
		}
	})

	t.Run("11. Delete non-existent peer should fail (404)", func(t *testing.T) {
		statusCode, _, err := deleteBGPPeerByID(env.Server.URL, client, token, 9999)
		if err != nil {
			t.Fatalf("couldn't attempt to delete non-existent peer: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})
}

func TestApiBGPPeerValidation(t *testing.T) {
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

	t.Run("Missing address", func(t *testing.T) {
		params := &CreateBGPPeerParams{
			RemoteAS: 64512,
		}

		statusCode, _, err := createBGPPeer(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't create BGP peer: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("Invalid address", func(t *testing.T) {
		params := &CreateBGPPeerParams{
			Address:  "not-an-ip",
			RemoteAS: 64512,
		}

		statusCode, _, err := createBGPPeer(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't create BGP peer: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("Invalid remoteAS (0)", func(t *testing.T) {
		params := &CreateBGPPeerParams{
			Address:  "10.0.0.1",
			RemoteAS: 0,
		}

		statusCode, _, err := createBGPPeer(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't create BGP peer: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("Invalid import prefix", func(t *testing.T) {
		params := &CreateBGPPeerParams{
			Address:  "10.0.0.1",
			RemoteAS: 64512,
			ImportPrefixes: []BGPImportPrefixResult{
				{Prefix: "not-a-cidr", MaxLength: 32},
			},
		}

		statusCode, _, err := createBGPPeer(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't create BGP peer: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("Invalid import prefix maxLength", func(t *testing.T) {
		params := &CreateBGPPeerParams{
			Address:  "10.0.0.1",
			RemoteAS: 64512,
			ImportPrefixes: []BGPImportPrefixResult{
				{Prefix: "10.0.0.0/24", MaxLength: 33},
			},
		}

		statusCode, _, err := createBGPPeer(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't create BGP peer: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("Max peers limit", func(t *testing.T) {
		// Create 5 peers (max)
		for i := 1; i <= 5; i++ {
			params := &CreateBGPPeerParams{
				Address:  fmt.Sprintf("10.0.0.%d", i),
				RemoteAS: 64512,
			}

			statusCode, _, err := createBGPPeer(env.Server.URL, client, token, params)
			if err != nil {
				t.Fatalf("couldn't create BGP peer %d: %s", i, err)
			}

			if statusCode != http.StatusCreated {
				t.Fatalf("expected status %d for peer %d, got %d", http.StatusCreated, i, statusCode)
			}
		}

		// 6th should fail
		params := &CreateBGPPeerParams{
			Address:  "10.0.0.6",
			RemoteAS: 64512,
		}

		statusCode, _, err := createBGPPeer(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't attempt to create 6th peer: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})
}

func TestApiBGPAdvertisedRoutesEndpoint(t *testing.T) {
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

	t.Run("Advertised routes returns empty when BGP not running", func(t *testing.T) {
		statusCode, resp, err := getBGPAdvertisedRoutes(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get BGP advertised routes: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(resp.Result.Routes) != 0 {
			t.Fatalf("expected 0 routes, got %d", len(resp.Result.Routes))
		}
	})
}

func TestApiBGPLearnedRoutesEndpoint(t *testing.T) {
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

	t.Run("Learned routes returns empty when BGP not running", func(t *testing.T) {
		statusCode, resp, err := getBGPLearnedRoutes(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get BGP learned routes: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(resp.Result.Routes) != 0 {
			t.Fatalf("expected 0 routes, got %d", len(resp.Result.Routes))
		}
	})
}
