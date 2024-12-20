package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const (
	Imsi = "001010100007487"
)

type CreateSubscriberSuccessResponse struct {
	Message string `json:"message"`
}

type GetSubscriberResponseResult struct {
	Imsi           string `json:"imsi"`
	OPc            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
	ProfileName    string `json:"profileName"`
}

type GetSubscriberResponse struct {
	Result GetSubscriberResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

type CreateSubscriberParams struct {
	Imsi           string `json:"imsi"`
	OPc            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
	ProfileName    string `json:"profileName"`
}

type CreateSubscriberResponseResult struct {
	Message string `json:"message"`
}

type CreateSubscriberResponse struct {
	Result CreateSubscriberSuccessResponse `json:"result"`
	Error  string                          `json:"error,omitempty"`
}

type DeleteSubscriberResponseResult struct {
	Message string `json:"message"`
}

type DeleteSubscriberResponse struct {
	Result DeleteSubscriberResponseResult `json:"result"`
	Error  string                         `json:"error,omitempty"`
}

func getSubscriber(url string, client *http.Client, imsi string) (int, *GetSubscriberResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/subscribers/"+imsi, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var subscriberResponse GetSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&subscriberResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &subscriberResponse, nil
}

func createSubscriber(url string, client *http.Client, data *CreateSubscriberParams) (int, *CreateSubscriberResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/subscribers", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var createResponse CreateSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteSubscriber(url string, client *http.Client, imsi string) (int, *DeleteSubscriberResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/subscribers/"+imsi, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var deleteResponse DeleteSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteResponse, nil
}

// This is an end-to-end test for the subscribers handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestSubscribersApiEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("1. Create profile", func(t *testing.T) {
		createProfileParams := &CreateProfileParams{
			Name:            ProfileName,
			UeIpPool:        "0.0.0.0/24",
			DnsPrimary:      "8.8.8.8",
			DnsSecondary:    "1.1.1.1",
			Mtu:             1500,
			BitrateUplink:   "100 Mbps",
			BitrateDownlink: "100 Mbps",
			Var5qi:          9,
			PriorityLevel:   1,
		}
		statusCode, response, err := createProfile(ts.URL, client, createProfileParams)
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

	t.Run("2. Create subscriber", func(t *testing.T) {
		createSubscriberParams := &CreateSubscriberParams{
			Imsi:           Imsi,
			OPc:            "123456",
			Key:            "123",
			SequenceNumber: "123456",
			ProfileName:    ProfileName,
		}
		statusCode, response, err := createSubscriber(ts.URL, client, createSubscriberParams)
		if err != nil {
			t.Fatalf("couldn't create subscriber: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Subscriber created successfully" {
			t.Fatalf("expected message 'Subscriber created successfully', got %q", response.Result.Message)
		}
	})

	t.Run("3. Get subscriber", func(t *testing.T) {
		statusCode, response, err := getSubscriber(ts.URL, client, Imsi)
		if err != nil {
			t.Fatalf("couldn't get subscriber: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Imsi != Imsi {
			t.Fatalf("expected imsi %s, got %s", Imsi, response.Result.Imsi)
		}
		if response.Result.OPc != "123456" {
			t.Fatalf("expected opc 123456, got %s", response.Result.OPc)
		}
		if response.Result.Key != "123" {
			t.Fatalf("expected key 123, got %s", response.Result.Key)
		}
		if response.Result.SequenceNumber != "123456" {
			t.Fatalf("expected sequenceNumber 123456, got %s", response.Result.SequenceNumber)
		}
		if response.Result.ProfileName != ProfileName {
			t.Fatalf("expected profileName %s, got %s", ProfileName, response.Result.ProfileName)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("4. Get subscriber - id not found", func(t *testing.T) {
		statusCode, response, err := getSubscriber(ts.URL, client, "001010100007488")
		if err != nil {
			t.Fatalf("couldn't get subscriber: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "Subscriber not found" {
			t.Fatalf("expected error %q, got %q", "Subscriber not found", response.Error)
		}
	})

	t.Run("5. Create subscriber - no Imsi", func(t *testing.T) {
		createSubscriberParams := &CreateSubscriberParams{}
		statusCode, response, err := createSubscriber(ts.URL, client, createSubscriberParams)
		if err != nil {
			t.Fatalf("couldn't create subscriber: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "Missing imsi parameter" {
			t.Fatalf("expected error %q, got %q", "Missing imsi parameter", response.Error)
		}
	})

	t.Run("6. Delete subscriber - success", func(t *testing.T) {
		statusCode, response, err := deleteSubscriber(ts.URL, client, Imsi)
		if err != nil {
			t.Fatalf("couldn't delete subscriber: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Subscriber deleted successfully" {
			t.Fatalf("expected message 'Subscriber deleted successfully', got %q", response.Result.Message)
		}
	})

	t.Run("7. Delete subscriber - no user", func(t *testing.T) {
		statusCode, response, err := deleteSubscriber(ts.URL, client, "001010100007488")
		if err != nil {
			t.Fatalf("couldn't delete subscriber: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "Subscriber not found" {
			t.Fatalf("expected error %q, got %q", "Subscriber not found", response.Error)
		}
	})
}

func TestCreateSubscribersBadImsis(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	tests := []struct {
		imsi string
	}{
		{imsi: "12345"},
		{imsi: "00101010000748812"},
		{imsi: "002010100007488"},
		{imsi: "00101"},
	}

	for _, tt := range tests {
		t.Run(tt.imsi, func(t *testing.T) {
			createSubscriberParams := &CreateSubscriberParams{
				Imsi:           tt.imsi,
				OPc:            "123456",
				Key:            "123",
				SequenceNumber: "123456",
				ProfileName:    ProfileName,
			}
			statusCode, response, err := createSubscriber(ts.URL, client, createSubscriberParams)
			if err != nil {
				t.Fatalf("couldn't create subscriber: %s", err)
			}
			if statusCode != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
			}
			if response.Error != "Invalid imsi" {
				t.Fatalf("expected error %q, got %q", "Invalid imsi", response.Error)
			}
		})
	}
}
