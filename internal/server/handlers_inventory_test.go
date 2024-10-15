package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type ListGnbsResponseResult []int

type ListGnbsResponse struct {
	Error  string                 `json:"error,omitempty"`
	Result ListGnbsResponseResult `json:"result"`
}

type CreateGnbParams struct {
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

type createGnbResponseResult struct {
	ID int64 `json:"id"`
}

type createGnbResponse struct {
	Error  string                  `json:"error,omitempty"`
	Result createGnbResponseResult `json:"result"`
}

type GetGnbResponseResult struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

type GetGnbResponse struct {
	Error  string               `json:"error,omitempty"`
	Result GetGnbResponseResult `json:"result"`
}

type DeleteGnbResponseResult struct {
	ID int64 `json:"id"`
}

type DeleteGnbResponse struct {
	Error  string                  `json:"error,omitempty"`
	Result DeleteGnbResponseResult `json:"result"`
}

func listGnbs(url string, client *http.Client) (int, *ListGnbsResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/inventory/gnbs", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var listResponse ListGnbsResponse
	if err := json.NewDecoder(res.Body).Decode(&listResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &listResponse, nil
}

func getGnb(url string, client *http.Client, id string) (int, *GetGnbResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/inventory/gnbs/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var getResponse GetGnbResponse
	if err := json.NewDecoder(res.Body).Decode(&getResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &getResponse, nil
}

func createGnb(url string, client *http.Client, data *CreateGnbParams) (int, *createGnbResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url+"/api/v1/inventory/gnbs", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse createGnbResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteGnb(url string, client *http.Client, id string) (int, *DeleteGnbResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/inventory/gnbs/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var deleteResponse DeleteGnbResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteResponse, nil
}

func TestGnbsHandlers(t *testing.T) {
	ts, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("List inventory gnbs - 0", func(t *testing.T) {
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

		if len(response.Result) != 0 {
			t.Fatalf("expected result %v, got %v", []int{}, response.Result)
		}
	})

	t.Run("Create inventory gnb - 1", func(t *testing.T) {
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

	t.Run("Create inventory gnb - 2", func(t *testing.T) {
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

	t.Run("List inventory gnbs - 2", func(t *testing.T) {
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
			t.Fatalf("expected result %v, got %v", []int{1, 2}, response.Result)
		}
	})

	t.Run("Get inventory gnb - 1", func(t *testing.T) {
		statusCode, response, err := getGnb(ts.URL, client, "1")
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
	})

	t.Run("Get inventory gnb - 2", func(t *testing.T) {
		statusCode, response, err := getGnb(ts.URL, client, "2")
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
	})

	t.Run("Delete inventory gnb - 1", func(t *testing.T) {
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

	t.Run("List inventory gnbs - 1", func(t *testing.T) {
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

		if len(response.Result) != 1 {
			t.Fatalf("expected result %v, got %v", []int{2}, response.Result)
		}
	})
}
