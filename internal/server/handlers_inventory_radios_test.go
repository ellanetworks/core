package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type ListRadiosResponseResult []int

type ListRadiosResponse struct {
	Error  string                   `json:"error,omitempty"`
	Result ListRadiosResponseResult `json:"result"`
}

type CreateRadioParams struct {
	Name           string `json:"name"`
	Tac            string `json:"tac"`
	NetworkSliceId int64  `json:"network_slice_id"`
}

type createRadioResponseResult struct {
	ID int64 `json:"id"`
}

type createRadioResponse struct {
	Error  string                    `json:"error,omitempty"`
	Result createRadioResponseResult `json:"result"`
}

type GetRadioResponseResult struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Tac            string `json:"tac"`
	NetworkSliceId int64  `json:"network_slice_id"`
}

type GetRadioResponse struct {
	Error  string                 `json:"error,omitempty"`
	Result GetRadioResponseResult `json:"result"`
}

type DeleteRadioResponseResult struct {
	ID int64 `json:"id"`
}

type DeleteRadioResponse struct {
	Error  string                    `json:"error,omitempty"`
	Result DeleteRadioResponseResult `json:"result"`
}

func listRadios(url string, client *http.Client) (int, *ListRadiosResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/inventory/radios", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var listResponse ListRadiosResponse
	if err := json.NewDecoder(res.Body).Decode(&listResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &listResponse, nil
}

func getRadio(url string, client *http.Client, id string) (int, *GetRadioResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/inventory/radios/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var getResponse GetRadioResponse
	if err := json.NewDecoder(res.Body).Decode(&getResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &getResponse, nil
}

func createRadio(url string, client *http.Client, data *CreateRadioParams) (int, *createRadioResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url+"/api/v1/inventory/radios", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse createRadioResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteRadio(url string, client *http.Client, id string) (int, *DeleteRadioResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/inventory/radios/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var deleteResponse DeleteRadioResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteResponse, nil
}

func TestRadiosHandlers(t *testing.T) {
	ts, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("List inventory radios - 0", func(t *testing.T) {
		statusCode, response, err := listRadios(ts.URL, client)
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

	t.Run("Create inventory radio - 1", func(t *testing.T) {
		data := CreateRadioParams{
			Name: "Name1",
			Tac:  "Tac1",
		}
		statusCode, response, err := createRadio(ts.URL, client, &data)
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

	t.Run("Create radio with non-existent network slice", func(t *testing.T) {
		data := CreateRadioParams{
			Name:           "Name2",
			Tac:            "Tac2",
			NetworkSliceId: 1,
		}
		statusCode, response, _ := createRadio(ts.URL, client, &data)

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

	t.Run("Create inventory radio (with network slice)- 2", func(t *testing.T) {
		data := CreateRadioParams{
			Name:           "Name2",
			Tac:            "Tac2",
			NetworkSliceId: 1,
		}
		statusCode, response, err := createRadio(ts.URL, client, &data)
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

	t.Run("List inventory radios - 2", func(t *testing.T) {
		statusCode, response, err := listRadios(ts.URL, client)
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

	t.Run("Get inventory radio - 1", func(t *testing.T) {
		statusCode, response, err := getRadio(ts.URL, client, "1")
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

		if response.Result.Tac != "Tac1" {
			t.Fatalf("expected tac %q, got %q", "Tac1", response.Result.Tac)
		}

		if response.Result.NetworkSliceId != 0 {
			t.Fatalf("expected network_slice_id %d, got %d", 0, response.Result.NetworkSliceId)
		}
	})

	t.Run("Get inventory radio - 2", func(t *testing.T) {
		statusCode, response, err := getRadio(ts.URL, client, "2")
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

		if response.Result.Tac != "Tac2" {
			t.Fatalf("expected tac %q, got %q", "Tac2", response.Result.Tac)
		}

		if response.Result.NetworkSliceId != 1 {
			t.Fatalf("expected network_slice_id %d, got %d", 1, response.Result.NetworkSliceId)
		}
	})

	t.Run("Delete inventory radio - 1", func(t *testing.T) {
		statusCode, response, err := deleteRadio(ts.URL, client, "1")
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

	t.Run("List inventory radios - 1", func(t *testing.T) {
		statusCode, response, err := listRadios(ts.URL, client)
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
