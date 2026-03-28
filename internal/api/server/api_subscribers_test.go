package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/smf"
)

const (
	Imsi           = "001010100007487"
	Opc            = "b9f9d006cbe505a0b79f1ad0b3e44d95"
	Key            = "5122250214c33e723a5dd523fc145fc0"
	SequenceNumber = "16f3b3f70fc2"
)

type ListSubscriberResponseResult struct {
	Items      []ListSubscriber `json:"items"`
	Page       int              `json:"page"`
	PerPage    int              `json:"per_page"`
	TotalCount int              `json:"total_count"`
}

type ListSubscriberResponse struct {
	Result ListSubscriberResponseResult `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

type CreateSubscriberSuccessResponse struct {
	Message string `json:"message"`
}

// ListSubscriberStatus matches the lightweight status in list responses.
type ListSubscriberStatus struct {
	Registered bool   `json:"registered"`
	IPAddress  string `json:"ipAddress"`
	LastSeenAt string `json:"lastSeenAt,omitempty"`
}

// ListSubscriber matches the summary representation in list responses.
type ListSubscriber struct {
	Imsi       string               `json:"imsi"`
	PolicyName string               `json:"policyName"`
	Radio      string               `json:"radio,omitempty"`
	Status     ListSubscriberStatus `json:"status"`
}

// SubscriberDetailStatus matches the rich status in get-single responses.
type SubscriberDetailStatus struct {
	Registered         bool   `json:"registered"`
	IPAddress          string `json:"ipAddress"`
	Imei               string `json:"imei"`
	CipheringAlgorithm string `json:"cipheringAlgorithm"`
	IntegrityAlgorithm string `json:"integrityAlgorithm"`
	LastSeenAt         string `json:"lastSeenAt,omitempty"`
	LastSeenRadio      string `json:"lastSeenRadio,omitempty"`
}

type SessionInfo struct {
	Status    string `json:"status"`
	IPAddress string `json:"ipAddress,omitempty"`
}

// SubscriberDetail matches the full representation in get-single responses.
type SubscriberDetail struct {
	Imsi        string                 `json:"imsi"`
	PolicyName  string                 `json:"policyName"`
	Status      SubscriberDetailStatus `json:"status"`
	PDUSessions []SessionInfo          `json:"pdu_sessions"`
}

type GetSubscriberResponse struct {
	Result SubscriberDetail `json:"result"`
	Error  string           `json:"error,omitempty"`
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

type UpdateSubscriberParams struct {
	Imsi       string `json:"imsi"`
	PolicyName string `json:"policyName"`
}

type UpdateSubscriberResponse struct {
	Result UpdateSubscriberSuccessResponse `json:"result"`
	Error  string                          `json:"error,omitempty"`
}

type UpdateSubscriberSuccessResponse struct {
	Message string `json:"message"`
}

// SubscriberCredentials matches the credentials endpoint response.
type SubscriberCredentials struct {
	Key            string `json:"key"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
}

type GetSubscriberCredentialsResponse struct {
	Result SubscriberCredentials `json:"result"`
	Error  string                `json:"error,omitempty"`
}

func getSubscriberCredentials(url string, client *http.Client, token string, imsi string) (int, *GetSubscriberCredentialsResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/subscribers/"+imsi+"/credentials", nil)
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

	var credsResponse GetSubscriberCredentialsResponse
	if err := json.NewDecoder(res.Body).Decode(&credsResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &credsResponse, nil
}

func updateSubscriber(url string, client *http.Client, token string, imsi string, data *UpdateSubscriberParams) (int, *UpdateSubscriberResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/subscribers/"+imsi, strings.NewReader(string(body)))
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

	var updateResponse UpdateSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &updateResponse, nil
}

// mockSessionForSubscriber creates a mock PDU session for a subscriber in the AMF context.
func mockSessionForSubscriber(amfInstance *amf.AMF, testSmfInstance *smf.SMF, imsi string, dnn string) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("failed to create SUPI from IMSI: %w", err)
	}

	ue, found := amfInstance.FindAMFUEBySupi(supi)
	if !found {
		ue = amf.NewAmfUe()
		ue.Supi = supi

		if err := amfInstance.AddAmfUeToUePool(ue); err != nil {
			return fmt.Errorf("failed to add UE to AMF pool: %w", err)
		}
	}

	pduSessionID := uint8(1)
	testSmfInstance.NewSession(supi, pduSessionID, dnn, nil)

	sessionRef := smf.CanonicalName(supi, pduSessionID)

	err = ue.CreateSmContext(pduSessionID, sessionRef, nil)
	if err != nil {
		return fmt.Errorf("failed to create SmContext: %w", err)
	}

	return nil
}

// This is an end-to-end test for the subscribers handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestSubscribersApiEndToEnd(t *testing.T) {
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

	t.Run("1. Create data network", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:   "whatever",
			MTU:    MTU,
			IPPool: IPPool,
			DNS:    DNS,
		}

		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, createDataNetworkParams)
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

		statusCode, response, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
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

		statusCode, response, err := createSubscriber(env.Server.URL, client, token, createSubscriberParams)
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
		statusCode, response, err := getSubscriber(env.Server.URL, client, token, Imsi)
		if err != nil {
			t.Fatalf("couldn't get subscriber: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Imsi != Imsi {
			t.Fatalf("expected imsi %s, got %s", Imsi, response.Result.Imsi)
		}

		if response.Result.PolicyName != PolicyName {
			t.Fatalf("expected policyName %s, got %s", PolicyName, response.Result.PolicyName)
		}

		if response.Result.Status.Registered != false {
			t.Fatalf("expected registered false, got %v", response.Result.Status.Registered)
		}

		if response.Result.Status.Imei != "" {
			t.Fatalf("expected empty imei, got %s", response.Result.Status.Imei)
		}

		if response.Result.Status.CipheringAlgorithm != "" {
			t.Fatalf("expected empty cipheringAlgorithm, got %s", response.Result.Status.CipheringAlgorithm)
		}

		if response.Result.Status.IntegrityAlgorithm != "" {
			t.Fatalf("expected empty integrityAlgorithm, got %s", response.Result.Status.IntegrityAlgorithm)
		}

		if response.Result.PDUSessions == nil {
			t.Fatalf("expected sessions field to be present, got nil")
		}

		if len(response.Result.PDUSessions) != 0 {
			t.Fatalf("expected 0 sessions, got %d", len(response.Result.PDUSessions))
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("3b. Get subscriber credentials", func(t *testing.T) {
		statusCode, response, err := getSubscriberCredentials(env.Server.URL, client, token, Imsi)
		if err != nil {
			t.Fatalf("couldn't get subscriber credentials: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Key != Key {
			t.Fatalf("expected key %s, got %s", Key, response.Result.Key)
		}

		if response.Result.Opc != Opc {
			t.Fatalf("expected opc %s, got %s", Opc, response.Result.Opc)
		}

		if response.Result.SequenceNumber != SequenceNumber {
			t.Fatalf("expected sequenceNumber %s, got %s", SequenceNumber, response.Result.SequenceNumber)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("3c. Get subscriber credentials - not found", func(t *testing.T) {
		statusCode, response, err := getSubscriberCredentials(env.Server.URL, client, token, "001010100007488")
		if err != nil {
			t.Fatalf("couldn't get subscriber credentials: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "Subscriber not found" {
			t.Fatalf("expected error %q, got %q", "Subscriber not found", response.Error)
		}
	})

	t.Run("4. Get subscriber - id not found", func(t *testing.T) {
		statusCode, response, err := getSubscriber(env.Server.URL, client, token, "001010100007488")
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

		statusCode, response, err := createSubscriber(env.Server.URL, client, token, createSubscriberParams)
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

	t.Run("6. Create second policy for update tests", func(t *testing.T) {
		createPolicyParams := &CreatePolicyParams{
			Name:            "policy2",
			BitrateUplink:   "50 Mbps",
			BitrateDownlink: "50 Mbps",
			Var5qi:          8,
			Arp:             2,
			DataNetworkName: "whatever",
		}

		statusCode, response, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
		if err != nil {
			t.Fatalf("couldn't create policy: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("7. Update subscriber - success", func(t *testing.T) {
		updateParams := &UpdateSubscriberParams{
			Imsi:       Imsi,
			PolicyName: "policy2",
		}

		statusCode, response, err := updateSubscriber(env.Server.URL, client, token, Imsi, updateParams)
		if err != nil {
			t.Fatalf("couldn't update subscriber: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "Subscriber updated successfully" {
			t.Fatalf("expected message 'Subscriber updated successfully', got %q", response.Result.Message)
		}

		// Verify the policy was actually updated
		statusCode, getResponse, err := getSubscriber(env.Server.URL, client, token, Imsi)
		if err != nil {
			t.Fatalf("couldn't get subscriber: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if getResponse.Result.PolicyName != "policy2" {
			t.Fatalf("expected policyName 'policy2', got %s", getResponse.Result.PolicyName)
		}
	})

	t.Run("8. Update subscriber - missing imsi in path", func(t *testing.T) {
		updateParams := &UpdateSubscriberParams{
			Imsi:       Imsi,
			PolicyName: PolicyName,
		}

		body, err := json.Marshal(updateParams)
		if err != nil {
			t.Fatalf("couldn't marshal params: %s", err)
		}

		req, err := http.NewRequestWithContext(context.Background(), "PUT", env.Server.URL+"/api/v1/subscribers/", strings.NewReader(string(body)))
		if err != nil {
			t.Fatalf("couldn't create request: %s", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)

		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("couldn't do request: %s", err)
		}

		defer func() {
			_ = res.Body.Close()
		}()

		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.StatusCode)
		}
	})

	t.Run("9. Update subscriber - invalid request body", func(t *testing.T) {
		body := strings.NewReader(`{"invalid": json}`)

		req, err := http.NewRequestWithContext(context.Background(), "PUT", env.Server.URL+"/api/v1/subscribers/"+Imsi, body)
		if err != nil {
			t.Fatalf("couldn't create request: %s", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)

		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("couldn't do request: %s", err)
		}

		defer func() {
			_ = res.Body.Close()
		}()

		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.StatusCode)
		}
	})

	t.Run("5f. Update subscriber - missing policy name", func(t *testing.T) {
		updateParams := &UpdateSubscriberParams{
			Imsi:       Imsi,
			PolicyName: "",
		}

		statusCode, response, err := updateSubscriber(env.Server.URL, client, token, Imsi, updateParams)
		if err != nil {
			t.Fatalf("couldn't update subscriber: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "Missing policyName parameter" {
			t.Fatalf("expected error 'Missing policyName parameter', got %q", response.Error)
		}
	})

	t.Run("11. Update subscriber - not found", func(t *testing.T) {
		updateParams := &UpdateSubscriberParams{
			Imsi:       "invalid-imsi",
			PolicyName: PolicyName,
		}

		statusCode, response, err := updateSubscriber(env.Server.URL, client, token, "invalid-imsi", updateParams)
		if err != nil {
			t.Fatalf("couldn't update subscriber: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "Subscriber not found" {
			t.Fatalf("expected error 'Subscriber not found', got %q", response.Error)
		}
	})

	t.Run("12. Update subscriber - policy not found", func(t *testing.T) {
		updateParams := &UpdateSubscriberParams{
			Imsi:       Imsi,
			PolicyName: "nonexistent-policy",
		}

		statusCode, response, err := updateSubscriber(env.Server.URL, client, token, Imsi, updateParams)
		if err != nil {
			t.Fatalf("couldn't update subscriber: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "Policy not found" {
			t.Fatalf("expected error 'Policy not found', got %q", response.Error)
		}
	})

	t.Run("13. Update subscriber - subscriber not found", func(t *testing.T) {
		updateParams := &UpdateSubscriberParams{
			Imsi:       "001010100007488",
			PolicyName: PolicyName,
		}

		statusCode, response, err := updateSubscriber(env.Server.URL, client, token, "001010100007488", updateParams)
		if err != nil {
			t.Fatalf("couldn't update subscriber: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "Subscriber not found" {
			t.Fatalf("expected error 'Subscriber not found', got %q", response.Error)
		}
	})

	t.Run("14. Delete subscriber - success", func(t *testing.T) {
		statusCode, response, err := deleteSubscriber(env.Server.URL, client, token, Imsi)
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

	t.Run("15. Delete subscriber - no user", func(t *testing.T) {
		statusCode, response, err := deleteSubscriber(env.Server.URL, client, token, "001010100007488")
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

	t.Run("16. Create subscriber (with opc)", func(t *testing.T) {
		createSubscriberParams := &CreateSubscriberParams{
			Imsi:           Imsi,
			Key:            Key,
			Opc:            Opc,
			SequenceNumber: SequenceNumber,
			PolicyName:     PolicyName,
		}

		statusCode, response, err := createSubscriber(env.Server.URL, client, token, createSubscriberParams)
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

	t.Run("17. Get subscriber - with opc", func(t *testing.T) {
		statusCode, response, err := getSubscriber(env.Server.URL, client, token, Imsi)
		if err != nil {
			t.Fatalf("couldn't get subscriber: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Imsi != Imsi {
			t.Fatalf("expected imsi %s, got %s", Imsi, response.Result.Imsi)
		}

		if response.Result.PolicyName != PolicyName {
			t.Fatalf("expected policyName %s, got %s", PolicyName, response.Result.PolicyName)
		}

		if response.Result.Status.Registered != false {
			t.Fatalf("expected registered false, got %v", response.Result.Status.Registered)
		}

		if response.Result.Status.CipheringAlgorithm != "" {
			t.Fatalf("expected empty cipheringAlgorithm, got %s", response.Result.Status.CipheringAlgorithm)
		}

		if response.Result.Status.IntegrityAlgorithm != "" {
			t.Fatalf("expected empty integrityAlgorithm, got %s", response.Result.Status.IntegrityAlgorithm)
		}

		if response.Result.PDUSessions == nil {
			t.Fatalf("expected sessions field to be present, got nil")
		}

		if len(response.Result.PDUSessions) != 0 {
			t.Fatalf("expected 0 sessions, got %d", len(response.Result.PDUSessions))
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("18. Get subscriber - with session", func(t *testing.T) {
		if err := mockSessionForSubscriber(env.AMF, env.SMF, Imsi, "internet"); err != nil {
			t.Fatalf("couldn't mock session: %s", err)
		}

		statusCode, response, err := getSubscriber(env.Server.URL, client, token, Imsi)
		if err != nil {
			t.Fatalf("couldn't get subscriber: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.PDUSessions == nil {
			t.Fatalf("expected sessions field to be present, got nil")
		}

		if len(response.Result.PDUSessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(response.Result.PDUSessions))
		}

		session := response.Result.PDUSessions[0]

		if session.Status != "active" {
			t.Fatalf("expected session status 'active', got %q", session.Status)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})
}

func TestCreateSubscriberInvalidInput(t *testing.T) {
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

			statusCode, response, err := createSubscriber(env.Server.URL, client, token, createSubscriberParams)
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

			statusCode, _, err := updateOperatorID(env.Server.URL, client, token, updateOperatorIDParams)
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

			statusCode, _, err = createSubscriber(env.Server.URL, client, token, createSubscriberParams)
			if err != nil {
				t.Fatalf("couldn't create subscriber: %s", err)
			}

			if statusCode != http.StatusCreated {
				t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
			}

			statusCode, _, err = deleteSubscriber(env.Server.URL, client, token, tt.imsi)
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

	createDataNetworkParams := &CreateDataNetworkParams{
		Name:   "whatever",
		MTU:    MTU,
		IPPool: IPPool,
		DNS:    DNS,
	}

	statusCode, response, err := createDataNetwork(env.Server.URL, client, token, createDataNetworkParams)
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

	statusCode, createPolicyResponse, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
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

		statusCode, response, err := createSubscriber(env.Server.URL, client, token, createSubscriberParams)
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

	statusCode, createSubscriberResponse, err := createSubscriber(env.Server.URL, client, token, createSubscriberParams)
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
