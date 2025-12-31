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
	Imsi           = "001010100007487"
	Opc            = "b9f9d006cbe505a0b79f1ad0b3e44d95"
	Key            = "5122250214c33e723a5dd523fc145fc0"
	SequenceNumber = "16f3b3f70fc2"
)

type ListSubscriberResponseResult struct {
	Items      []Subscriber `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type ListSubscriberResponse struct {
	Result ListSubscriberResponseResult `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

type CreateSubscriberSuccessResponse struct {
	Message string `json:"message"`
}

type Subscriber struct {
	Imsi           string `json:"imsi"`
	OPc            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
	PolicyName     string `json:"policyName"`
}

type GetSubscriberResponse struct {
	Result Subscriber `json:"result"`
	Error  string     `json:"error,omitempty"`
}

type CreateSubscriberParams struct {
	Imsi           string `json:"imsi"`
	Key            string `json:"key"`
	Opc            string `json:"opc,omitempty"`
	SequenceNumber string `json:"sequenceNumber"`
	PolicyName     string `json:"policyName"`
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

func listSubscribers(url string, client *http.Client, token string, page int, perPage int) (int, *ListSubscriberResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/subscribers?page=%d&per_page=%d", url, page, perPage), nil)
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

	var subscriberResponse ListSubscriberResponse

	if err := json.NewDecoder(res.Body).Decode(&subscriberResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &subscriberResponse, nil
}

func getSubscriber(url string, client *http.Client, token string, imsi string) (int, *GetSubscriberResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/subscribers/"+imsi, nil)
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

	var subscriberResponse GetSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&subscriberResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &subscriberResponse, nil
}

func createSubscriber(url string, client *http.Client, token string, data *CreateSubscriberParams) (int, *CreateSubscriberResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/subscribers", strings.NewReader(string(body)))
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

	var createResponse CreateSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &createResponse, nil
}

func deleteSubscriber(url string, client *http.Client, token string, imsi string) (int, *DeleteSubscriberResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/subscribers/"+imsi, nil)
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
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Create data network", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:   "whatever",
			MTU:    MTU,
			IPPool: IPPool,
			DNS:    DNS,
		}

		statusCode, response, err := createDataNetwork(ts.URL, client, token, createDataNetworkParams)
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

	t.Run("1. Create policy", func(t *testing.T) {
		createPolicyParams := &CreatePolicyParams{
			Name:            PolicyName,
			BitrateUplink:   "100 Mbps",
			BitrateDownlink: "100 Mbps",
			Var5qi:          9,
			Arp:             1,
			DataNetworkName: "whatever",
		}

		statusCode, response, err := createPolicy(ts.URL, client, token, createPolicyParams)
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
			Key:            Key,
			Opc:            Opc,
			SequenceNumber: SequenceNumber,
			PolicyName:     PolicyName,
		}

		statusCode, response, err := createSubscriber(ts.URL, client, token, createSubscriberParams)
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
		statusCode, response, err := getSubscriber(ts.URL, client, token, Imsi)
		if err != nil {
			t.Fatalf("couldn't get subscriber: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Imsi != Imsi {
			t.Fatalf("expected imsi %s, got %s", Imsi, response.Result.Imsi)
		}

		if response.Result.OPc != Opc {
			t.Fatalf("expected opc %s, got %s", Opc, response.Result.OPc)
		}

		if response.Result.Key != Key {
			t.Fatalf("expected key %s, got %s", Key, response.Result.Key)
		}

		if response.Result.SequenceNumber != SequenceNumber {
			t.Fatalf("expected sequenceNumber %s, got %s", SequenceNumber, response.Result.SequenceNumber)
		}

		if response.Result.PolicyName != PolicyName {
			t.Fatalf("expected policyName %s, got %s", PolicyName, response.Result.PolicyName)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("4. Get subscriber - id not found", func(t *testing.T) {
		statusCode, response, err := getSubscriber(ts.URL, client, token, "001010100007488")
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

		statusCode, response, err := createSubscriber(ts.URL, client, token, createSubscriberParams)
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
		statusCode, response, err := deleteSubscriber(ts.URL, client, token, Imsi)
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
		statusCode, response, err := deleteSubscriber(ts.URL, client, token, "001010100007488")
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

	t.Run("8. Create subscriber (with opc)", func(t *testing.T) {
		createSubscriberParams := &CreateSubscriberParams{
			Imsi:           Imsi,
			Key:            Key,
			Opc:            Opc,
			SequenceNumber: SequenceNumber,
			PolicyName:     PolicyName,
		}

		statusCode, response, err := createSubscriber(ts.URL, client, token, createSubscriberParams)
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

	t.Run("9. Get subscriber - with opc", func(t *testing.T) {
		statusCode, response, err := getSubscriber(ts.URL, client, token, Imsi)
		if err != nil {
			t.Fatalf("couldn't get subscriber: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Imsi != Imsi {
			t.Fatalf("expected imsi %s, got %s", Imsi, response.Result.Imsi)
		}

		if response.Result.OPc != Opc {
			t.Fatalf("expected opc %s, got %s", Opc, response.Result.OPc)
		}

		if response.Result.Key != Key {
			t.Fatalf("expected key %s, got %s", Key, response.Result.Key)
		}

		if response.Result.SequenceNumber != SequenceNumber {
			t.Fatalf("expected sequenceNumber %s, got %s", SequenceNumber, response.Result.SequenceNumber)
		}

		if response.Result.PolicyName != PolicyName {
			t.Fatalf("expected policyName %s, got %s", PolicyName, response.Result.PolicyName)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})
}

func TestCreateSubscriberInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	tests := []struct {
		imsi           string
		key            string
		sequenceNumber string
		error          string
	}{
		{
			imsi:           "12345",
			key:            Key,
			sequenceNumber: SequenceNumber,
			error:          "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.",
		},
		{
			imsi:           "00101010000748812",
			key:            Key,
			sequenceNumber: SequenceNumber,
			error:          "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.",
		},
		{
			imsi:           "002010100007488",
			key:            Key,
			sequenceNumber: SequenceNumber,
			error:          "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.",
		},
		{
			imsi:           "00101",
			key:            Key,
			sequenceNumber: SequenceNumber,
			error:          "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.",
		},
		{
			imsi:           Imsi,
			key:            "12345",
			sequenceNumber: SequenceNumber,
			error:          "Invalid key format. Must be a 32-character hexadecimal string.",
		},
		{
			imsi:           Imsi,
			key:            "12345678901234567890123456789012345678901234567890123456789012345",
			sequenceNumber: SequenceNumber,
			error:          "Invalid key format. Must be a 32-character hexadecimal string.",
		},
		{
			imsi:           Imsi,
			key:            Key,
			sequenceNumber: "12345",
			error:          "Invalid sequenceNumber. Must be a 6-byte hexadecimal string.",
		},
		{
			imsi:           Imsi,
			key:            Key,
			sequenceNumber: "1234567890123",
			error:          "Invalid sequenceNumber. Must be a 6-byte hexadecimal string.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.imsi, func(t *testing.T) {
			createSubscriberParams := &CreateSubscriberParams{
				Imsi:           tt.imsi,
				Key:            tt.key,
				SequenceNumber: tt.sequenceNumber,
				PolicyName:     PolicyName,
			}

			statusCode, response, err := createSubscriber(ts.URL, client, token, createSubscriberParams)
			if err != nil {
				t.Fatalf("couldn't create subscriber: %s", err)
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

func TestCreateSubscriberValidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	tests := []struct {
		mcc  string
		mnc  string
		imsi string
	}{
		{
			mcc:  "001",
			mnc:  "01",
			imsi: "001019756139935",
		},
		{
			mcc:  "001",
			mnc:  "001",
			imsi: "001001975613993",
		},
	}
	for _, tt := range tests {
		t.Run(tt.imsi, func(t *testing.T) {
			updateOperatorIDParams := &UpdateOperatorIDParams{
				Mcc: tt.mcc,
				Mnc: tt.mnc,
			}

			statusCode, _, err := updateOperatorID(ts.URL, client, token, updateOperatorIDParams)
			if err != nil {
				t.Fatalf("couldn't update operator ID: %s", err)
			}

			if statusCode != http.StatusCreated {
				t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
			}

			createSubscriberParams := &CreateSubscriberParams{
				Imsi:           tt.imsi,
				Key:            Key,
				SequenceNumber: SequenceNumber,
				PolicyName:     "default",
			}

			statusCode, _, err = createSubscriber(ts.URL, client, token, createSubscriberParams)
			if err != nil {
				t.Fatalf("couldn't create subscriber: %s", err)
			}

			if statusCode != http.StatusCreated {
				t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
			}

			statusCode, _, err = deleteSubscriber(ts.URL, client, token, tt.imsi)
			if err != nil {
				t.Fatalf("couldn't delete subscriber: %s", err)
			}

			if statusCode != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
			}
		})
	}
}

func TestCreateTooManySubscribers(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	createDataNetworkParams := &CreateDataNetworkParams{
		Name:   "whatever",
		MTU:    MTU,
		IPPool: IPPool,
		DNS:    DNS,
	}

	statusCode, response, err := createDataNetwork(ts.URL, client, token, createDataNetworkParams)
	if err != nil {
		t.Fatalf("couldn't create data network: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	if response.Error != "" {
		t.Fatalf("unexpected error :%q", response.Error)
	}

	createPolicyParams := &CreatePolicyParams{
		Name:            PolicyName,
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "100 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkName: "whatever",
	}

	statusCode, createPolicyResponse, err := createPolicy(ts.URL, client, token, createPolicyParams)
	if err != nil {
		t.Fatalf("couldn't create policy: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	if createPolicyResponse.Error != "" {
		t.Fatalf("unexpected error :%q", createPolicyResponse.Error)
	}

	baseImsi := Imsi[:len(Imsi)-4]

	for i := 0; i < 1000; i++ {
		createSubscriberParams := &CreateSubscriberParams{
			Imsi:           fmt.Sprintf("%s%04d", baseImsi, i),
			Key:            Key,
			Opc:            Opc,
			SequenceNumber: SequenceNumber,
			PolicyName:     PolicyName,
		}
		t.Log("Creating subscriber:", createSubscriberParams.Imsi)

		statusCode, response, err := createSubscriber(ts.URL, client, token, createSubscriberParams)
		if err != nil {
			t.Fatalf("couldn't create subscriber: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	}

	createSubscriberParams := &CreateSubscriberParams{
		Imsi:           fmt.Sprintf("%s%04d", baseImsi, 1000),
		Key:            Key,
		Opc:            Opc,
		SequenceNumber: SequenceNumber,
		PolicyName:     PolicyName,
	}

	statusCode, createSubscriberResponse, err := createSubscriber(ts.URL, client, token, createSubscriberParams)
	if err != nil {
		t.Fatalf("couldn't create subscriber: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
	}

	if createSubscriberResponse.Error != "Maximum number of subscribers reached (1000)" {
		t.Fatalf("expected error %q, got %q", "Maximum number of subscribers reached (1000)", createSubscriberResponse.Error)
	}
}
