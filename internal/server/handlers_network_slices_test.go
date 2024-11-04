package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type ListNetworkSlicesResponseResult []int

type ListNetworkSlicesResponse struct {
	Error  string                          `json:"error,omitempty"`
	Result ListNetworkSlicesResponseResult `json:"result"`
}

type CreateNetworkSliceParams struct {
	Name     string `json:"name"`
	Sst      int32  `json:"sst"`
	Sd       string `json:"sd"`
	SiteName string `json:"site_name"`
	Mcc      string `json:"mcc"`
	Mnc      string `json:"mnc"`
}

type createNetworkSliceResponseResult struct {
	ID int64 `json:"id"`
}

type createNetworkSliceResponse struct {
	Error  string                           `json:"error,omitempty"`
	Result createNetworkSliceResponseResult `json:"result"`
}

type GetNetworkSliceResponseResult struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Sst      int32  `json:"sst"`
	Sd       string `json:"sd"`
	SiteName string `json:"site_name"`
	Mcc      string `json:"mcc"`
	Mnc      string `json:"mnc"`
}

type GetNetworkSliceResponse struct {
	Error  string                        `json:"error,omitempty"`
	Result GetNetworkSliceResponseResult `json:"result"`
}

type DeleteNetworkSliceResponseResult struct {
	ID int64 `json:"id"`
}

type DeleteNetworkSliceResponse struct {
	Error  string                           `json:"error,omitempty"`
	Result DeleteNetworkSliceResponseResult `json:"result"`
}

func listNetworkSlices(url string, client *http.Client) (int, *ListNetworkSlicesResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/network-slices", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var listResponse ListNetworkSlicesResponse
	if err := json.NewDecoder(res.Body).Decode(&listResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &listResponse, nil
}

func getNetworkSlice(url string, client *http.Client, id string) (int, *GetNetworkSliceResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/network-slices/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var getResponse GetNetworkSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&getResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &getResponse, nil
}

func createNetworkSlice(url string, client *http.Client, data *CreateNetworkSliceParams) (int, *createNetworkSliceResponse, error) {
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
	var createResponse createNetworkSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteNetworkSlice(url string, client *http.Client, id string) (int, *DeleteNetworkSliceResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/network-slices/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var deleteResponse DeleteNetworkSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteResponse, nil
}

func TestNetworkSlicesHandlers(t *testing.T) {
	ts, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("List network slices - 0", func(t *testing.T) {
		statusCode, response, err := listNetworkSlices(ts.URL, client)
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

	t.Run("Create network slice - 1", func(t *testing.T) {
		data := CreateNetworkSliceParams{
			Name:     "Name1",
			SiteName: "SiteName1",
			Sst:      1,
			Sd:       "SD1",
			Mcc:      "MCC1",
			Mnc:      "MNC1",
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

	t.Run("Create network slice - 2", func(t *testing.T) {
		data := CreateNetworkSliceParams{
			Name:     "Name2",
			SiteName: "SiteName2",
			Sst:      2,
			Sd:       "SD2",
			Mcc:      "MCC2",
			Mnc:      "MNC2",
		}
		statusCode, response, err := createNetworkSlice(ts.URL, client, &data)
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

	t.Run("List network slices - 2", func(t *testing.T) {
		statusCode, response, err := listNetworkSlices(ts.URL, client)
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

	t.Run("Get network slice - 1", func(t *testing.T) {
		statusCode, response, err := getNetworkSlice(ts.URL, client, "1")
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

		if response.Result.SiteName != "SiteName1" {
			t.Fatalf("expected site_name %q, got %q", "SiteName1", response.Result.SiteName)
		}

		if response.Result.Sst != 1 {
			t.Fatalf("expected sst %q, got %q", 1, response.Result.Sst)
		}

		if response.Result.Sd != "SD1" {
			t.Fatalf("expected sd %q, got %q", "SD1", response.Result.Sd)
		}

		if response.Result.Mcc != "MCC1" {
			t.Fatalf("expected mcc %q, got %q", "MCC1", response.Result.Mcc)
		}

		if response.Result.Mnc != "MNC1" {
			t.Fatalf("expected mnc %q, got %q", "MNC1", response.Result.Mnc)
		}
	})

	t.Run("Get network slice - 2", func(t *testing.T) {
		statusCode, response, err := getNetworkSlice(ts.URL, client, "2")
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

		if response.Result.SiteName != "SiteName2" {
			t.Fatalf("expected site_name %q, got %q", "SiteName2", response.Result.SiteName)
		}

		if response.Result.Sst != 2 {
			t.Fatalf("expected sst %q, got %q", 2, response.Result.Sst)
		}

		if response.Result.Sd != "SD2" {
			t.Fatalf("expected sd %q, got %q", "SD2", response.Result.Sd)
		}

		if response.Result.Mcc != "MCC2" {
			t.Fatalf("expected mcc %q, got %q", "MCC2", response.Result.Mcc)
		}
	})

	t.Run("Delete network slice - 1", func(t *testing.T) {
		statusCode, response, err := deleteNetworkSlice(ts.URL, client, "1")
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

	t.Run("List network slices - 1", func(t *testing.T) {
		statusCode, response, err := listNetworkSlices(ts.URL, client)
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
