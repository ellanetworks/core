package server_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const NetworkSliceName = "test-network-slice"

type CreateNetworkSliceSuccessResponse struct {
	Message string `json:"message"`
}

type GNodeB struct {
	Name string `json:"name,omitempty"`
	Tac  int32  `json:"tac,omitempty"`
}

type UPF struct {
	Name string `json:"name,omitempty"`
	Port int    `json:"port,omitempty"`
}

type GetNetworkSliceResponse struct {
	Name     string   `json:"name,omitempty"`
	Sst      string   `json:"sst,omitempty"`
	Sd       string   `json:"sd,omitempty"`
	Profiles []string `json:"profiles"`
	Mcc      string   `json:"mcc,omitempty"`
	Mnc      string   `json:"mnc,omitempty"`
	GNodeBs  []GNodeB `json:"gNodeBs"`
	Upf      UPF      `json:"upf,omitempty"`
}

type CreateNetworkSliceParams struct {
	Name     string   `json:"name,omitempty"`
	Sst      string   `json:"sst,omitempty"`
	Sd       string   `json:"sd,omitempty"`
	Profiles []string `json:"profiles"`
	Mcc      string   `json:"mcc,omitempty"`
	Mnc      string   `json:"mnc,omitempty"`
	GNodeBs  []GNodeB `json:"gNodeBs"`
	Upf      UPF      `json:"upf,omitempty"`
}

type CreateNetworkSliceResponseResult struct {
	ID int `json:"id"`
}

type CreateNetworkSliceResponse struct {
	Result CreateNetworkSliceSuccessResponse `json:"result"`
	Error  string                            `json:"error,omitempty"`
}

type DeleteNetworkSliceResponseResult struct {
	ID int `json:"id"`
}

func getNetworkSlice(url string, client *http.Client, name string) (int, *GetNetworkSliceResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/network-slices/"+name, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var networkSliceResponse GetNetworkSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&networkSliceResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &networkSliceResponse, nil
}

func createNetworkSlice(url string, client *http.Client, data *CreateNetworkSliceParams) (int, *CreateNetworkSliceResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url+"/api/v1/network-slices", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse CreateNetworkSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteNetworkSlice(url string, client *http.Client, name string) (int, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/network-slices/"+name, nil)
	if err != nil {
		return 0, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return res.StatusCode, nil
}

// This is an end-to-end test for the network-slices handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestNetworkSlicesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("1. Create network slice", func(t *testing.T) {
		createNetworkSliceParams := &CreateNetworkSliceParams{
			Name: NetworkSliceName,
			Sst:  "001",
			Sd:   "1",
			Profiles: []string{
				"my-profile",
			},
			Mcc: "123",
			Mnc: "456",
			GNodeBs: []GNodeB{
				{
					Name: "gnb-001",
					Tac:  12345,
				},
			},
			Upf: UPF{
				Name: "upf-001",
				Port: 1234,
			},
		}
		statusCode, response, err := createNetworkSlice(ts.URL, client, createNetworkSliceParams)
		if err != nil {
			t.Fatalf("couldn't create network slice: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("2. Get network slice", func(t *testing.T) {
		statusCode, response, err := getNetworkSlice(ts.URL, client, NetworkSliceName)
		if err != nil {
			t.Fatalf("couldn't get network slice: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Name != NetworkSliceName {
			t.Fatalf("expected name %s, got %s", NetworkSliceName, response.Name)
		}
		if response.Sst != "001" {
			t.Fatalf("expected sst %s, got %s", "001", response.Sst)
		}
		if response.Sd != "1" {
			t.Fatalf("expected sd %s, got %s", "1", response.Sd)
		}
		if len(response.Profiles) != 1 {
			t.Fatalf("expected 1 profile, got %d", len(response.Profiles))
		}

		if response.Profiles[0] != "my-profile" {
			t.Fatalf("expected profile my-profile, got %s", response.Profiles[0])
		}
		if response.Mcc != "123" {
			t.Fatalf("expected mcc %s, got %s", "123", response.Mcc)
		}
		if response.Mnc != "456" {
			t.Fatalf("expected mnc %s, got %s", "456", response.Mnc)
		}
		if len(response.GNodeBs) != 1 {
			t.Fatalf("expected 1 gNodeB, got %d", len(response.GNodeBs))
		}
		if response.GNodeBs[0].Name != "gnb-001" {
			t.Fatalf("expected gnb-001, got %s", response.GNodeBs[0].Name)
		}
		if response.GNodeBs[0].Tac != 12345 {
			t.Fatalf("expected tac 12345, got %d", response.GNodeBs[0].Tac)
		}
		if response.Upf.Name != "upf-001" {
			t.Fatalf("expected upf-001, got %s", response.Upf.Name)
		}
		if response.Upf.Port != 1234 {
			t.Fatalf("expected port 1234, got %d", response.Upf.Port)
		}
	})

	t.Run("3. Get network slice - id not found", func(t *testing.T) {
		statusCode, _, err := getNetworkSlice(ts.URL, client, "network-slice-002")
		if err != nil {
			t.Fatalf("couldn't get network slice: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})

	t.Run("4. Create network slice - no name", func(t *testing.T) {
		createNetworkSliceParams := &CreateNetworkSliceParams{}
		statusCode, _, err := createNetworkSlice(ts.URL, client, createNetworkSliceParams)
		if err != nil {
			t.Fatalf("couldn't create network slice: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("5. Delete network slice - success", func(t *testing.T) {
		statusCode, err := deleteNetworkSlice(ts.URL, client, NetworkSliceName)
		if err != nil {
			t.Fatalf("couldn't delete network slice: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
	})

	t.Run("6. Delete network slice - no slice", func(t *testing.T) {
		statusCode, err := deleteNetworkSlice(ts.URL, client, NetworkSliceName)
		if err != nil {
			t.Fatalf("couldn't delete network slice: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})
}
