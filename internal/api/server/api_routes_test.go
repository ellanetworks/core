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
	ID      string `json:"id"` // CreateSuccessResponse.id is a string (decimal-formatted route id for local-only routes table)
	Message string `json:"message"`
}

type Route struct {
	ID          int64  `json:"id"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

type GetRouteResponse struct {
	Result Route  `json:"result"`
	Error  string `json:"error,omitempty"`
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

type ListRoutesResponseResult struct {
	Items      []Route `json:"items"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	TotalCount int     `json:"total_count"`
}

type ListRouteResponse struct {
	Result ListRoutesResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

func listRoutes(url string, client *http.Client, token string, page int, perPage int) (int, *ListRouteResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/networking/routes?page=%d&per_page=%d", url, page, perPage), nil)
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
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/networking/routes/%d", url, id), nil)
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

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/networking/routes", strings.NewReader(string(body)))
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
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", fmt.Sprintf("%s/api/v1/networking/routes/%d", url, id), nil)
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

	t.Run("1. List routes - 0", func(t *testing.T) {
		statusCode, response, err := listRoutes(env.Server.URL, client, token, 1, 10)
		if err != nil {
			t.Fatalf("couldn't list route: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 0 {
			t.Fatalf("expected 0 routes, got %d", len(response.Result.Items))
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

		statusCode, response, err := createRoute(env.Server.URL, client, token, createRouteParams)
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
		statusCode, response, err := listRoutes(env.Server.URL, client, token, 1, 10)
		if err != nil {
			t.Fatalf("couldn't list route: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 1 {
			t.Fatalf("expected 1 route, got %d", len(response.Result.Items))
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("4. Get route", func(t *testing.T) {
		statusCode, response, err := getRoute(env.Server.URL, client, token, 1)
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
		statusCode, response, err := getRoute(env.Server.URL, client, token, 2)
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

		statusCode, response, err := createRoute(env.Server.URL, client, token, createRouteParams)
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
		statusCode, response, err := deleteRoute(env.Server.URL, client, token, 1)
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
		statusCode, response, err := deleteRoute(env.Server.URL, client, token, 1)
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
			error:            "invalid gateway format: expecting an IPv4 or IPv6 address",
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
			error:            "invalid interface: only n3 and n6 are allowed",
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

			statusCode, response, err := createRoute(env.Server.URL, client, token, createRouteParams)
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

func TestCreateRouteIPv6(t *testing.T) {
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

	t.Run("create route with IPv6 destination and gateway", func(t *testing.T) {
		createRouteParams := &CreateRouteParams{
			Destination: "fd45::/48",
			Gateway:     "fd00:6::2",
			Interface:   "n6",
			Metric:      0,
		}

		statusCode, response, err := createRoute(env.Server.URL, client, token, createRouteParams)
		if err != nil {
			t.Fatalf("couldn't create route: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error: %q", response.Error)
		}

		if response.Result.Message != "Route created successfully" {
			t.Fatalf("expected message 'Route created successfully', got %q", response.Result.Message)
		}
	})

	t.Run("list routes - 1 IPv6 route", func(t *testing.T) {
		statusCode, response, err := listRoutes(env.Server.URL, client, token, 1, 10)
		if err != nil {
			t.Fatalf("couldn't list routes: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 1 {
			t.Fatalf("expected 1 route, got %d", len(response.Result.Items))
		}

		if response.Error != "" {
			t.Fatalf("unexpected error: %q", response.Error)
		}

		route := response.Result.Items[0]
		if route.Destination != "fd45::/48" {
			t.Fatalf("expected destination fd45::/48, got %s", route.Destination)
		}

		if route.Gateway != "fd00:6::2" {
			t.Fatalf("expected gateway fd00:6::2, got %s", route.Gateway)
		}

		if route.Interface != "n6" {
			t.Fatalf("expected interface n6, got %s", route.Interface)
		}

		if route.Metric != 0 {
			t.Fatalf("expected metric 0, got %d", route.Metric)
		}
	})

	t.Run("get route - IPv6", func(t *testing.T) {
		statusCode, response, err := getRoute(env.Server.URL, client, token, 1)
		if err != nil {
			t.Fatalf("couldn't get route: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Destination != "fd45::/48" {
			t.Fatalf("expected destination fd45::/48, got %s", response.Result.Destination)
		}

		if response.Result.Gateway != "fd00:6::2" {
			t.Fatalf("expected gateway fd00:6::2, got %s", response.Result.Gateway)
		}

		if response.Result.Interface != "n6" {
			t.Fatalf("expected interface n6, got %s", response.Result.Interface)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error: %q", response.Error)
		}
	})

	t.Run("delete route - IPv6", func(t *testing.T) {
		statusCode, response, err := deleteRoute(env.Server.URL, client, token, 1)
		if err != nil {
			t.Fatalf("couldn't delete route: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error: %q", response.Error)
		}
	})
}

func TestCreateInvalidIPv6(t *testing.T) {
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
		name             string
		destination      string
		gateway          string
		networkInterface string
		metric           int
		error            string
	}{
		{
			name:             "invalid IPv6 gateway - missing digits",
			destination:      "fd45::/48",
			gateway:          "fd00:::",
			networkInterface: "n6",
			metric:           0,
			error:            "invalid gateway format: expecting an IPv4 or IPv6 address",
		},
		{
			name:             "invalid IPv6 gateway - too many hex groups",
			destination:      "fd45::/48",
			gateway:          "fd00:1:2:3:4:5:6:7:8",
			networkInterface: "n6",
			metric:           0,
			error:            "invalid gateway format: expecting an IPv4 or IPv6 address",
		},
		{
			name:             "invalid IPv6 gateway - out of range hex",
			destination:      "fd45::/48",
			gateway:          "gggg::1",
			networkInterface: "n6",
			metric:           0,
			error:            "invalid gateway format: expecting an IPv4 or IPv6 address",
		},
		{
			name:             "invalid IPv6 destination - prefix too long",
			destination:      "fd45::/129",
			gateway:          "fd00:6::2",
			networkInterface: "n6",
			metric:           0,
			error:            "invalid destination format: expecting CIDR notation",
		},
		{
			name:             "invalid IPv6 destination - not a valid prefix",
			destination:      "fd45:::1/48",
			gateway:          "fd00:6::2",
			networkInterface: "n6",
			metric:           0,
			error:            "invalid destination format: expecting CIDR notation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createRouteParams := &CreateRouteParams{
				Destination: tt.destination,
				Gateway:     tt.gateway,
				Interface:   tt.networkInterface,
				Metric:      tt.metric,
			}

			statusCode, response, err := createRoute(env.Server.URL, client, token, createRouteParams)
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

func TestCreateTooManyRoutes(t *testing.T) {
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

	for i := 0; i < 12; i++ {
		createRouteParams := &CreateRouteParams{
			Destination: fmt.Sprintf("1.1.%d.0/24", i),
			Gateway:     fmt.Sprintf("1.2.%d.4", i),
			Interface:   Interface,
			Metric:      Metric,
		}

		statusCode, response, err := createRoute(env.Server.URL, client, token, createRouteParams)
		if err != nil {
			t.Fatalf("couldn't create route: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	}

	createRouteParams := &CreateRouteParams{
		Destination: "1.2.3.4/24",
		Gateway:     "1.2.2.1",
		Interface:   Interface,
		Metric:      Metric,
	}

	statusCode, response, err := createRoute(env.Server.URL, client, token, createRouteParams)
	if err != nil {
		t.Fatalf("couldn't create route: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
	}

	if response.Error != "Maximum number of routes reached (12)" {
		t.Fatalf("expected error %q, got %q", "Maximum number of routes reached (12)", response.Error)
	}
}
