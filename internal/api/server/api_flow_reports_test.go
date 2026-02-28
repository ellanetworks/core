// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
)

type FlowReport struct {
	ID              int    `json:"id"`
	SubscriberID    string `json:"subscriber_id"`
	SourceIP        string `json:"source_ip"`
	DestinationIP   string `json:"destination_ip"`
	SourcePort      uint16 `json:"source_port"`
	DestinationPort uint16 `json:"destination_port"`
	Protocol        uint8  `json:"protocol"`
	Packets         uint64 `json:"packets"`
	Bytes           uint64 `json:"bytes"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
}

type ListFlowReportsResponseResult struct {
	Items      []FlowReport `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type ListFlowReportsResponse struct {
	Result ListFlowReportsResponseResult `json:"result"`
	Error  string                        `json:"error,omitempty"`
}

type GetFlowReportsRetentionPolicyResponseResult struct {
	Days int `json:"days"`
}

type GetFlowReportsRetentionPolicyResponse struct {
	Result *GetFlowReportsRetentionPolicyResponseResult `json:"result,omitempty"`
	Error  string                                       `json:"error,omitempty"`
}

type UpdateFlowReportsRetentionPolicyResponseResult struct {
	Message string `json:"message"`
}

type UpdateFlowReportsRetentionPolicyResponse struct {
	Result *UpdateFlowReportsRetentionPolicyResponseResult `json:"result,omitempty"`
	Error  string                                          `json:"error,omitempty"`
}

type UpdateFlowReportsRetentionPolicyParams struct {
	Days int `json:"days"`
}

type ClearFlowReportsResponse struct {
	Result *UpdateFlowReportsRetentionPolicyResponseResult `json:"result,omitempty"`
	Error  string                                          `json:"error,omitempty"`
}

func listFlowReports(url string, client *http.Client, token string, page int, perPage int, filters map[string]string) (int, *ListFlowReportsResponse, error) {
	var queryParams []string

	queryParams = append(queryParams, fmt.Sprintf("page=%d", page))
	queryParams = append(queryParams, fmt.Sprintf("per_page=%d", perPage))

	for k, v := range filters {
		queryParams = append(queryParams, fmt.Sprintf("%s=%s", k, v))
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/flow-reports?%s", url, strings.Join(queryParams, "&")), nil)
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

	var flowReportsResponse ListFlowReportsResponse

	if err := json.NewDecoder(res.Body).Decode(&flowReportsResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &flowReportsResponse, nil
}

func getFlowReportsRetentionPolicy(url string, client *http.Client, token string) (int, *GetFlowReportsRetentionPolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/flow-reports/retention", nil)
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

	var retentionPolicyResponse GetFlowReportsRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&retentionPolicyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &retentionPolicyResponse, nil
}

func updateFlowReportsRetentionPolicy(url string, client *http.Client, token string, data *UpdateFlowReportsRetentionPolicyParams) (int, *UpdateFlowReportsRetentionPolicyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/flow-reports/retention", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response UpdateFlowReportsRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func clearFlowReports(url string, client *http.Client, token string) (int, *ClearFlowReportsResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/flow-reports", nil)
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

	var response ClearFlowReportsResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func TestAPIFlowReports(t *testing.T) {
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

	statusCode, response, err := listFlowReports(ts.URL, client, token, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 0 {
		t.Fatalf("expected 0 flow reports, got %d", len(response.Result.Items))
	}

	if response.Result.Page != 1 {
		t.Fatalf("expected page to be 1, got %d", response.Result.Page)
	}

	if response.Result.PerPage != 10 {
		t.Fatalf("expected per_page to be 10, got %d", response.Result.PerPage)
	}

	if response.Result.TotalCount != 0 {
		t.Fatalf("expected total_count to be 0, got %d", response.Result.TotalCount)
	}

	if response.Error != "" {
		t.Fatalf("unexpected error: %q", response.Error)
	}
}

func TestGetFlowReportsRetentionPolicy(t *testing.T) {
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

	statusCode, response, err := getFlowReportsRetentionPolicy(ts.URL, client, token)
	if err != nil {
		t.Fatalf("couldn't get retention policy: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Result == nil {
		t.Fatalf("expected result to not be nil")
	}

	if response.Result.Days != 7 {
		t.Fatalf("expected default retention policy to be 7 days, got %d", response.Result.Days)
	}
}

func TestUpdateFlowReportsRetentionPolicy(t *testing.T) {
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

	// Update retention policy to 14 days
	statusCode, response, err := updateFlowReportsRetentionPolicy(ts.URL, client, token, &UpdateFlowReportsRetentionPolicyParams{Days: 14})
	if err != nil {
		t.Fatalf("couldn't update retention policy: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Result == nil {
		t.Fatalf("expected result to not be nil")
	}

	if strings.Contains(response.Result.Message, "successfully") == false {
		t.Fatalf("expected success message, got: %q", response.Result.Message)
	}

	// Verify the retention policy was updated
	statusCode, getResponse, err := getFlowReportsRetentionPolicy(ts.URL, client, token)
	if err != nil {
		t.Fatalf("couldn't get retention policy: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if getResponse.Result.Days != 14 {
		t.Fatalf("expected retention policy to be 14 days, got %d", getResponse.Result.Days)
	}
}

func TestUpdateFlowReportsRetentionPolicyInvalidInput(t *testing.T) {
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

	testCases := []struct {
		name       string
		days       int
		shouldFail bool
	}{
		{"Zero days", 0, true},
		{"Negative days", -5, true},
		{"Valid days", 10, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			statusCode, _, err := updateFlowReportsRetentionPolicy(ts.URL, client, token, &UpdateFlowReportsRetentionPolicyParams{Days: tc.days})
			if err != nil {
				t.Fatalf("error: %s", err)
			}

			if tc.shouldFail {
				if statusCode == http.StatusOK {
					t.Fatalf("expected failure for %s", tc.name)
				}
			} else {
				if statusCode != http.StatusOK {
					t.Fatalf("expected success for %s", tc.name)
				}
			}
		})
	}
}

func createFlowReportTestSubscriber(t *testing.T, dbInstance *db.Database) int {
	t.Helper()

	const imsi = "001010100000001"

	ctx := context.Background()

	dataNetwork := &db.DataNetwork{
		Name:   "test-dn-" + imsi,
		IPPool: "10.0.0.0/24",
	}
	if err := dbInstance.CreateDataNetwork(ctx, dataNetwork); err != nil {
		t.Fatalf("couldn't create data network: %s", err)
	}

	createdDN, err := dbInstance.GetDataNetwork(ctx, dataNetwork.Name)
	if err != nil {
		t.Fatalf("couldn't get data network: %s", err)
	}

	policy := &db.Policy{
		Name:            "test-policy-" + imsi,
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   createdDN.ID,
	}
	if err := dbInstance.CreatePolicy(ctx, policy); err != nil {
		t.Fatalf("couldn't create policy: %s", err)
	}

	createdPolicy, err := dbInstance.GetPolicy(ctx, policy.Name)
	if err != nil {
		t.Fatalf("couldn't get policy: %s", err)
	}

	subscriber := &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000022",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		PolicyID:       createdPolicy.ID,
	}
	if err := dbInstance.CreateSubscriber(ctx, subscriber); err != nil {
		t.Fatalf("couldn't create subscriber: %s", err)
	}

	return createdPolicy.ID
}

func TestListFlowReportsPagination(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, dbInstance, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	// Create prerequisite subscriber
	createFlowReportTestSubscriber(t, dbInstance)

	// Insert test flow reports
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range 25 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "001010100000001",
			SourceIP:        "192.168.1.100",
			DestinationIP:   "8.8.8.8",
			SourcePort:      uint16(10000 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         100,
			Bytes:           5000,
			StartTime:       now,
			EndTime:         now,
		}
		if err := dbInstance.InsertFlowReport(context.Background(), fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	// Test page 1
	statusCode, response, err := listFlowReports(ts.URL, client, token, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 10 {
		t.Fatalf("expected 10 items on page 1, got %d", len(response.Result.Items))
	}

	if response.Result.TotalCount != 25 {
		t.Fatalf("expected total count to be 25, got %d", response.Result.TotalCount)
	}

	// Test page 2
	statusCode, response, err = listFlowReports(ts.URL, client, token, 2, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 10 {
		t.Fatalf("expected 10 items on page 2, got %d", len(response.Result.Items))
	}

	// Test page 3 (partial)
	statusCode, response, err = listFlowReports(ts.URL, client, token, 3, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 5 {
		t.Fatalf("expected 5 items on page 3, got %d", len(response.Result.Items))
	}
}

func TestListFlowReportsFilterBySubscriber(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, dbInstance, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	// Create prerequisite subscribers
	policyID := createFlowReportTestSubscriber(t, dbInstance)

	sub2 := &db.Subscriber{
		Imsi:           "001010100000002",
		SequenceNumber: "000000000022",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		PolicyID:       policyID,
	}
	if err := dbInstance.CreateSubscriber(context.Background(), sub2); err != nil {
		t.Fatalf("couldn't create subscriber: %s", err)
	}

	// Insert test flow reports with different subscribers
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range 5 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "001010100000001",
			SourceIP:        "192.168.1.100",
			DestinationIP:   "8.8.8.8",
			SourcePort:      uint16(10000 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         100,
			Bytes:           5000,
			StartTime:       now,
			EndTime:         now,
		}
		if err := dbInstance.InsertFlowReport(context.Background(), fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	for i := range 3 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "001010100000002",
			SourceIP:        "192.168.1.101",
			DestinationIP:   "8.8.4.4",
			SourcePort:      uint16(20000 + i),
			DestinationPort: 443,
			Protocol:        6,
			Packets:         200,
			Bytes:           10000,
			StartTime:       now,
			EndTime:         now,
		}
		if err := dbInstance.InsertFlowReport(context.Background(), fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	// Filter by 001010100000001
	statusCode, response, err := listFlowReports(ts.URL, client, token, 1, 100, map[string]string{"subscriber_id": "001010100000001"})
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 5 {
		t.Fatalf("expected 5 flow reports for 001010100000001, got %d", len(response.Result.Items))
	}

	for _, item := range response.Result.Items {
		if item.SubscriberID != "001010100000001" {
			t.Fatalf("expected all items to have subscriber_id=001010100000001, got %s", item.SubscriberID)
		}
	}

	// Filter by 001010100000002
	statusCode, response, err = listFlowReports(ts.URL, client, token, 1, 100, map[string]string{"subscriber_id": "001010100000002"})
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 3 {
		t.Fatalf("expected 3 flow reports for 001010100000002, got %d", len(response.Result.Items))
	}
}

func TestClearFlowReports(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, dbInstance, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	// Create prerequisite subscriber
	createFlowReportTestSubscriber(t, dbInstance)

	// Insert test flow reports
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range 10 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "001010100000001",
			SourceIP:        "192.168.1.100",
			DestinationIP:   "8.8.8.8",
			SourcePort:      uint16(10000 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         100,
			Bytes:           5000,
			StartTime:       now,
			EndTime:         now,
		}
		if err := dbInstance.InsertFlowReport(context.Background(), fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	// Verify reports were inserted
	statusCode, response, err := listFlowReports(ts.URL, client, token, 1, 100, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Result.TotalCount != 10 {
		t.Fatalf("expected 10 reports before clear, got %d", response.Result.TotalCount)
	}

	// Clear all flow reports
	statusCode, clearResponse, err := clearFlowReports(ts.URL, client, token)
	if err != nil {
		t.Fatalf("couldn't clear flow reports: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if clearResponse.Result == nil {
		t.Fatalf("expected result to not be nil")
	}

	// Verify all reports were cleared
	statusCode, response, err = listFlowReports(ts.URL, client, token, 1, 100, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Result.TotalCount != 0 {
		t.Fatalf("expected 0 reports after clear, got %d", response.Result.TotalCount)
	}
}

type FlowReportProtocolStat struct {
	Protocol uint8 `json:"protocol"`
	Count    int   `json:"count"`
}

type FlowReportIPStat struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

type FlowReportStatsResult struct {
	Protocols             []FlowReportProtocolStat `json:"protocols"`
	TopDestinationsUplink []FlowReportIPStat       `json:"top_destinations_uplink"`
}

type GetFlowReportStatsResponse struct {
	Result *FlowReportStatsResult `json:"result,omitempty"`
	Error  string                 `json:"error,omitempty"`
}

func getFlowReportStats(url string, client *http.Client, token string, filters map[string]string) (int, *GetFlowReportStatsResponse, error) {
	var queryParams []string

	for k, v := range filters {
		queryParams = append(queryParams, fmt.Sprintf("%s=%s", k, v))
	}

	reqURL := url + "/api/v1/flow-reports/stats"
	if len(queryParams) > 0 {
		reqURL += "?" + strings.Join(queryParams, "&")
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", reqURL, nil)
	if err != nil {
		return 0, nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

	var statsResponse GetFlowReportStatsResponse

	if err := json.NewDecoder(res.Body).Decode(&statsResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &statsResponse, nil
}

func TestGetFlowReportStats_Empty(t *testing.T) {
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

	statusCode, response, err := getFlowReportStats(ts.URL, client, token, nil)
	if err != nil {
		t.Fatalf("couldn't get flow report stats: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Result == nil {
		t.Fatalf("expected result to not be nil")
	}

	if len(response.Result.Protocols) != 0 {
		t.Fatalf("expected 0 protocol entries, got %d", len(response.Result.Protocols))
	}

	if len(response.Result.TopDestinationsUplink) != 0 {
		t.Fatalf("expected 0 top destinations uplink, got %d", len(response.Result.TopDestinationsUplink))
	}
}

func TestGetFlowReportStats_WithData(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, dbInstance, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	createFlowReportTestSubscriber(t, dbInstance)

	now := time.Now().UTC().Format(time.RFC3339)

	// Insert 3 TCP flows from different sources to different destinations
	for i := range 3 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "001010100000001",
			SourceIP:        fmt.Sprintf("10.0.0.%d", i+1),
			DestinationIP:   fmt.Sprintf("8.8.8.%d", i+1),
			SourcePort:      uint16(10000 + i),
			DestinationPort: 443,
			Protocol:        6,
			Packets:         100,
			Bytes:           5000,
			StartTime:       now,
			EndTime:         now,
		}
		if err := dbInstance.InsertFlowReport(context.Background(), fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	// Insert 2 UDP flows
	for i := range 2 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "001010100000001",
			SourceIP:        fmt.Sprintf("192.168.0.%d", i+1),
			DestinationIP:   "1.1.1.1",
			SourcePort:      uint16(20000 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         50,
			Bytes:           2500,
			StartTime:       now,
			EndTime:         now,
		}
		if err := dbInstance.InsertFlowReport(context.Background(), fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	statusCode, response, err := getFlowReportStats(ts.URL, client, token, nil)
	if err != nil {
		t.Fatalf("couldn't get flow report stats: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Result == nil {
		t.Fatalf("expected result to not be nil")
	}

	if len(response.Result.Protocols) != 2 {
		t.Fatalf("expected 2 protocol entries, got %d", len(response.Result.Protocols))
	}

	// TCP should be first (higher count)
	if response.Result.Protocols[0].Protocol != 6 {
		t.Fatalf("expected first protocol to be TCP (6), got %d", response.Result.Protocols[0].Protocol)
	}

	if response.Result.Protocols[0].Count != 3 {
		t.Fatalf("expected TCP count 3, got %d", response.Result.Protocols[0].Count)
	}
}

func TestGetFlowReportStats_FilterBySubscriber(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, dbInstance, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	policyID := createFlowReportTestSubscriber(t, dbInstance)

	sub2 := &db.Subscriber{
		Imsi:           "001010100000002",
		SequenceNumber: "000000000022",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		PolicyID:       policyID,
	}
	if err := dbInstance.CreateSubscriber(context.Background(), sub2); err != nil {
		t.Fatalf("couldn't create subscriber: %s", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// 3 TCP flows for subscriber 1
	for i := range 3 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "001010100000001",
			SourceIP:        "10.0.0.1",
			DestinationIP:   "8.8.8.8",
			SourcePort:      uint16(10000 + i),
			DestinationPort: 443,
			Protocol:        6,
			Packets:         100,
			Bytes:           5000,
			StartTime:       now,
			EndTime:         now,
		}
		if err := dbInstance.InsertFlowReport(context.Background(), fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	// 2 UDP flows for subscriber 2
	for i := range 2 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "001010100000002",
			SourceIP:        "10.0.1.1",
			DestinationIP:   "1.1.1.1",
			SourcePort:      uint16(20000 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         50,
			Bytes:           2500,
			StartTime:       now,
			EndTime:         now,
		}
		if err := dbInstance.InsertFlowReport(context.Background(), fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	statusCode, response, err := getFlowReportStats(ts.URL, client, token, map[string]string{"subscriber_id": "001010100000001"})
	if err != nil {
		t.Fatalf("couldn't get flow report stats: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Result == nil {
		t.Fatalf("expected result to not be nil")
	}

	// Only subscriber 1's flows: 1 protocol (TCP), 1 source, 1 destination
	if len(response.Result.Protocols) != 1 {
		t.Fatalf("expected 1 protocol entry, got %d", len(response.Result.Protocols))
	}

	if response.Result.Protocols[0].Protocol != 6 {
		t.Fatalf("expected protocol TCP (6), got %d", response.Result.Protocols[0].Protocol)
	}

	if response.Result.Protocols[0].Count != 3 {
		t.Fatalf("expected count 3, got %d", response.Result.Protocols[0].Count)
	}
}

func TestGetFlowReportStats_FilterByProtocol(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, dbInstance, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	createFlowReportTestSubscriber(t, dbInstance)

	now := time.Now().UTC().Format(time.RFC3339)

	// Insert 4 TCP flows
	for i := range 4 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "001010100000001",
			SourceIP:        fmt.Sprintf("10.0.0.%d", i+1),
			DestinationIP:   "8.8.8.8",
			SourcePort:      uint16(10000 + i),
			DestinationPort: 443,
			Protocol:        6,
			Packets:         100,
			Bytes:           5000,
			StartTime:       now,
			EndTime:         now,
		}
		if err := dbInstance.InsertFlowReport(context.Background(), fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	// Insert 2 UDP flows
	for i := range 2 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "001010100000001",
			SourceIP:        fmt.Sprintf("192.168.0.%d", i+1),
			DestinationIP:   "1.1.1.1",
			SourcePort:      uint16(20000 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         50,
			Bytes:           2500,
			StartTime:       now,
			EndTime:         now,
		}
		if err := dbInstance.InsertFlowReport(context.Background(), fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	// Filter for TCP only
	statusCode, response, err := getFlowReportStats(ts.URL, client, token, map[string]string{"protocol": "6"})
	if err != nil {
		t.Fatalf("couldn't get flow report stats: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Result == nil {
		t.Fatalf("expected result to not be nil")
	}

	if len(response.Result.Protocols) != 1 {
		t.Fatalf("expected 1 protocol entry, got %d", len(response.Result.Protocols))
	}

	if response.Result.Protocols[0].Protocol != 6 {
		t.Fatalf("expected protocol TCP (6), got %d", response.Result.Protocols[0].Protocol)
	}

	if response.Result.Protocols[0].Count != 4 {
		t.Fatalf("expected count 4, got %d", response.Result.Protocols[0].Count)
	}
}

func TestGetFlowReportStats_Unauthenticated(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	// Initialize so the server is ready, but don't use the token
	_, err = initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize server: %s", err)
	}

	statusCode, _, err := getFlowReportStats(ts.URL, client, "", nil)
	if err != nil {
		t.Fatalf("couldn't call flow report stats endpoint: %s", err)
	}

	if statusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
	}
}
