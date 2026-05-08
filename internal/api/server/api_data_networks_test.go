package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	DataNetworkName = "not-internet"
	DNS             = "8.8.8.8"
	IPv4Pool        = "0.0.0.0/24"
	MTU             = 1500
)

type CreateDataNetworkResponseResult struct {
	Message string `json:"message"`
}

type DataNetworkIPAllocation struct {
	PoolSize  int `json:"pool_size"`
	Allocated int `json:"allocated"`
	Available int `json:"available"`
}

type DataNetwork struct {
	Name           string                   `json:"name"`
	IPv4Pool       string                   `json:"ipv4_pool,omitempty"`
	IPv6Pool       string                   `json:"ipv6_pool,omitempty"`
	DNS            string                   `json:"dns,omitempty"`
	MTU            int32                    `json:"mtu,omitempty"`
	IPAllocation   *DataNetworkIPAllocation `json:"ip_allocation,omitempty"`
	IPv6Allocation *DataNetworkIPAllocation `json:"ipv6_allocation,omitempty"`
}

type GetDataNetworkResponse struct {
	Result DataNetwork `json:"result"`
	Error  string      `json:"error,omitempty"`
}

type CreateDataNetworkParams struct {
	Name     string `json:"name"`
	IPv4Pool string `json:"ipv4_pool,omitempty"`
	IPv6Pool string `json:"ipv6_pool,omitempty"`
	DNS      string `json:"dns,omitempty"`
	MTU      int32  `json:"mtu,omitempty"`
}

type CreateDataNetworkResponse struct {
	Result CreateDataNetworkResponseResult `json:"result"`
	Error  string                          `json:"error,omitempty"`
}

type UpdateDataNetworkParams struct {
	IPv4Pool string `json:"ipv4_pool,omitempty"`
	IPv6Pool string `json:"ipv6_pool,omitempty"`
	DNS      string `json:"dns,omitempty"`
	MTU      int32  `json:"mtu,omitempty"`
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
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/data-networks/"+name, nil)
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

func editDataNetwork(url string, client *http.Client, name string, token string, data *UpdateDataNetworkParams) (int, *CreateDataNetworkResponse, error) {
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
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/networking/data-networks/"+name, nil)
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

	t.Run("1. List data networks - 1", func(t *testing.T) {
		statusCode, response, err := listDataNetworks(env.Server.URL, client, token)
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
			Name:     DataNetworkName,
			MTU:      MTU,
			IPv4Pool: IPv4Pool,
			DNS:      DNS,
		}

		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, createDataNetworkParams)
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
		statusCode, response, err := listDataNetworks(env.Server.URL, client, token)
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
		statusCode, response, err := getDataNetwork(env.Server.URL, client, token, DataNetworkName)
		if err != nil {
			t.Fatalf("couldn't get data network: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Name != DataNetworkName {
			t.Fatalf("expected name %s, got %s", DataNetworkName, response.Result.Name)
		}

		if response.Result.IPv4Pool != IPv4Pool {
			t.Fatalf("expected ip pool %s got %s", IPv4Pool, response.Result.IPv4Pool)
		}

		if response.Result.DNS != DNS {
			t.Fatalf("expected DNS %s got %s", DNS, response.Result.DNS)
		}

		if response.Result.MTU != MTU {
			t.Fatalf("expected MTU %v got %d", MTU, response.Result.MTU)
		}

		if response.Result.IPAllocation == nil {
			t.Fatal("expected ip_allocation to be present in detail response")
		}

		if response.Result.IPAllocation.PoolSize <= 0 {
			t.Fatalf("expected positive pool_size, got %d", response.Result.IPAllocation.PoolSize)
		}

		if response.Result.IPAllocation.Available != response.Result.IPAllocation.PoolSize-response.Result.IPAllocation.Allocated {
			t.Fatal("available should equal pool_size - allocated")
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("6b. Create data network - IPv6 DNS", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:     "ipv6-dns-test",
			IPv4Pool: "10.50.0.0/24",
			DNS:      "2001:4860:4860::8888",
			IPv6Pool: "2001:db8::/56",
			MTU:      1456,
		}

		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, createDataNetworkParams)
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error: %q", response.Error)
		}
	})

	t.Run("8. Get data network - id not found", func(t *testing.T) {
		statusCode, response, err := getDataNetwork(env.Server.URL, client, token, "data network-002")
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

	t.Run("9. Create data network - no name", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{}

		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, createDataNetworkParams)
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

	t.Run("10. Edit data network - success", func(t *testing.T) {
		updateDataNetworkParams := &UpdateDataNetworkParams{
			DNS:      "2.2.2.2",
			IPv4Pool: "1.1.1.0/29",
			MTU:      1400,
		}

		statusCode, response, err := editDataNetwork(env.Server.URL, client, DataNetworkName, token, updateDataNetworkParams)
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

	t.Run("11. Create profile and policy", func(t *testing.T) {
		_, _, profErr := createProfile(env.Server.URL, client, token, &CreateProfileParams{
			Name:           "dn-test-profile",
			UeAmbrUplink:   SessionAmbrUplink,
			UeAmbrDownlink: SessionAmbrDownlink,
		})
		if profErr != nil {
			t.Fatalf("couldn't create profile: %s", profErr)
		}

		createPolicyParams := &CreatePolicyParams{
			Name:                "whatever",
			ProfileName:         "dn-test-profile",
			SliceName:           DefaultSliceName,
			SessionAmbrUplink:   SessionAmbrUplink,
			SessionAmbrDownlink: SessionAmbrDownlink,
			Var5qi:              Var5qi,
			Arp:                 Arp,
			DataNetworkName:     DataNetworkName,
		}

		statusCode, response, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
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

	t.Run("12. Delete data network - failure because data network has policies", func(t *testing.T) {
		statusCode, response, err := deleteDataNetwork(env.Server.URL, client, token, DataNetworkName)
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

	t.Run("13. Delete policy", func(t *testing.T) {
		statusCode, response, err := deletePolicy(env.Server.URL, client, token, "whatever")
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

	t.Run("14. Delete data network - success", func(t *testing.T) {
		statusCode, response, err := deleteDataNetwork(env.Server.URL, client, token, DataNetworkName)
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

	t.Run("15. Delete data network - no data network", func(t *testing.T) {
		statusCode, response, err := deleteDataNetwork(env.Server.URL, client, token, DataNetworkName)
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

func TestEditInexistentDataNetwork(t *testing.T) {
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

	editDataNetworkParams := &UpdateDataNetworkParams{
		IPv4Pool: IPv4Pool,
		DNS:      DNS,
		MTU:      MTU,
	}

	statusCode, response, err := editDataNetwork(env.Server.URL, client, "inexistent-dn", token, editDataNetworkParams)
	if err != nil {
		t.Fatalf("couldn't edit data network: %s", err)
	}

	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
	}

	if response.Error != "Data Network not found" {
		t.Fatalf("expected error %q, got %q", "Data Network not found", response.Error)
	}
}

// TestUpdateDataNetworkPathBodyMismatch verifies that the path name is used
// for the DB update instead of any name sent in the request body.
func TestUpdateDataNetworkPathBodyMismatch(t *testing.T) {
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

	// Create a data network.
	_, _, err = createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: "real-dn", IPv4Pool: IPv4Pool, DNS: DNS, MTU: MTU,
	})
	if err != nil {
		t.Fatalf("couldn't create data network: %s", err)
	}

	// Update with a different name in the body than the path.
	updateParams := &UpdateDataNetworkParams{
		IPv4Pool: "10.0.0.0/24",
		DNS:      "8.8.8.8",
		MTU:      1400,
	}

	statusCode, response, err := editDataNetwork(env.Server.URL, client, "real-dn", token, updateParams)
	if err != nil {
		t.Fatalf("couldn't edit data network: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d (error: %s)", http.StatusOK, statusCode, response.Error)
	}

	// Verify the real data network was updated (looked up by path name).
	getStatus, getResp, err := getDataNetwork(env.Server.URL, client, token, "real-dn")
	if err != nil {
		t.Fatalf("couldn't get data network: %s", err)
	}

	if getStatus != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getStatus)
	}

	if getResp.Result.MTU != 1400 {
		t.Fatalf("expected MTU 1400, got %d", getResp.Result.MTU)
	}

	// Verify the audit log references the path name, not the body name.
	_, auditResp, err := listAuditLogs(env.Server.URL, client, token, 1, 100)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	var found bool

	for _, entry := range auditResp.Result.Items {
		if entry.Action != "update_data_network" {
			continue
		}

		found = true

		expected := "User updated data network: real-dn"
		if entry.Details != expected {
			t.Errorf("audit log records wrong name: got %q, want %q", entry.Details, expected)
		}

		break
	}

	if !found {
		t.Fatal("no update_data_network audit entry found")
	}
}

func TestCreateDataNetworkInvalidInput(t *testing.T) {
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

	tests := []struct {
		testName string
		name     string
		ipv4Pool string
		dns      string
		mtu      int32
		error    string
	}{
		{
			testName: "Invalid Name - 1",
			name:     "Internet",
			ipv4Pool: IPv4Pool,
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid name format, must be a valid DNN format",
		},
		{
			testName: "Invalid Name - 2",
			name:     "data_network",
			ipv4Pool: IPv4Pool,
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid name format, must be a valid DNN format",
		},
		{
			testName: "Invalid Name - 3",
			name:     "-privatenet",
			ipv4Pool: IPv4Pool,
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid name format, must be a valid DNN format",
		},
		{
			testName: "Invalid Name - 4",
			name:     "net.",
			ipv4Pool: IPv4Pool,
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid name format, must be a valid DNN format",
		},
		{
			testName: "Invalid IP Pool - bad format",
			name:     DataNetworkName,
			ipv4Pool: "invalid-ip_pool",
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid ipv4_pool format, must be in CIDR format",
		},
		{
			testName: "Invalid IP Pool - missing subnet",
			name:     DataNetworkName,
			ipv4Pool: "0.0.0.0",
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid ipv4_pool format, must be in CIDR format",
		},
		{
			testName: "Invalid IP Pool - Too many bits",
			name:     DataNetworkName,
			ipv4Pool: "0.0.0.0/2555",
			dns:      DNS,
			mtu:      MTU,
			error:    "invalid ipv4_pool format, must be in CIDR format",
		},
		{
			testName: "Invalid DNS",
			name:     DataNetworkName,
			ipv4Pool: IPv4Pool,
			dns:      "invalid-dns",
			mtu:      MTU,
			error:    "invalid dns format, must be a valid IP address",
		},
		{
			testName: "Invalid MTU - too small",
			name:     DataNetworkName,
			ipv4Pool: IPv4Pool,
			dns:      DNS,
			mtu:      -1,
			error:    "invalid mtu format, must be an integer between 0 and 65535",
		},
		{
			testName: "Invalid MTU - too large",
			name:     DataNetworkName,
			ipv4Pool: IPv4Pool,
			dns:      DNS,
			mtu:      65535 + 1,
			error:    "invalid mtu format, must be an integer between 0 and 65535",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			createDataNetworkParams := &CreateDataNetworkParams{
				Name:     tt.name,
				IPv4Pool: tt.ipv4Pool,
				DNS:      tt.dns,
				MTU:      tt.mtu,
			}

			statusCode, response, err := createDataNetwork(env.Server.URL, client, token, createDataNetworkParams)
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

	for i := range 11 { // We use 11 instead of 12 because the first data network is created by default
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:     "data-network-" + strconv.Itoa(i),
			IPv4Pool: fmt.Sprintf("10.%d.0.0/24", i),
			DNS:      DNS,
			MTU:      MTU,
		}

		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, createDataNetworkParams)
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
		Name:     "data-network-too-many",
		IPv4Pool: "10.100.0.0/24",
		DNS:      DNS,
		MTU:      MTU,
	}

	statusCode, response, err := createDataNetwork(env.Server.URL, client, token, createDataNetworkParams)
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

func TestCreateDataNetworkOverlappingPool(t *testing.T) {
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

	// Create the first data network with a /24 pool.
	_, _, err = createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: "dn-a", IPv4Pool: "10.45.0.0/24", DNS: DNS, MTU: MTU,
	})
	if err != nil {
		t.Fatalf("couldn't create first data network: %s", err)
	}

	t.Run("exact overlap", func(t *testing.T) {
		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-b", IPv4Pool: "10.45.0.0/24", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if !strings.Contains(response.Error, "overlaps") {
			t.Fatalf("expected overlap error, got %q", response.Error)
		}
	})

	t.Run("superset overlap", func(t *testing.T) {
		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-c", IPv4Pool: "10.45.0.0/16", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if !strings.Contains(response.Error, "overlaps") {
			t.Fatalf("expected overlap error, got %q", response.Error)
		}
	})

	t.Run("subset overlap", func(t *testing.T) {
		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-d", IPv4Pool: "10.45.0.128/25", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if !strings.Contains(response.Error, "overlaps") {
			t.Fatalf("expected overlap error, got %q", response.Error)
		}
	})

	t.Run("non-overlapping succeeds", func(t *testing.T) {
		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-e", IPv4Pool: "10.46.0.0/24", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d (error: %s)", http.StatusCreated, statusCode, response.Error)
		}
	})
}

type IPAllocationItem struct {
	Address   string `json:"address"`
	IMSI      string `json:"imsi"`
	Type      string `json:"type"`
	SessionID *int   `json:"session_id"`
}

type ListIPAllocationsResult struct {
	Items      []IPAllocationItem `json:"items"`
	Page       int                `json:"page"`
	PerPage    int                `json:"per_page"`
	TotalCount int                `json:"total_count"`
}

type ListIPAllocationsResponse struct {
	Result ListIPAllocationsResult `json:"result"`
	Error  string                  `json:"error,omitempty"`
}

func listIPv4Allocations(url string, client *http.Client, token string, name string, page, perPage int) (int, *ListIPAllocationsResponse, error) {
	reqURL := fmt.Sprintf("%s/api/v1/networking/data-networks/%s/ipv4-allocations?page=%d&per_page=%d", url, name, page, perPage)

	req, err := http.NewRequestWithContext(context.Background(), "GET", reqURL, nil)
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

	var response ListIPAllocationsResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func TestListIPAllocationsEndpoint(t *testing.T) {
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

	t.Run("empty allocations for default data network", func(t *testing.T) {
		statusCode, response, err := listIPv4Allocations(env.Server.URL, client, token, "internet", 1, 25)
		if err != nil {
			t.Fatalf("couldn't list ip allocations: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.TotalCount != 0 {
			t.Fatalf("expected 0 allocations, got %d", response.Result.TotalCount)
		}

		if len(response.Result.Items) != 0 {
			t.Fatalf("expected empty items, got %d", len(response.Result.Items))
		}

		if response.Result.Page != 1 {
			t.Fatalf("expected page 1, got %d", response.Result.Page)
		}

		if response.Result.PerPage != 25 {
			t.Fatalf("expected per_page 25, got %d", response.Result.PerPage)
		}
	})

	t.Run("404 for unknown data network", func(t *testing.T) {
		statusCode, response, err := listIPv4Allocations(env.Server.URL, client, token, "nonexistent", 1, 25)
		if err != nil {
			t.Fatalf("couldn't list ip allocations: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "Data Network not found" {
			t.Fatalf("expected error %q, got %q", "Data Network not found", response.Error)
		}
	})

	t.Run("invalid page parameter", func(t *testing.T) {
		statusCode, _, err := listIPv4Allocations(env.Server.URL, client, token, "internet", 0, 25)
		if err != nil {
			t.Fatalf("couldn't list ip allocations: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("invalid per_page parameter", func(t *testing.T) {
		statusCode, _, err := listIPv4Allocations(env.Server.URL, client, token, "internet", 1, 101)
		if err != nil {
			t.Fatalf("couldn't list ip allocations: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})
}

func TestUpdateDataNetworkOverlappingPool(t *testing.T) {
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

	// Create two data networks with non-overlapping pools.
	_, _, err = createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: "dn-x", IPv4Pool: "10.50.0.0/24", DNS: DNS, MTU: MTU,
	})
	if err != nil {
		t.Fatalf("couldn't create first data network: %s", err)
	}

	_, _, err = createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: "dn-y", IPv4Pool: "10.51.0.0/24", DNS: DNS, MTU: MTU,
	})
	if err != nil {
		t.Fatalf("couldn't create second data network: %s", err)
	}

	t.Run("update to overlapping pool rejected", func(t *testing.T) {
		statusCode, response, err := editDataNetwork(env.Server.URL, client, "dn-y", token, &UpdateDataNetworkParams{
			IPv4Pool: "10.50.0.0/24", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't edit data network: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if !strings.Contains(response.Error, "overlaps") {
			t.Fatalf("expected overlap error, got %q", response.Error)
		}
	})

	t.Run("update own pool succeeds", func(t *testing.T) {
		statusCode, response, err := editDataNetwork(env.Server.URL, client, "dn-y", token, &UpdateDataNetworkParams{
			IPv4Pool: "10.51.0.0/22", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't edit data network: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d (error: %s)", http.StatusOK, statusCode, response.Error)
		}
	})
}

func TestCreateDataNetworkWithIPv6Pool(t *testing.T) {
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

	t.Run("create with valid IPv6 pool", func(t *testing.T) {
		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-v6-a", IPv4Pool: "10.60.0.0/24", IPv6Pool: "2001:db8::/48", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d (error: %s)", http.StatusCreated, statusCode, response.Error)
		}
	})

	t.Run("get data network with IPv6 allocation stats", func(t *testing.T) {
		statusCode, response, err := getDataNetwork(env.Server.URL, client, token, "dn-v6-a")
		if err != nil {
			t.Fatalf("couldn't get data network: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.IPv6Pool != "2001:db8::/48" {
			t.Fatalf("expected ipv6_pool %q, got %q", "2001:db8::/48", response.Result.IPv6Pool)
		}

		if response.Result.IPv6Allocation == nil {
			t.Fatal("expected ipv6_allocation to be present in detail response")
		}

		if response.Result.IPv6Allocation.PoolSize <= 0 {
			t.Fatalf("expected positive IPv6 pool_size, got %d", response.Result.IPv6Allocation.PoolSize)
		}

		if response.Result.IPv6Allocation.Allocated != 0 {
			t.Fatalf("expected 0 allocated IPv6 prefixes, got %d", response.Result.IPv6Allocation.Allocated)
		}

		if response.Result.IPv6Allocation.Available != response.Result.IPv6Allocation.PoolSize {
			t.Fatalf("expected available == pool_size when none allocated, got available=%d pool_size=%d",
				response.Result.IPv6Allocation.Available, response.Result.IPv6Allocation.PoolSize)
		}
	})

	t.Run("create without IPv6 pool omits ipv6_allocation", func(t *testing.T) {
		_, _, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-v4-only", IPv4Pool: "10.61.0.0/24", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		statusCode, response, err := getDataNetwork(env.Server.URL, client, token, "dn-v4-only")
		if err != nil {
			t.Fatalf("couldn't get data network: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.IPv6Allocation != nil {
			t.Fatal("expected ipv6_allocation to be nil for IPv4-only data network")
		}
	})
}

func TestCreateDataNetworkIPv6PoolValidation(t *testing.T) {
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

	expectedErr := "invalid ipv6_pool format, must be a valid IPv6 CIDR with prefix length between /48 and /60"

	tests := []struct {
		testName string
		ipv6Pool string
	}{
		{
			testName: "not a CIDR",
			ipv6Pool: "2001:db8::1",
		},
		{
			testName: "IPv4 address",
			ipv6Pool: "10.0.0.0/24",
		},
		{
			testName: "IPv4-mapped IPv6",
			ipv6Pool: "::ffff:10.0.0.0/96",
		},
		{
			testName: "prefix too short /32",
			ipv6Pool: "2001:db8::/32",
		},
		{
			testName: "prefix too long /64",
			ipv6Pool: "2001:db8:abcd:1234::/64",
		},
		{
			testName: "prefix too long /128",
			ipv6Pool: "2001:db8::1/128",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
				Name: "dn-bad-v6", IPv4Pool: "10.70.0.0/24", IPv6Pool: tt.ipv6Pool, DNS: DNS, MTU: MTU,
			})
			if err != nil {
				t.Fatalf("couldn't create data network: %s", err)
			}

			if statusCode != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
			}

			if response.Error != expectedErr {
				t.Fatalf("expected error %q, got %q", expectedErr, response.Error)
			}
		})
	}

	t.Run("valid /48", func(t *testing.T) {
		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-v6-48", IPv4Pool: "10.71.0.0/24", IPv6Pool: "2001:db8:1::/48", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d (error: %s)", http.StatusCreated, statusCode, response.Error)
		}
	})

	t.Run("valid /60", func(t *testing.T) {
		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-v6-60", IPv4Pool: "10.72.0.0/24", IPv6Pool: "2001:db8:2::/60", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d (error: %s)", http.StatusCreated, statusCode, response.Error)
		}
	})
}

func TestCreateDataNetworkIPv6PoolOverlap(t *testing.T) {
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

	// Create first data network with an IPv6 pool.
	_, _, err = createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: "dn-v6-first", IPv4Pool: "10.80.0.0/24", IPv6Pool: "2001:db8:abcd::/48", DNS: DNS, MTU: MTU,
	})
	if err != nil {
		t.Fatalf("couldn't create first data network: %s", err)
	}

	t.Run("exact IPv6 overlap", func(t *testing.T) {
		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-v6-dup", IPv4Pool: "10.81.0.0/24", IPv6Pool: "2001:db8:abcd::/48", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if !strings.Contains(response.Error, "overlaps") {
			t.Fatalf("expected overlap error, got %q", response.Error)
		}
	})

	t.Run("subset IPv6 overlap", func(t *testing.T) {
		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-v6-sub", IPv4Pool: "10.82.0.0/24", IPv6Pool: "2001:db8:abcd:0010::/60", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if !strings.Contains(response.Error, "overlaps") {
			t.Fatalf("expected overlap error, got %q", response.Error)
		}
	})

	t.Run("non-overlapping IPv6 succeeds", func(t *testing.T) {
		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
			Name: "dn-v6-ok", IPv4Pool: "10.83.0.0/24", IPv6Pool: "2001:db8:cafe::/48", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d (error: %s)", http.StatusCreated, statusCode, response.Error)
		}
	})
}

func TestUpdateDataNetworkIPv6PoolOverlap(t *testing.T) {
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

	// Create two data networks with non-overlapping IPv6 pools.
	_, _, err = createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: "dn-v6-u1", IPv4Pool: "10.90.0.0/24", IPv6Pool: "2001:db8:1::/48", DNS: DNS, MTU: MTU,
	})
	if err != nil {
		t.Fatalf("couldn't create first data network: %s", err)
	}

	_, _, err = createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: "dn-v6-u2", IPv4Pool: "10.91.0.0/24", IPv6Pool: "2001:db8:2::/48", DNS: DNS, MTU: MTU,
	})
	if err != nil {
		t.Fatalf("couldn't create second data network: %s", err)
	}

	t.Run("update to overlapping IPv6 pool rejected", func(t *testing.T) {
		statusCode, response, err := editDataNetwork(env.Server.URL, client, "dn-v6-u2", token, &UpdateDataNetworkParams{
			IPv4Pool: "10.91.0.0/24", IPv6Pool: "2001:db8:1::/48", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't edit data network: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if !strings.Contains(response.Error, "overlaps") {
			t.Fatalf("expected overlap error, got %q", response.Error)
		}
	})

	t.Run("update own IPv6 pool succeeds", func(t *testing.T) {
		statusCode, response, err := editDataNetwork(env.Server.URL, client, "dn-v6-u2", token, &UpdateDataNetworkParams{
			IPv4Pool: "10.91.0.0/24", IPv6Pool: "2001:db8:2::/52", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("couldn't edit data network: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d (error: %s)", http.StatusOK, statusCode, response.Error)
		}
	})
}
