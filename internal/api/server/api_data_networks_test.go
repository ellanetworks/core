package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	DataNetworkName = "not-internet"
	DNS             = "8.8.8.8"
	IPPool          = "0.0.0.0/24"
	MTU             = 1500
)

type CreateDataNetworkResponseResult struct {
	Message string `json:"message"`
}

type DataNetwork struct {
	Name   string `json:"name"`
	IPPool string `json:"ip_pool,omitempty"`
	DNS    string `json:"dns,omitempty"`
	MTU    int32  `json:"mtu,omitempty"`
}

type GetDataNetworkResponse struct {
	Result DataNetwork `json:"result"`
	Error  string      `json:"error,omitempty"`
}

type CreateDataNetworkParams struct {
	Name   string `json:"name"`
	IPPool string `json:"ip_pool,omitempty"`
	DNS    string `json:"dns,omitempty"`
	MTU    int32  `json:"mtu,omitempty"`
}

type CreateDataNetworkResponse struct {
	Result CreateDataNetworkResponseResult `json:"result"`
	Error  string                          `json:"error,omitempty"`
}

type DeleteDataNetworkResponseResult struct {
	Message string `json:"message"`
}

type DeleteDataNetworkResponse struct {
	Result DeleteDataNetworkResponseResult `json:"result"`
	Error  string                          `json:"error,omitempty"`
}

type ListDataNetworksResponseResult struct {
	Items      []DataNetwork `json:"items"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalCount int           `json:"total_count"`
}

type ListDataNetworkResponse struct {
	Result ListDataNetworksResponseResult `json:"result"`
	Error  string                         `json:"error,omitempty"`
}

func listDataNetworks(url string, client *http.Client, token string) (int, *ListDataNetworkResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/data-networks", nil)
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

	var dataNetworkResponse ListDataNetworkResponse

	if err := json.NewDecoder(res.Body).Decode(&dataNetworkResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &dataNetworkResponse, nil
}

func getDataNetwork(url string, client *http.Client, token string, name string) (int, *GetDataNetworkResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/networking/data-networks/"+name, nil)
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
	var dataNetworkResponse GetDataNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&dataNetworkResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &dataNetworkResponse, nil
}

func createDataNetwork(url string, client *http.Client, token string, data *CreateDataNetworkParams) (int, *CreateDataNetworkResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/networking/data-networks", strings.NewReader(string(body)))
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
	var createResponse CreateDataNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func editDataNetwork(url string, client *http.Client, name string, token string, data *CreateDataNetworkParams) (int, *CreateDataNetworkResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/networking/data-networks/"+name, strings.NewReader(string(body)))
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
	var createResponse CreateDataNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteDataNetwork(url string, client *http.Client, token, name string) (int, *DeleteDataNetworkResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/networking/data-networks/"+name, nil)
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
	var deleteDataNetworkResponse DeleteDataNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteDataNetworkResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteDataNetworkResponse, nil
}

// This is an end-to-end test for the data networks handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestAPIDataNetworksEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. List data networks - 1", func(t *testing.T) {
		statusCode, response, err := listDataNetworks(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list data networks: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(response.Result.Items) != 1 {
			t.Fatalf("expected 1 data networks, got %d", len(response.Result.Items))
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("2. Create data network", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:   DataNetworkName,
			MTU:    MTU,
			IPPool: IPPool,
			DNS:    DNS,
		}
		statusCode, response, err := createDataNetwork(ts.URL, client, token, createDataNetworkParams)
		if err != nil {
			t.Fatalf("couldn't create subscriber: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("3. List data networks - 2", func(t *testing.T) {
		statusCode, response, err := listDataNetworks(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list data network: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(response.Result.Items) != 2 {
			t.Fatalf("expected 2 data network, got %d", len(response.Result.Items))
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("4. Get data network", func(t *testing.T) {
		statusCode, response, err := getDataNetwork(ts.URL, client, token, DataNetworkName)
		if err != nil {
			t.Fatalf("couldn't get data network: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Name != DataNetworkName {
			t.Fatalf("expected name %s, got %s", DataNetworkName, response.Result.Name)
		}

		if response.Result.IPPool != IPPool {
			t.Fatalf("expected ip pool %s got %s", IPPool, response.Result.IPPool)
		}
		if response.Result.DNS != DNS {
			t.Fatalf("expected DNS %s got %s", DNS, response.Result.DNS)
		}
		if response.Result.MTU != MTU {
			t.Fatalf("expected MTU %v got %d", MTU, response.Result.MTU)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("6. Get data network - id not found", func(t *testing.T) {
		statusCode, response, err := getDataNetwork(ts.URL, client, token, "data network-002")
		if err != nil {
			t.Fatalf("couldn't get data network: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "Data Network not found" {
			t.Fatalf("expected error %q, got %q", "Data Network not found", response.Error)
		}
	})

	t.Run("7. Create data network - no name", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{}
		statusCode, response, err := createDataNetwork(ts.URL, client, token, createDataNetworkParams)
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "name is missing" {
			t.Fatalf("expected error %q, got %q", "name is missing", response.Error)
		}
	})

	t.Run("8. Edit data network - success", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:   DataNetworkName,
			DNS:    "2.2.2.2",
			IPPool: "1.1.1.0/29",
			MTU:    1400,
		}
		statusCode, response, err := editDataNetwork(ts.URL, client, DataNetworkName, token, createDataNetworkParams)
		if err != nil {
			t.Fatalf("couldn't edit data network: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("9. Create policy", func(t *testing.T) {
		createPolicyParams := &CreatePolicyParams{
			Name:            "whatever",
			BitrateUplink:   BitrateUplink,
			BitrateDownlink: BitrateDownlink,
			Var5qi:          Var5qi,
			PriorityLevel:   PriorityLevel,
			DataNetworkName: DataNetworkName,
		}
		statusCode, response, err := createPolicy(ts.URL, client, token, createPolicyParams)
		if err != nil {
			t.Fatalf("couldn't edit data network: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("10. Delete data network - failure because data network has policies", func(t *testing.T) {
		statusCode, response, err := deleteDataNetwork(ts.URL, client, token, DataNetworkName)
		if err != nil {
			t.Fatalf("couldn't delete data network: %s", err)
		}
		if statusCode != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, statusCode)
		}
		if response.Error != "Data Network has policies" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("11. Delete policy", func(t *testing.T) {
		statusCode, response, err := deletePolicy(ts.URL, client, token, "whatever")
		if err != nil {
			t.Fatalf("couldn't delete policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("12. Delete data network - success", func(t *testing.T) {
		statusCode, response, err := deleteDataNetwork(ts.URL, client, token, DataNetworkName)
		if err != nil {
			t.Fatalf("couldn't delete data network: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("13. Delete data network - no data network", func(t *testing.T) {
		statusCode, response, err := deleteDataNetwork(ts.URL, client, token, DataNetworkName)
		if err != nil {
			t.Fatalf("couldn't delete data network: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "Data Network not found" {
			t.Fatalf("expected error %q, got %q", "Data Network not found", response.Error)
		}
	})
}

func TestCreateDataNetworkInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	tests := []struct {
		testName string
		name     string
		ipPool   string
		dns      string
		mtu      int32
		error    string
	}{
		{
			testName: "Invalid Name - 1",
			name:     "Internet",
			ipPool:   IPPool,
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid name format, must be a valid DNN format",
		},
		{
			testName: "Invalid Name - 2",
			name:     "data_network",
			ipPool:   IPPool,
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid name format, must be a valid DNN format",
		},
		{
			testName: "Invalid Name - 3",
			name:     "-privatenet",
			ipPool:   IPPool,
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid name format, must be a valid DNN format",
		},
		{
			testName: "Invalid Name - 4",
			name:     "net.",
			ipPool:   IPPool,
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid name format, must be a valid DNN format",
		},
		{
			testName: "Invalid IP Pool - bad format",
			name:     DataNetworkName,
			ipPool:   "invalid-ip_pool",
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid ip_pool format, must be in CIDR format",
		},
		{
			testName: "Invalid IP Pool - missing subnet",
			name:     DataNetworkName,
			ipPool:   "0.0.0.0",
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid ip_pool format, must be in CIDR format",
		},
		{
			testName: "Invalid IP Pool - Too many bits",
			name:     DataNetworkName,
			ipPool:   "0.0.0.0/2555",
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid ip_pool format, must be in CIDR format",
		},
		{
			testName: "Invalid DNS",
			name:     DataNetworkName,
			ipPool:   IPPool,
			dns:      "invalid-dns",
			mtu:      MTU,
			error:    "invalid dns format, must be a valid IP address",
		},
		{
			testName: "Invalid MTU - too small",
			name:     DataNetworkName,
			ipPool:   IPPool,
			dns:      DNS,
			mtu:      -1,
			error:    "invalid mtu format, must be an integer between 0 and 65535",
		},
		{
			testName: "Invalid MTU - too large",
			name:     DataNetworkName,
			ipPool:   IPPool,
			dns:      DNS,
			mtu:      65535 + 1,
			error:    "invalid mtu format, must be an integer between 0 and 65535",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			createDataNetworkParams := &CreateDataNetworkParams{
				Name:   tt.name,
				IPPool: tt.ipPool,
				DNS:    tt.dns,
				MTU:    tt.mtu,
			}
			statusCode, response, err := createDataNetwork(ts.URL, client, token, createDataNetworkParams)
			if err != nil {
				t.Fatalf("couldn't create data network: %s", err)
			}
			if statusCode != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
			}
			if response.Error != tt.error {
				t.Fatalf("expected error %q, got %q", tt.error, response.Error)
			}
		})
	}
}

func TestCreateTooManyDataNetworks(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	for i := 0; i < 11; i++ { // We use 11 instead of 12 because the first data network is created by default
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:   "data-network-" + strconv.Itoa(i),
			IPPool: IPPool,
			DNS:    DNS,
			MTU:    MTU,
		}

		statusCode, response, err := createDataNetwork(ts.URL, client, token, createDataNetworkParams)
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	}

	createDataNetworkParams := &CreateDataNetworkParams{
		Name:   "data-network-too-many",
		IPPool: IPPool,
		DNS:    DNS,
		MTU:    MTU,
	}
	statusCode, response, err := createDataNetwork(ts.URL, client, token, createDataNetworkParams)
	if err != nil {
		t.Fatalf("couldn't create data network: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
	}

	if response.Error != "Maximum number of data networks reached (12)" {
		t.Fatalf("expected error %q, got %q", "Maximum number of data networks reached (12)", response.Error)
	}
}
