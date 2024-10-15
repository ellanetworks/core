package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type ListNetworkSliceGnbsResponseResult []int

type ListNetworkSliceGnbsResponse struct {
	Error  string                             `json:"error,omitempty"`
	Result ListNetworkSliceGnbsResponseResult `json:"result"`
}

type CreateNetworkSliceGnbParams struct {
	GnbID int `json:"gnb_id"`
}

type CreateNetworkSliceGnbResponseResult struct {
	GnbID int `json:"gnb_id"`
}

type CreateNetworkSliceGnbResponse struct {
	Error  string                              `json:"error,omitempty"`
	Result CreateNetworkSliceGnbResponseResult `json:"result"`
}

type DeleteNetworkSliceGnbResponseResult struct {
	GnbID int `json:"gnb_id"`
}

type DeleteNetworkSliceGnbResponse struct {
	Error  string                              `json:"error,omitempty"`
	Result DeleteNetworkSliceGnbResponseResult `json:"result"`
}

func listNetworkSliceGnbs(url string, client *http.Client) (int, *ListNetworkSliceGnbsResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/network-slices/1/gnbs", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var listResponse ListNetworkSliceGnbsResponse
	if err := json.NewDecoder(res.Body).Decode(&listResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &listResponse, nil
}

func createNetworkSliceGnb(url string, client *http.Client, data *CreateNetworkSliceGnbParams) (int, *CreateNetworkSliceGnbResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url+"/api/v1/network-slices/1/gnbs", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse CreateNetworkSliceGnbResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteNetworkSliceGnb(url string, client *http.Client, id, gnbID string) (int, *DeleteNetworkSliceGnbResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/network-slices/"+id+"/gnbs/"+gnbID, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var deleteResponse DeleteNetworkSliceGnbResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteResponse, nil
}

func TestNetworkSliceGnbsHandlers(t *testing.T) {
	ts, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("List network slice gnbs - not found", func(t *testing.T) {
		statusCode, response, err := listNetworkSliceGnbs(ts.URL, client)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "network slice not found" {
			t.Fatalf("expected error %q, got %q", "network slice not found", response.Error)
		}

		if len(response.Result) != 0 {
			t.Fatalf("expected result %v, got %v", []int{}, response.Result)
		}
	})

	t.Run("Create network slice", func(t *testing.T) {
		data := CreateNetworkSliceParams{
			Name:     "Name1",
			Sst:      "Sst1",
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

	t.Run("List network slice gnbs - 0", func(t *testing.T) {
		statusCode, response, err := listNetworkSliceGnbs(ts.URL, client)
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

	t.Run("Create network slice gnb - gnb does not exist", func(t *testing.T) {
		data := CreateNetworkSliceGnbParams{
			GnbID: 1,
		}
		statusCode, response, err := createNetworkSliceGnb(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "gnb not found" {
			t.Fatalf("expected error %q, got %q", "gnb not found", response.Error)
		}
	})

	t.Run("Create gnb - 1", func(t *testing.T) {
		data := CreateGnbParams{
			Name: "Name1",
			Tac:  "Tac1",
		}
		statusCode, response, err := createGnb(ts.URL, client, &data)
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

	t.Run("Create gnb - 2", func(t *testing.T) {
		data := CreateGnbParams{
			Name: "Name2",
			Tac:  "Tac2",
		}
		statusCode, response, err := createGnb(ts.URL, client, &data)
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

	t.Run("Create gnb - 3", func(t *testing.T) {
		data := CreateGnbParams{
			Name: "Name3",
			Tac:  "Tac3",
		}
		statusCode, response, err := createGnb(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.ID != 3 {
			t.Fatalf("expected id %d, got %d", 3, response.Result.ID)
		}
	})

	t.Run("Create network slice gnb - 1", func(t *testing.T) {
		data := CreateNetworkSliceGnbParams{
			GnbID: 1,
		}
		statusCode, response, err := createNetworkSliceGnb(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.GnbID != 1 {
			t.Fatalf("expected gnb_id %d, got %d", 2, response.Result.GnbID)
		}
	})

	t.Run("Create network slice gnb - 2", func(t *testing.T) {
		data := CreateNetworkSliceGnbParams{
			GnbID: 2,
		}
		statusCode, response, err := createNetworkSliceGnb(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.GnbID != 2 {
			t.Fatalf("expected gnb_id %d, got %d", 2, response.Result.GnbID)
		}
	})

	t.Run("Create network slice gnb - 3", func(t *testing.T) {
		data := CreateNetworkSliceGnbParams{
			GnbID: 3,
		}
		statusCode, response, err := createNetworkSliceGnb(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.GnbID != 3 {
			t.Fatalf("expected gnb_id %d, got %d", 3, response.Result.GnbID)
		}
	})

	t.Run("Create network slice gnb - gnb already exists", func(t *testing.T) {
		data := CreateNetworkSliceGnbParams{
			GnbID: 1,
		}
		statusCode, response, err := createNetworkSliceGnb(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, statusCode)
		}

		if response.Error != "gnb already in network slice" {
			t.Fatalf("expected error %q, got %q", "gnb already in network slice", response.Error)
		}
	})

	t.Run("List network slice gnbs - 3", func(t *testing.T) {
		statusCode, response, err := listNetworkSliceGnbs(ts.URL, client)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if len(response.Result) != 3 {
			t.Fatalf("expected result %v, got %v", []int{1, 2, 3}, response.Result)
		}
	})

	t.Run("Delete gnb - 1", func(t *testing.T) {
		statusCode, response, err := deleteGnb(ts.URL, client, "1")
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

	t.Run("List network slice gnbs - 1", func(t *testing.T) {
		statusCode, response, err := listNetworkSliceGnbs(ts.URL, client)
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
			t.Fatalf("expected result %v, got %v", []int{2, 3}, response.Result)
		}
	})

	t.Run("Delete network slice gnb - 2", func(t *testing.T) {
		statusCode, response, err := deleteNetworkSliceGnb(ts.URL, client, "1", "2")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusAccepted {
			t.Fatalf("expected status %d, got %d", http.StatusAccepted, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if response.Result.GnbID != 2 {
			t.Fatalf("expected gnb_id %d, got %d", 2, response.Result.GnbID)
		}
	})

	t.Run("List network slice gnbs - 1", func(t *testing.T) {
		statusCode, response, err := listNetworkSliceGnbs(ts.URL, client)
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
			t.Fatalf("expected result %v, got %v", []int{3}, response.Result)
		}
	})

	t.Run("List gnbs - 2", func(t *testing.T) {
		statusCode, response, err := listGnbs(ts.URL, client)
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
			t.Fatalf("expected result %v, got %v", []int{2, 3}, response.Result)
		}
	})

	t.Run("Get gnb - 2 - should still exist", func(t *testing.T) {
		statusCode, _, err := getGnb(ts.URL, client, "2")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
	})
}
