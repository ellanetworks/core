package server_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const UeId = "imsi-001010100007487"

type CreateSubscriberSuccessResponse struct {
	Message string `json:"message"`
}

type GetSubscriberResponse struct {
	UeId           string `json:"ueId"`
	PlmnID         string `json:"plmnID"`
	OPc            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
}

type CreateSubscriberParams struct {
	UeId           string `json:"ueId"`
	PlmnID         string `json:"plmnID"`
	OPc            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
}

type CreateSubscriberResponseResult struct {
	ID int `json:"id"`
}

type CreateSubscriberResponse struct {
	Result CreateSubscriberSuccessResponse `json:"result"`
	Error  string                          `json:"error,omitempty"`
}

type DeleteSubscriberResponseResult struct {
	ID int `json:"id"`
}

func getSubscriber(url string, client *http.Client, ueId string) (int, *GetSubscriberResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/subscribers/"+ueId, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
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
	req, err := http.NewRequest("POST", url+"/api/v1/subscribers", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse CreateSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteSubscriber(url string, client *http.Client, ueId string) (int, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/subscribers/"+ueId, nil)
	if err != nil {
		return 0, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return res.StatusCode, nil
}

// This is an end-to-end test for the subscribers handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestSubscribersEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("1. Create subscriber", func(t *testing.T) {
		createSubscriberParams := &CreateSubscriberParams{
			UeId:           UeId,
			PlmnID:         "123456",
			OPc:            "123456",
			Key:            "123",
			SequenceNumber: "123456",
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
	})

	t.Run("2. Get subscriber", func(t *testing.T) {
		statusCode, response, err := getSubscriber(ts.URL, client, UeId)
		if err != nil {
			t.Fatalf("couldn't get subscriber: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.UeId != UeId {
			t.Fatalf("expected ueId %s, got %s", UeId, response.UeId)
		}
		if response.PlmnID != "123456" {
			t.Fatalf("expected plmnID 123456, got %s", response.PlmnID)
		}
		if response.OPc != "123456" {
			t.Fatalf("expected opc 123456, got %s", response.OPc)
		}
		if response.Key != "123" {
			t.Fatalf("expected key 123, got %s", response.Key)
		}
		if response.SequenceNumber != "123456" {
			t.Fatalf("expected sequenceNumber 123456, got %s", response.SequenceNumber)
		}
	})

	t.Run("3. Get subscriber - id not found", func(t *testing.T) {
		statusCode, _, err := getSubscriber(ts.URL, client, "imsi-001010100007488")
		if err != nil {
			t.Fatalf("couldn't get subscriber: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})

	t.Run("4. Create subscriber - no ueId", func(t *testing.T) {
		createSubscriberParams := &CreateSubscriberParams{
			PlmnID: "1234",
		}
		statusCode, _, err := createSubscriber(ts.URL, client, createSubscriberParams)
		if err != nil {
			t.Fatalf("couldn't create subscriber: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("5. Delete subscriber - success", func(t *testing.T) {
		statusCode, err := deleteSubscriber(ts.URL, client, UeId)
		if err != nil {
			t.Fatalf("couldn't delete subscriber: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
	})

	t.Run("6. Delete subscriber - no user", func(t *testing.T) {
		statusCode, err := deleteSubscriber(ts.URL, client, "imsi-001010100007488")
		if err != nil {
			t.Fatalf("couldn't delete subscriber: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})
}
