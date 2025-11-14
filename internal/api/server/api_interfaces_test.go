package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
)

type N2Interface struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type N3Interface struct {
	Name            string `json:"name"`
	Address         string `json:"address"`
	ExternalAddress string `json:"external_address"`
}

type N6Interface struct {
	Name string `json:"name"`
}

type APIInterface struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type NetworkInterfaces struct {
	N2  N2Interface  `json:"n2"`
	N3  N3Interface  `json:"n3"`
	N6  N6Interface  `json:"n6"`
	API APIInterface `json:"api"`
}

type GetNetworkInterfaceInfoResponse struct {
	Result NetworkInterfaces `json:"result"`
	Error  string            `json:"error,omitempty"`
}

type UpdateN3InfoParams struct {
	ExternalAddress string `json:"external_address"`
}

type UpdateN3InfoResponseResult struct {
	Message string `json:"message"`
}

type UpdateN3InfoResponse struct {
	Result UpdateN3InfoResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

func listNetworkInterfaces(url string, client *http.Client, token string) (int, *GetNetworkInterfaceInfoResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/interfaces", nil)
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

	var resp GetNetworkInterfaceInfoResponse

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func updateN3Info(url string, client *http.Client, token string, externalAddress string) (int, *UpdateN3InfoResponse, error) {
	params := UpdateN3InfoParams{
		ExternalAddress: externalAddress,
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/networking/interfaces/n3", bytes.NewReader(payloadBytes))
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

	var updateResponse UpdateN3InfoResponse

	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &updateResponse, nil
}

func TestNetworkInteraces_EndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. List network interfaces", func(t *testing.T) {
		statusCode, resp, err := listNetworkInterfaces(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list network interfaces: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}

		if resp.Result.N2.Address != "12.12.12.12" {
			t.Fatalf("unexpected N2 interface address: %s", resp.Result.N2.Address)
		}

		if resp.Result.N2.Port != 2152 {
			t.Fatalf("unexpected N2 interface port: %d", resp.Result.N2.Port)
		}

		if resp.Result.N3.Name != "eth0" {
			t.Fatalf("unexpected N3 interface name: %s", resp.Result.N3.Name)
		}

		if resp.Result.N3.Address != "13.13.13.13" {
			t.Fatalf("unexpected N3 interface address: %s", resp.Result.N3.Address)
		}

		if resp.Result.N3.ExternalAddress != "" {
			t.Fatalf("unexpected N3 interface external address: %s", resp.Result.N3.ExternalAddress)
		}

		if resp.Result.N6.Name != "eth1" {
			t.Fatalf("unexpected N6 interface name: %s", resp.Result.N6.Name)
		}

		if resp.Result.API.Address != "" {
			t.Fatalf("unexpected API interface address: %s", resp.Result.API.Address)
		}

		if resp.Result.API.Port != 8443 {
			t.Fatalf("unexpected API interface port: %d", resp.Result.API.Port)
		}
	})

	t.Run("2. Update N3 external address to IP", func(t *testing.T) {
		statusCode, updateResponse, err := updateN3Info(ts.URL, client, token, "192.168.1.1")
		if err != nil {
			t.Fatalf("couldn't update N3 info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if updateResponse.Error != "" {
			t.Fatalf("expected no error, got %s", updateResponse.Error)
		}

		if updateResponse.Result.Message != "N3 interface updated" {
			t.Fatalf("unexpected message: %s", updateResponse.Result.Message)
		}
	})

	t.Run("3. Validate that N3 external address is updated", func(t *testing.T) {
		statusCode, resp, err := listNetworkInterfaces(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list network interfaces: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}

		if resp.Result.N3.ExternalAddress != "192.168.1.1" {
			t.Fatalf("unexpected N3 interface external address: %s", resp.Result.N3.ExternalAddress)
		}
	})

	t.Run("4. Update N3 external address to FQDN", func(t *testing.T) {
		statusCode, updateResponse, err := updateN3Info(ts.URL, client, token, "example.com")
		if err != nil {
			t.Fatalf("couldn't update N3 info: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if updateResponse.Error != "Invalid external address. Must be a valid IP address" {
			t.Fatalf("expected no error, got %s", updateResponse.Error)
		}
	})

	t.Run("5. Update N3 external address to empty", func(t *testing.T) {
		statusCode, updateResponse, err := updateN3Info(ts.URL, client, token, "")
		if err != nil {
			t.Fatalf("couldn't update N3 info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if updateResponse.Error != "" {
			t.Fatalf("expected no error, got %s", updateResponse.Error)
		}

		if updateResponse.Result.Message != "N3 interface updated" {
			t.Fatalf("unexpected message: %s", updateResponse.Result.Message)
		}
	})

	t.Run("6. Validate that N3 external address is updated", func(t *testing.T) {
		statusCode, resp, err := listNetworkInterfaces(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list network interfaces: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}

		if resp.Result.N3.ExternalAddress != "" {
			t.Fatalf("unexpected N3 interface external address: %s", resp.Result.N3.ExternalAddress)
		}
	})
}
