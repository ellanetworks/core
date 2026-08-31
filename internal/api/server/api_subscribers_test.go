// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	url2 "net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/api/server"
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
	Registered       bool     `json:"registered"`
	RadioAccessTypes []string `json:"radio_access_types,omitempty"`
	NumSessions      int      `json:"num_sessions"`
	LastSeenAt       string   `json:"last_seen_at,omitempty"`
}

// ListSubscriber matches the summary representation in list responses.
type ListSubscriber struct {
	Imsi        string               `json:"imsi"`
	ProfileName string               `json:"profile_name"`
	Description string               `json:"description,omitempty"`
	Radio       string               `json:"radio,omitempty"`
	Status      ListSubscriberStatus `json:"status"`
}

// SubscriberDetailStatus matches the rich status in get-single responses.
type SubscriberDetailStatus struct {
	Registered         bool     `json:"registered"`
	RadioAccessTypes   []string `json:"radio_access_types,omitempty"`
	Imei               string   `json:"imei"`
	CipheringAlgorithm string   `json:"ciphering_algorithm"`
	IntegrityAlgorithm string   `json:"integrity_algorithm"`
	LastSeenAt         string   `json:"last_seen_at,omitempty"`
	LastSeenRadio      string   `json:"last_seen_radio,omitempty"`
}

type Slice struct {
	SST int32  `json:"sst"`
	SD  string `json:"sd,omitempty"`
}

type Session struct {
	RadioAccessType string `json:"radio_access_type"`
	ID              uint8  `json:"id"`
	Status          string `json:"status"`
	IPType          string `json:"ip_type,omitempty"`
	IPv4Address     string `json:"ipv4_address,omitempty"`
	IPv6Prefix      string `json:"ipv6_prefix,omitempty"`
	DataNetwork     string `json:"data_network,omitempty"`
	Slice           *Slice `json:"slice,omitempty"`
	AMBRUplink      string `json:"ambr_uplink,omitempty"`
	AMBRDownlink    string `json:"ambr_downlink,omitempty"`
}

// SubscriberDetail matches the full representation in get-single responses.
type SubscriberDetail struct {
	Imsi        string                 `json:"imsi"`
	ProfileName string                 `json:"profile_name"`
	Description string                 `json:"description,omitempty"`
	Status      SubscriberDetailStatus `json:"status"`
	Sessions    []Session              `json:"sessions"`
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
	ProfileName    string `json:"profile_name"`
	Description    string `json:"description,omitempty"`
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
	return apiDo[ListSubscriberResponse](client, "GET", fmt.Sprintf("%s/api/v1/subscribers?page=%d&per_page=%d", url, page, perPage), token, nil)
}

func getSubscriber(url string, client *http.Client, token string, imsi string) (int, *GetSubscriberResponse, error) {
	return apiDo[GetSubscriberResponse](client, "GET", url+"/api/v1/subscribers/"+imsi, token, nil)
}

func createSubscriber(url string, client *http.Client, token string, data *CreateSubscriberParams) (int, *CreateSubscriberResponse, error) {
	return apiDo[CreateSubscriberResponse](client, "POST", url+"/api/v1/subscribers", token, data)
}

func deleteSubscriber(url string, client *http.Client, token string, imsi string) (int, *DeleteSubscriberResponse, error) {
	return apiDo[DeleteSubscriberResponse](client, "DELETE", url+"/api/v1/subscribers/"+imsi, token, nil)
}

type UpdateSubscriberParams struct {
	Imsi        string `json:"imsi"`
	ProfileName string `json:"profile_name"`
	Description string `json:"description,omitempty"`
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
	return apiDo[GetSubscriberCredentialsResponse](client, "GET", url+"/api/v1/subscribers/"+imsi+"/credentials", token, nil)
}

func updateSubscriber(url string, client *http.Client, token string, imsi string, data *UpdateSubscriberParams) (int, *UpdateSubscriberResponse, error) {
	return apiDo[UpdateSubscriberResponse](client, "PUT", url+"/api/v1/subscribers/"+imsi, token, data)
}

// mockSessionForSubscriber creates a mock PDU session for a subscriber in the AMF context.
func mockSessionForSubscriber(amfInstance *amf.AMF, testSmfInstance *smf.SMF, imsi string, dnn string) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("failed to create SUPI from IMSI: %w", err)
	}

	ue, found := amfInstance.LookupUeBySupi(supi)
	if !found {
		ue = amf.NewUeContext()
		ue.SetSupiForTest(supi)

		if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
			return fmt.Errorf("failed to add UE to AMF pool: %w", err)
		}
	}

	// A UE holding a PDU session is Registered (registration precedes session
	// establishment); the status API surfaces sessions only for Registered UEs.
	ue.ForceStateForTest(amf.Registered)

	pduSessionID := uint8(1)
	sc, _ := testSmfInstance.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: pduSessionID}, dnn, nil)

	err = ue.CreateSmContext(pduSessionID, sc.Ref, nil, "internet")
	if err != nil {
		return fmt.Errorf("failed to create SmContext: %w", err)
	}

	return nil
}

// This is an end-to-end test for the subscribers handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestSubscribersApiEndToEnd(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	t.Run("1. Create data network", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:     "whatever",
			MTU:      MTU,
			IPv4Pool: IPv4Pool,
			DNS:      DNS,
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

	t.Run("1. Create profile and policy", func(t *testing.T) {
		createProfileParams := &CreateProfileParams{
			Name:           TestProfileName,
			UeAmbrUplink:   "200 Mbps",
			UeAmbrDownlink: "200 Mbps",
		}

		statusCode, createProfileResponse, err := createProfile(env.Server.URL, client, token, createProfileParams)
		if err != nil {
			t.Fatalf("couldn't create profile: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if createProfileResponse.Error != "" {
			t.Fatalf("unexpected error :%q", createProfileResponse.Error)
		}

		createPolicyParams := &CreatePolicyParams{
			Name:                PolicyName,
			ProfileName:         TestProfileName,
			SliceName:           DefaultSliceName,
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "100 Mbps",
			Var5qi:              9,
			Arp:                 1,
			DataNetworkName:     "whatever",
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

	t.Run("2. Create subscriber", func(t *testing.T) {
		createSubscriberParams := &CreateSubscriberParams{
			Imsi:           Imsi,
			Key:            Key,
			Opc:            Opc,
			SequenceNumber: SequenceNumber,
			ProfileName:    TestProfileName,
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

		if response.Result.ProfileName != TestProfileName {
			t.Fatalf("expected profileName %s, got %s", TestProfileName, response.Result.ProfileName)
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

		if response.Result.Sessions == nil {
			t.Fatalf("expected sessions field to be present, got nil")
		}

		if len(response.Result.Sessions) != 0 {
			t.Fatalf("expected 0 sessions, got %d", len(response.Result.Sessions))
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

	t.Run("6. Create second profile and policy for update tests", func(t *testing.T) {
		createProfileParams := &CreateProfileParams{
			Name:           "profile2",
			UeAmbrUplink:   "100 Mbps",
			UeAmbrDownlink: "100 Mbps",
		}

		statusCode, createProfileResponse, err := createProfile(env.Server.URL, client, token, createProfileParams)
		if err != nil {
			t.Fatalf("couldn't create profile: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if createProfileResponse.Error != "" {
			t.Fatalf("unexpected error :%q", createProfileResponse.Error)
		}

		createPolicyParams := &CreatePolicyParams{
			Name:                "policy2",
			ProfileName:         "profile2",
			SliceName:           DefaultSliceName,
			SessionAmbrUplink:   "50 Mbps",
			SessionAmbrDownlink: "50 Mbps",
			Var5qi:              8,
			Arp:                 2,
			DataNetworkName:     "whatever",
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
			Imsi:        Imsi,
			ProfileName: "profile2",
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

		if getResponse.Result.ProfileName != "profile2" {
			t.Fatalf("expected profileName 'profile2', got %s", getResponse.Result.ProfileName)
		}
	})

	t.Run("8. Update subscriber - missing imsi in path", func(t *testing.T) {
		updateParams := &UpdateSubscriberParams{
			Imsi:        Imsi,
			ProfileName: TestProfileName,
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
			Imsi:        Imsi,
			ProfileName: "",
		}

		statusCode, response, err := updateSubscriber(env.Server.URL, client, token, Imsi, updateParams)
		if err != nil {
			t.Fatalf("couldn't update subscriber: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "Missing profile_name parameter" {
			t.Fatalf("expected error 'Missing profile_name parameter', got %q", response.Error)
		}
	})

	t.Run("11. Update subscriber - not found", func(t *testing.T) {
		updateParams := &UpdateSubscriberParams{
			Imsi:        "invalid-imsi",
			ProfileName: TestProfileName,
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
			Imsi:        Imsi,
			ProfileName: "nonexistent-profile",
		}

		statusCode, response, err := updateSubscriber(env.Server.URL, client, token, Imsi, updateParams)
		if err != nil {
			t.Fatalf("couldn't update subscriber: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "Profile not found" {
			t.Fatalf("expected error 'Profile not found', got %q", response.Error)
		}
	})

	t.Run("13. Update subscriber - subscriber not found", func(t *testing.T) {
		updateParams := &UpdateSubscriberParams{
			Imsi:        "001010100007488",
			ProfileName: TestProfileName,
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
			ProfileName:    TestProfileName,
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

		if response.Result.ProfileName != TestProfileName {
			t.Fatalf("expected profileName %s, got %s", TestProfileName, response.Result.ProfileName)
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

		if response.Result.Sessions == nil {
			t.Fatalf("expected sessions field to be present, got nil")
		}

		if len(response.Result.Sessions) != 0 {
			t.Fatalf("expected 0 sessions, got %d", len(response.Result.Sessions))
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

		if response.Result.Sessions == nil {
			t.Fatalf("expected sessions field to be present, got nil")
		}

		if len(response.Result.Sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(response.Result.Sessions))
		}

		session := response.Result.Sessions[0]

		if session.ID != 1 {
			t.Fatalf("expected session ID 1, got %d", session.ID)
		}

		if session.RadioAccessType != "5G" {
			t.Fatalf("expected radio_access_type '5G', got %q", session.RadioAccessType)
		}

		if got := response.Result.Status.RadioAccessTypes; len(got) != 1 || got[0] != "5G" {
			t.Fatalf("expected radio_access_types [5G], got %v", got)
		}

		if session.Status != "active" {
			t.Fatalf("expected session status 'active', got %q", session.Status)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})
}

func TestCreateSubscriberInvalidInput(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

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
			error:          "Invalid IMSI format. Must be a string of 6 to 15 digits starting with `<mcc><mnc>`.",
		},
		{
			imsi:           "00101010000748812",
			key:            Key,
			sequenceNumber: SequenceNumber,
			error:          "Invalid IMSI format. Must be a string of 6 to 15 digits starting with `<mcc><mnc>`.",
		},
		{
			imsi:           "002010100007488",
			key:            Key,
			sequenceNumber: SequenceNumber,
			error:          "Invalid IMSI format. Must be a string of 6 to 15 digits starting with `<mcc><mnc>`.",
		},
		{
			imsi:           "00101",
			key:            Key,
			sequenceNumber: SequenceNumber,
			error:          "Invalid IMSI format. Must be a string of 6 to 15 digits starting with `<mcc><mnc>`.",
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
				ProfileName:    DefaultProfileName,
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
	env, client, token := newAuthedTestEnv(t)

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
		{
			mcc:  "001",
			mnc:  "01",
			imsi: "001011",
		},
		{
			mcc:  "001",
			mnc:  "01",
			imsi: "0010112345",
		},
		{
			mcc:  "001",
			mnc:  "001",
			imsi: "0010017",
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
				ProfileName:    "default",
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

func TestCreateSubscriberRejectsEmptyMSIN(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	statusCode, _, err := updateOperatorID(env.Server.URL, client, token, &UpdateOperatorIDParams{Mcc: "001", Mnc: "001"})
	if err != nil {
		t.Fatalf("couldn't update operator ID: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	statusCode, response, err := createSubscriber(env.Server.URL, client, token, &CreateSubscriberParams{
		Imsi:           "001001",
		Key:            Key,
		SequenceNumber: SequenceNumber,
		ProfileName:    "default",
	})
	if err != nil {
		t.Fatalf("couldn't create subscriber: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Fatalf("an IMSI that is only MCC+MNC was accepted: status %d", statusCode)
	}

	want := "Invalid IMSI format. Must be a string of 6 to 15 digits starting with `<mcc><mnc>`."
	if response.Error != want {
		t.Errorf("error = %q, want %q", response.Error, want)
	}
}

func TestCreateTooManySubscribers(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	createDataNetworkParams := &CreateDataNetworkParams{
		Name:     "whatever",
		MTU:      MTU,
		IPv4Pool: IPv4Pool,
		DNS:      DNS,
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

	createProfileParams := &CreateProfileParams{
		Name:           TestProfileName,
		UeAmbrUplink:   "200 Mbps",
		UeAmbrDownlink: "200 Mbps",
	}

	statusCode, createProfileResponse, err := createProfile(env.Server.URL, client, token, createProfileParams)
	if err != nil {
		t.Fatalf("couldn't create profile: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	if createProfileResponse.Error != "" {
		t.Fatalf("unexpected error :%q", createProfileResponse.Error)
	}

	createPolicyParams := &CreatePolicyParams{
		Name:                PolicyName,
		ProfileName:         TestProfileName,
		SliceName:           DefaultSliceName,
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkName:     "whatever",
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
			ProfileName:    TestProfileName,
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
		ProfileName:    TestProfileName,
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

func listSubscribersByDataNetwork(url string, client *http.Client, token, dn string) (int, *ListSubscriberResponse, error) {
	return apiDo[ListSubscriberResponse](client, "GET", url+"/api/v1/subscribers?data_network="+dn, token, nil)
}

func TestListSubscribers_DataNetworkFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)
	url := env.Server.URL

	token, err := initializeAndRefresh(url, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	mustOK := func(name string, code int, callErr error) {
		t.Helper()

		if callErr != nil {
			t.Fatalf("%s: %s", name, callErr)
		}

		if code != http.StatusCreated {
			t.Fatalf("%s: expected 201, got %d", name, code)
		}
	}

	sc, _, err := createSlice(url, client, token, &CreateSliceParams{Name: "filter-slice", Sst: 1})
	mustOK("createSlice", sc, err)

	// Two profile → data network → subscriber chains, each reaching only its own DN.
	for _, chain := range []struct{ dn, pool, profile, policy, imsi string }{
		{"filter-dn-1", "10.61.0.0/24", "filter-profile-1", "filter-policy-1", "001010100000061"},
		{"filter-dn-2", "10.62.0.0/24", "filter-profile-2", "filter-policy-2", "001010100000062"},
	} {
		sc, _, err = createDataNetwork(url, client, token, &CreateDataNetworkParams{Name: chain.dn, IPv4Pool: chain.pool, DNS: DNS, MTU: MTU})
		mustOK("createDataNetwork "+chain.dn, sc, err)

		sc, _, err = createProfile(url, client, token, &CreateProfileParams{Name: chain.profile, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps"})
		mustOK("createProfile "+chain.profile, sc, err)

		sc, _, err = createPolicy(url, client, token, &CreatePolicyParams{
			Name: chain.policy, ProfileName: chain.profile, SliceName: "filter-slice",
			SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1, DataNetworkName: chain.dn,
		})
		mustOK("createPolicy "+chain.policy, sc, err)

		sc, _, err = createSubscriber(url, client, token, &CreateSubscriberParams{Imsi: chain.imsi, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: chain.profile})
		mustOK("createSubscriber "+chain.imsi, sc, err)
	}

	t.Run("filter returns only entitled subscribers", func(t *testing.T) {
		code, resp, err := listSubscribersByDataNetwork(url, client, token, "filter-dn-1")
		if err != nil || code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%v)", code, err)
		}

		if len(resp.Result.Items) != 1 || resp.Result.Items[0].Imsi != "001010100000061" {
			t.Fatalf("expected only 001010100000061, got %+v", resp.Result.Items)
		}

		code, resp, err = listSubscribersByDataNetwork(url, client, token, "filter-dn-2")
		if err != nil || code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%v)", code, err)
		}

		if len(resp.Result.Items) != 1 || resp.Result.Items[0].Imsi != "001010100000062" {
			t.Fatalf("expected only 001010100000062, got %+v", resp.Result.Items)
		}
	})

	t.Run("unknown data network is 404", func(t *testing.T) {
		code, _, _ := listSubscribersByDataNetwork(url, client, token, "no-such-dn")
		if code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", code)
		}
	})
}

func listSubscribersWithQuery(url string, client *http.Client, token, query string) (int, *ListSubscriberResponse, error) {
	return apiDo[ListSubscriberResponse](client, "GET", url+"/api/v1/subscribers?"+query, token, nil)
}

func TestListSubscribers_SearchFilter(t *testing.T) {
	env, err := setupServer(filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)
	url := env.Server.URL

	token, err := initializeAndRefresh(url, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	mustCreate := func(name string, code int, callErr error) {
		t.Helper()

		if callErr != nil {
			t.Fatalf("%s: %s", name, callErr)
		}

		if code != http.StatusCreated {
			t.Fatalf("%s: expected 201, got %d", name, code)
		}
	}

	sc, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{Name: "search-dn", IPv4Pool: IPv4Pool, DNS: DNS, MTU: MTU})
	mustCreate("createDataNetwork", sc, err)

	sc, _, err = createSlice(url, client, token, &CreateSliceParams{Name: "search-slice", Sst: 1})
	mustCreate("createSlice", sc, err)

	sc, _, err = createProfile(url, client, token, &CreateProfileParams{Name: TestProfileName, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps"})
	mustCreate("createProfile", sc, err)

	sc, _, err = createPolicy(url, client, token, &CreatePolicyParams{
		Name: "search-policy", ProfileName: TestProfileName, SliceName: "search-slice",
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1, DataNetworkName: "search-dn",
	})
	mustCreate("createPolicy", sc, err)

	imsis := []string{"001010100007487", "001010100007488", "001010100009999"}
	for _, imsi := range imsis {
		sc, _, err = createSubscriber(url, client, token, &CreateSubscriberParams{
			Imsi: imsi, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: TestProfileName,
		})
		mustCreate("createSubscriber "+imsi, sc, err)
	}

	list := func(t *testing.T, query string) *ListSubscriberResponseResult {
		t.Helper()

		code, resp, err := listSubscribersWithQuery(url, client, token, query)
		if err != nil {
			t.Fatalf("list %q: %s", query, err)
		}

		if code != http.StatusOK {
			t.Fatalf("list %q: expected 200, got %d (%q)", query, code, resp.Error)
		}

		return &resp.Result
	}

	t.Run("substring narrows results and total_count", func(t *testing.T) {
		res := list(t, "search=0748")
		if len(res.Items) != 2 || res.TotalCount != 2 {
			t.Fatalf("expected 2 matches, got total=%d items=%+v", res.TotalCount, res.Items)
		}

		res = list(t, "search=9999")
		if len(res.Items) != 1 || res.Items[0].Imsi != "001010100009999" {
			t.Fatalf("expected only 001010100009999, got %+v", res.Items)
		}
	})

	t.Run("no match returns an empty page", func(t *testing.T) {
		res := list(t, "search=12345")
		if len(res.Items) != 0 || res.TotalCount != 0 {
			t.Fatalf("expected no matches, got total=%d items=%+v", res.TotalCount, res.Items)
		}
	})

	t.Run("empty and whitespace search behave as unset", func(t *testing.T) {
		for _, query := range []string{"", "search=", "search=%20%20"} {
			res := list(t, query)
			if len(res.Items) != len(imsis) || res.TotalCount != len(imsis) {
				t.Fatalf("query %q: expected all %d subscribers, got total=%d items=%+v", query, len(imsis), res.TotalCount, res.Items)
			}
		}
	})

	t.Run("wildcards are literal", func(t *testing.T) {
		for _, query := range []string{"search=%25", "search=_"} {
			res := list(t, query)
			if len(res.Items) != 0 || res.TotalCount != 0 {
				t.Fatalf("query %q: expected no matches, got total=%d items=%+v", query, res.TotalCount, res.Items)
			}
		}
	})

	t.Run("combines with the data network filter", func(t *testing.T) {
		res := list(t, "search=0748&data_network=search-dn")
		if len(res.Items) != 2 || res.TotalCount != 2 {
			t.Fatalf("expected 2 matches on the bound data network, got total=%d items=%+v", res.TotalCount, res.Items)
		}
	})

	t.Run("paginates within the filtered set", func(t *testing.T) {
		res := list(t, "search=0748&page=1&per_page=1")
		if len(res.Items) != 1 || res.TotalCount != 2 || res.Items[0].Imsi != "001010100007487" {
			t.Fatalf("expected page 1 of 2 matches, got total=%d items=%+v", res.TotalCount, res.Items)
		}

		res = list(t, "search=0748&page=2&per_page=1")
		if len(res.Items) != 1 || res.TotalCount != 2 || res.Items[0].Imsi != "001010100007488" {
			t.Fatalf("expected page 2 of 2 matches, got total=%d items=%+v", res.TotalCount, res.Items)
		}
	})

	t.Run("the length cap counts characters, not bytes", func(t *testing.T) {
		code, resp, err := listSubscribersWithQuery(url, client, token,
			"search="+url2.QueryEscape(strings.Repeat("é", server.MaxSearchLength)))
		if err != nil {
			t.Fatalf("list: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("expected 200 for a %d-character search, got %d (%q)", server.MaxSearchLength, code, resp.Error)
		}
	})

	t.Run("over-long search is rejected", func(t *testing.T) {
		code, resp, err := listSubscribersWithQuery(url, client, token, "search="+strings.Repeat("1", server.MaxSearchLength+1))
		if err != nil {
			t.Fatalf("list: %s", err)
		}

		if code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", code)
		}

		if resp.Error == "" {
			t.Fatal("expected an error message")
		}
	})
}

func TestSubscriberDescription(t *testing.T) {
	env, err := setupServer(filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)
	url := env.Server.URL

	token, err := initializeAndRefresh(url, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	mustCreate := func(name string, code int, callErr error) {
		t.Helper()

		if callErr != nil {
			t.Fatalf("%s: %s", name, callErr)
		}

		if code != http.StatusCreated {
			t.Fatalf("%s: expected 201, got %d", name, code)
		}
	}

	sc, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{Name: "description-dn", IPv4Pool: IPv4Pool, DNS: DNS, MTU: MTU})
	mustCreate("createDataNetwork", sc, err)

	sc, _, err = createSlice(url, client, token, &CreateSliceParams{Name: "description-slice", Sst: 1})
	mustCreate("createSlice", sc, err)

	sc, _, err = createProfile(url, client, token, &CreateProfileParams{Name: TestProfileName, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps"})
	mustCreate("createProfile", sc, err)

	sc, _, err = createPolicy(url, client, token, &CreatePolicyParams{
		Name: "description-policy", ProfileName: TestProfileName, SliceName: "description-slice",
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1, DataNetworkName: "description-dn",
	})
	mustCreate("createPolicy", sc, err)

	const (
		describedIMSI = "001010100007487"
		plainIMSI     = "001010100007488"
	)

	sc, _, err = createSubscriber(url, client, token, &CreateSubscriberParams{
		Imsi: describedIMSI, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber,
		ProfileName: TestProfileName, Description: "  Warehouse gate reader  ",
	})
	mustCreate("createSubscriber "+describedIMSI, sc, err)

	sc, _, err = createSubscriber(url, client, token, &CreateSubscriberParams{
		Imsi: plainIMSI, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: TestProfileName,
	})
	mustCreate("createSubscriber "+plainIMSI, sc, err)

	get := func(t *testing.T, imsi string) *SubscriberDetail {
		t.Helper()

		code, resp, err := getSubscriber(url, client, token, imsi)
		if err != nil {
			t.Fatalf("get %s: %s", imsi, err)
		}

		if code != http.StatusOK {
			t.Fatalf("get %s: expected 200, got %d (%q)", imsi, code, resp.Error)
		}

		return &resp.Result
	}

	t.Run("create stores a trimmed description", func(t *testing.T) {
		if got := get(t, describedIMSI).Description; got != "Warehouse gate reader" {
			t.Fatalf("description = %q, want %q", got, "Warehouse gate reader")
		}
	})

	t.Run("create without a description leaves it unset", func(t *testing.T) {
		if got := get(t, plainIMSI).Description; got != "" {
			t.Fatalf("description = %q, want %q", got, "")
		}
	})

	t.Run("list reports the description", func(t *testing.T) {
		code, resp, err := listSubscribers(url, client, token, 1, 25)
		if err != nil {
			t.Fatalf("list: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("list: expected 200, got %d (%q)", code, resp.Error)
		}

		byIMSI := make(map[string]string, len(resp.Result.Items))
		for _, item := range resp.Result.Items {
			byIMSI[item.Imsi] = item.Description
		}

		if byIMSI[describedIMSI] != "Warehouse gate reader" {
			t.Fatalf("listed description = %q, want %q", byIMSI[describedIMSI], "Warehouse gate reader")
		}

		if byIMSI[plainIMSI] != "" {
			t.Fatalf("listed description = %q, want %q", byIMSI[plainIMSI], "")
		}
	})

	t.Run("search matches a description-only hit", func(t *testing.T) {
		code, resp, err := listSubscribersWithQuery(url, client, token, "search=gate")
		if err != nil {
			t.Fatalf("list: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("list: expected 200, got %d (%q)", code, resp.Error)
		}

		if len(resp.Result.Items) != 1 || resp.Result.TotalCount != 1 || resp.Result.Items[0].Imsi != describedIMSI {
			t.Fatalf("expected only %s, got total=%d items=%+v", describedIMSI, resp.Result.TotalCount, resp.Result.Items)
		}
	})

	t.Run("update keeps the description when it is resent", func(t *testing.T) {
		code, resp, err := updateSubscriber(url, client, token, describedIMSI, &UpdateSubscriberParams{
			ProfileName: TestProfileName, Description: "Warehouse gate reader",
		})
		if err != nil {
			t.Fatalf("update: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("update: expected 200, got %d (%q)", code, resp.Error)
		}

		if got := get(t, describedIMSI).Description; got != "Warehouse gate reader" {
			t.Fatalf("description = %q, want %q", got, "Warehouse gate reader")
		}
	})

	t.Run("update clears the description when it is omitted", func(t *testing.T) {
		code, resp, err := updateSubscriber(url, client, token, describedIMSI, &UpdateSubscriberParams{
			ProfileName: TestProfileName,
		})
		if err != nil {
			t.Fatalf("update: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("update: expected 200, got %d (%q)", code, resp.Error)
		}

		if got := get(t, describedIMSI).Description; got != "" {
			t.Fatalf("description = %q, want %q", got, "")
		}
	})

	t.Run("an over-long description is rejected", func(t *testing.T) {
		tooLong := strings.Repeat("é", server.MaxDescriptionLength+1)

		code, resp, err := createSubscriber(url, client, token, &CreateSubscriberParams{
			Imsi: "001010100009999", Key: Key, Opc: Opc, SequenceNumber: SequenceNumber,
			ProfileName: TestProfileName, Description: tooLong,
		})
		if err != nil {
			t.Fatalf("create: %s", err)
		}

		if code != http.StatusBadRequest {
			t.Fatalf("create with a %d-character description: expected 400, got %d", server.MaxDescriptionLength+1, code)
		}

		if resp.Error == "" {
			t.Fatal("expected an error message")
		}

		updateCode, updateResp, err := updateSubscriber(url, client, token, plainIMSI, &UpdateSubscriberParams{
			ProfileName: TestProfileName, Description: tooLong,
		})
		if err != nil {
			t.Fatalf("update: %s", err)
		}

		if updateCode != http.StatusBadRequest {
			t.Fatalf("update with a %d-character description: expected 400, got %d", server.MaxDescriptionLength+1, updateCode)
		}

		if updateResp.Error == "" {
			t.Fatal("expected an error message")
		}
	})

	t.Run("the audit entry names the description only when it changed", func(t *testing.T) {
		auditDetails := func(imsi string) []string {
			t.Helper()

			_, auditResp, err := listAuditLogs(url, client, token, 1, 100)
			if err != nil {
				t.Fatalf("couldn't list audit logs: %s", err)
			}

			var details []string

			for _, entry := range auditResp.Result.Items {
				if entry.Action == "update_subscriber" && strings.Contains(entry.Details, imsi) {
					details = append(details, entry.Details)
				}
			}

			return details
		}

		before := len(auditDetails(plainIMSI))

		// A profile-only update of a subscriber that never had a description.
		code, resp, err := updateSubscriber(url, client, token, plainIMSI, &UpdateSubscriberParams{
			ProfileName: TestProfileName,
		})
		if err != nil {
			t.Fatalf("update: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("update: expected 200, got %d (%q)", code, resp.Error)
		}

		details := auditDetails(plainIMSI)
		if len(details) != before+1 {
			t.Fatalf("expected one new update_subscriber entry, got %d then %d", before, len(details))
		}

		latest := details[0]
		if want := "User updated subscriber: " + plainIMSI; latest != want {
			t.Fatalf("audit detail = %q, want %q", latest, want)
		}
	})

	t.Run("the length cap counts characters, not bytes", func(t *testing.T) {
		code, resp, err := updateSubscriber(url, client, token, plainIMSI, &UpdateSubscriberParams{
			ProfileName: TestProfileName, Description: strings.Repeat("é", server.MaxDescriptionLength),
		})
		if err != nil {
			t.Fatalf("update: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("update with a %d-character description: expected 200, got %d (%q)", server.MaxDescriptionLength, code, resp.Error)
		}
	})
}
