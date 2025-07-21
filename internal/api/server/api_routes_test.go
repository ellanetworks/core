package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const (
	Destination = "1.1.1.0/24"
	Gateway     = "1.2.3.4"
	Interface   = "n3"
	Metric      = 100
)

type CreateRouteResponseResult struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
}

type GetRouteResponseResult struct {
	ID          int64  `json:"id"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

type GetRouteResponse struct {
	Result GetRouteResponseResult `json:"result"`
	Error  string                 `json:"error,omitempty"`
}

type CreateRouteParams struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

type CreateRouteResponse struct {
	Result CreateRouteResponseResult `json:"result"`
	Error  string                    `json:"error,omitempty"`
}

type DeleteRouteResponseResult struct {
	Message string `json:"message"`
}

type DeleteRouteResponse struct {
	Result DeleteRouteResponseResult `json:"result"`
	Error  string                    `json:"error,omitempty"`
}

type ListRouteResponse struct {
	Result []GetRouteResponse `json:"result"`
	Error  string             `json:"error,omitempty"`
}

func listRoutes(url string, client *http.Client, token string) (int, *ListRouteResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/routes", nil)
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
	var routeResponse ListRouteResponse
	if err := json.NewDecoder(res.Body).Decode(&routeResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &routeResponse, nil
}

func getRoute(url string, client *http.Client, token string, id int64) (int, *GetRouteResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/routes/%d", url, id), nil)
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
	var routeResponse GetRouteResponse
	if err := json.NewDecoder(res.Body).Decode(&routeResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &routeResponse, nil
}

func createRoute(url string, client *http.Client, token string, data *CreateRouteParams) (int, *CreateRouteResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/routes", strings.NewReader(string(body)))
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
	var createResponse CreateRouteResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteRoute(url string, client *http.Client, token string, id int64) (int, *DeleteRouteResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", fmt.Sprintf("%s/api/v1/routes/%d", url, id), nil)
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
	var deleteRouteResponse DeleteRouteResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteRouteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteRouteResponse, nil
}

// This is an end-to-end test for the routes handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestAPIRoutesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath, ReqsPerSec)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. List routes - 0", func(t *testing.T) {
		statusCode, response, err := listRoutes(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list route: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(response.Result) != 0 {
			t.Fatalf("expected 0 routes, got %d", len(response.Result))
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("2. Create route", func(t *testing.T) {
		createRouteParams := &CreateRouteParams{
			Destination: Destination,
			Gateway:     Gateway,
			Interface:   Interface,
			Metric:      Metric,
		}
		statusCode, response, err := createRoute(ts.URL, client, token, createRouteParams)
		if err != nil {
			t.Fatalf("couldn't create route: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Route created successfully" {
			t.Fatalf("expected message 'Route created successfully', got %q", response.Result.Message)
		}
	})

	t.Run("3. List routes - 1", func(t *testing.T) {
		statusCode, response, err := listRoutes(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list route: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(response.Result) != 1 {
			t.Fatalf("expected 1 route, got %d", len(response.Result))
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("4. Get route", func(t *testing.T) {
		statusCode, response, err := getRoute(ts.URL, client, token, 1)
		if err != nil {
			t.Fatalf("couldn't get route: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Destination != Destination {
			t.Fatalf("expected destination %s, got %s", Destination, response.Result.Destination)
		}
		if response.Result.Gateway != Gateway {
			t.Fatalf("expected gateway %s, got %s", Gateway, response.Result.Gateway)
		}
		if response.Result.Interface != Interface {
			t.Fatalf("expected interface %s, got %s", Interface, response.Result.Interface)
		}
		if response.Result.Metric != Metric {
			t.Fatalf("expected metric %d, got %d", Metric, response.Result.Metric)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("5. Get route - id not found", func(t *testing.T) {
		statusCode, response, err := getRoute(ts.URL, client, token, 2)
		if err != nil {
			t.Fatalf("couldn't get route: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "Route not found" {
			t.Fatalf("expected error %q, got %q", "Route not found", response.Error)
		}
	})

	t.Run("5. Create route - no destination", func(t *testing.T) {
		createRouteParams := &CreateRouteParams{}
		statusCode, response, err := createRoute(ts.URL, client, token, createRouteParams)
		if err != nil {
			t.Fatalf("couldn't create route: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "destination is missing" {
			t.Fatalf("expected error %q, got %q", "destination is missing", response.Error)
		}
	})

	t.Run("8. Delete route - success", func(t *testing.T) {
		statusCode, response, err := deleteRoute(ts.URL, client, token, 1)
		if err != nil {
			t.Fatalf("couldn't delete route: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("9. Delete route - no route", func(t *testing.T) {
		statusCode, response, err := deleteRoute(ts.URL, client, token, 1)
		if err != nil {
			t.Fatalf("couldn't delete route: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "Route not found" {
			t.Fatalf("expected error %q, got %q", "Route not found", response.Error)
		}
	})
}

func TestCreateRouteInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath, ReqsPerSec)
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
		destination      string
		gateway          string
		networkInterface string
		metric           int
		error            string
	}{
		{
			destination:      "",
			gateway:          Gateway,
			networkInterface: Interface,
			metric:           Metric,
			error:            "destination is missing",
		},
		{
			destination:      "abcdef",
			gateway:          Gateway,
			networkInterface: Interface,
			metric:           Metric,
			error:            "invalid destination format: expecting CIDR notation",
		},
		{
			destination:      Destination,
			gateway:          "",
			networkInterface: Interface,
			metric:           Metric,
			error:            "gateway is missing",
		},
		{
			destination:      Destination,
			gateway:          "abcdef",
			networkInterface: Interface,
			metric:           Metric,
			error:            "invalid gateway format: expecting an IPv4 address",
		},
		{
			destination:      Destination,
			gateway:          Gateway,
			networkInterface: "",
			metric:           Metric,
			error:            "interface is missing",
		},
		{
			destination:      Destination,
			gateway:          Gateway,
			networkInterface: "abcdef",
			metric:           Metric,
			error:            "invalid interface: abcdef: only n3 and n6 are allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.destination, func(t *testing.T) {
			createRouteParams := &CreateRouteParams{
				Destination: tt.destination,
				Gateway:     tt.gateway,
				Interface:   tt.networkInterface,
				Metric:      tt.metric,
			}
			statusCode, response, err := createRoute(ts.URL, client, token, createRouteParams)
			if err != nil {
				t.Fatalf("couldn't create route: %s", err)
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
