package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type ListUPFsResponseResult []int

type ListUPFsResponse struct {
	Error  string                 `json:"error,omitempty"`
	Result ListUPFsResponseResult `json:"result"`
}

type CreateUPFParams struct {
	Name           string `json:"name"`
	NetworkSliceId int64  `json:"network_slice_id"`
}

type createUPFResponseResult struct {
	ID int64 `json:"id"`
}

type createUPFResponse struct {
	Error  string                  `json:"error,omitempty"`
	Result createUPFResponseResult `json:"result"`
}

type GetUPFResponseResult struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	NetworkSliceId int64  `json:"network_slice_id"`
}

type GetUPFResponse struct {
	Error  string               `json:"error,omitempty"`
	Result GetUPFResponseResult `json:"result"`
}

type DeleteUPFResponseResult struct {
	ID int64 `json:"id"`
}

type DeleteUPFResponse struct {
	Error  string                  `json:"error,omitempty"`
	Result DeleteUPFResponseResult `json:"result"`
}

func listUPFs(url string, client *http.Client) (int, *ListUPFsResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/inventory/upfs", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var listResponse ListUPFsResponse
	if err := json.NewDecoder(res.Body).Decode(&listResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &listResponse, nil
}

func getUPF(url string, client *http.Client, id string) (int, *GetUPFResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/inventory/upfs/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var getResponse GetUPFResponse
	if err := json.NewDecoder(res.Body).Decode(&getResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &getResponse, nil
}

func createUPF(url string, client *http.Client, data *CreateUPFParams) (int, *createUPFResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url+"/api/v1/inventory/upfs", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse createUPFResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteUPF(url string, client *http.Client, id string) (int, *DeleteUPFResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/inventory/upfs/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var deleteResponse DeleteUPFResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteResponse, nil
}

func TestUPFsHandlers(t *testing.T) {
	ts, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("List inventory upfs - 0", func(t *testing.T) {
		statusCode, response, err := listUPFs(ts.URL, client)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if len(response.Result) != 0 {
			t.Fatalf("expected result %v, got %v", []int{}, response.Result)
		}
	})

	t.Run("Create inventory upf - 1", func(t *testing.T) {
		data := CreateUPFParams{
			Name: "Name1",
		}
		statusCode, response, err := createUPF(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.ID != 1 {
			t.Fatalf("expected id %d, got %d", 1, response.Result.ID)
		}
	})

	t.Run("Create upf with non-existent network slice", func(t *testing.T) {
		data := CreateUPFParams{
			Name:           "Name2",
			NetworkSliceId: 1,
		}
		statusCode, response, _ := createUPF(ts.URL, client, &data)

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "network slice not found" {
			t.Fatalf("expected error %q, got %q", "network slice not found", response.Error)
		}
	})

	t.Run("Create network slice - 1", func(t *testing.T) {
		data := CreateNetworkSliceParams{
			Name:     "Name1",
			Sst:      1,
			Sd:       "Sd1",
			SiteName: "SiteName1",
			Mcc:      "Mcc1",
			Mnc:      "Mnc1",
		}
		statusCode, response, err := createNetworkSlice(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.ID != 1 {
			t.Fatalf("expected id %d, got %d", 1, response.Result.ID)
		}
	})

	t.Run("Create inventory upf (with network slice)- 2", func(t *testing.T) {
		data := CreateUPFParams{
			Name:           "Name2",
			NetworkSliceId: 1,
		}
		statusCode, response, err := createUPF(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.ID != 2 {
			t.Fatalf("expected id %d, got %d", 2, response.Result.ID)
		}
	})

	t.Run("List inventory upfs - 2", func(t *testing.T) {
		statusCode, response, err := listUPFs(ts.URL, client)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if len(response.Result) != 2 {
			t.Fatalf("expected result %v, got %v", []int{1, 2}, response.Result)
		}
	})

	t.Run("Get inventory upf - 1", func(t *testing.T) {
		statusCode, response, err := getUPF(ts.URL, client, "1")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if response.Result.ID != 1 {
			t.Fatalf("expected id %d, got %d", 1, response.Result.ID)
		}

		if response.Result.Name != "Name1" {
			t.Fatalf("expected name %q, got %q", "Name1", response.Result.Name)
		}

		if response.Result.NetworkSliceId != 0 {
			t.Fatalf("expected network_slice_id %d, got %d", 0, response.Result.NetworkSliceId)
		}
	})

	t.Run("Get inventory upf - 2", func(t *testing.T) {
		statusCode, response, err := getUPF(ts.URL, client, "2")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if response.Result.ID != 2 {
			t.Fatalf("expected id %d, got %d", 2, response.Result.ID)
		}

		if response.Result.Name != "Name2" {
			t.Fatalf("expected name %q, got %q", "Name2", response.Result.Name)
		}

		if response.Result.NetworkSliceId != 1 {
			t.Fatalf("expected network_slice_id %d, got %d", 1, response.Result.NetworkSliceId)
		}
	})

	t.Run("Delete inventory upf - 1", func(t *testing.T) {
		statusCode, response, err := deleteUPF(ts.URL, client, "1")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusAccepted {
			t.Fatalf("expected status %d, got %d", http.StatusAccepted, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if response.Result.ID != 1 {
			t.Fatalf("expected id %d, got %d", 1, response.Result.ID)
		}
	})

	t.Run("List inventory upfs - 1", func(t *testing.T) {
		statusCode, response, err := listUPFs(ts.URL, client)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if len(response.Result) != 1 {
			t.Fatalf("expected result %v, got %v", []int{2}, response.Result)
		}
	})
}
