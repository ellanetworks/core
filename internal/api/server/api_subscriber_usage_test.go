// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

type DailySubscriberUsage struct {
	Date          string `json:"date"`
	UplinkBytes   int64  `json:"uplink_bytes"`
	DownlinkBytes int64  `json:"downlink_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type PerSubscriberUsage struct {
	IMSI          string `json:"imsi"`
	UplinkBytes   int64  `json:"uplink_bytes"`
	DownlinkBytes int64  `json:"downlink_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type GetUsagePerDayResponse struct {
	Result []DailySubscriberUsage `json:"result,omitempty"`
	Error  string                 `json:"error,omitempty"`
}

type GetUsagePerSubscriberResponse struct {
	Result []PerSubscriberUsage `json:"result,omitempty"`
	Error  string               `json:"error,omitempty"`
}

type GetSubscriberUsagesRetentionPolicyResponseResult struct {
	Days int `json:"days"`
}

type UpdateSubscriberUsagePolicyResponseResult struct {
	Message string `json:"message"`
}

type DeleteSubscriberUsageResponseResult struct {
	Message string `json:"message"`
}

type DeleteSubscriberUsageResponse struct {
	Result DeleteSubscriberUsageResponseResult `json:"result"`
	Error  string                              `json:"error,omitempty"`
}

type GroupBy string

const (
	GroupByDay        GroupBy = "day"
	GroupBySubscriber GroupBy = "subscriber"
)

func usageForDate(result []DailySubscriberUsage, date string) (DailySubscriberUsage, bool) {
	for _, entry := range result {
		if entry.Date == date {
			return entry, true
		}
	}

	return DailySubscriberUsage{}, false
}

func usageQuery(startDate string, endDate string, subscriber string, groupBy GroupBy) string {
	var queryParams []string

	queryParams = append(queryParams, fmt.Sprintf("start=%s", startDate))
	queryParams = append(queryParams, fmt.Sprintf("end=%s", endDate))
	queryParams = append(queryParams, fmt.Sprintf("group_by=%s", groupBy))

	if subscriber != "" {
		queryParams = append(queryParams, fmt.Sprintf("subscriber=%s", subscriber))
	}

	return strings.Join(queryParams, "&")
}

func getUsagePerDay(url string, client *http.Client, token string, startDate string, endDate string, subscriber string) (int, *GetUsagePerDayResponse, error) {
	return apiDo[GetUsagePerDayResponse](client, "GET", fmt.Sprintf("%s/api/v1/subscriber-usage?%s", url, usageQuery(startDate, endDate, subscriber, GroupByDay)), token, nil)
}

func getUsagePerSubscriber(url string, client *http.Client, token string, startDate string, endDate string, subscriber string) (int, *GetUsagePerSubscriberResponse, error) {
	return apiDo[GetUsagePerSubscriberResponse](client, "GET", fmt.Sprintf("%s/api/v1/subscriber-usage?%s", url, usageQuery(startDate, endDate, subscriber, GroupBySubscriber)), token, nil)
}

func clearSubscriberUsage(url string, client *http.Client, token string) (int, *DeleteSubscriberUsageResponse, error) {
	return apiDo[DeleteSubscriberUsageResponse](client, "DELETE", fmt.Sprintf("%s/api/v1/subscriber-usage", url), token, nil)
}

func createDataNetworkAndPolicy(url string, client *http.Client, token string) error {
	createDataNetworkParams := &CreateDataNetworkParams{
		Name:     DataNetworkName,
		MTU:      MTU,
		IPv4Pool: IPv4Pool,
		DNS:      DNS,
	}

	_, _, err := createDataNetwork(url, client, token, createDataNetworkParams)
	if err != nil {
		return err
	}

	createProfileParams := &CreateProfileParams{
		Name:           TestProfileName,
		UeAmbrUplink:   "200 Mbps",
		UeAmbrDownlink: "200 Mbps",
	}

	_, _, err = createProfile(url, client, token, createProfileParams)
	if err != nil {
		return err
	}

	createPolicyParams := &CreatePolicyParams{
		Name:                PolicyName,
		ProfileName:         TestProfileName,
		SliceName:           DefaultSliceName,
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkName:     DataNetworkName,
	}

	_, _, err = createPolicy(url, client, token, createPolicyParams)
	if err != nil {
		return err
	}

	return nil
}

func TestAPISubscriberUsagePerDayEndToEnd(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	imsi1 := "001010100007487"
	imsi2 := "001010100007488"

	t.Run("1. Get subscriber usage per day - no usage", func(t *testing.T) {
		statusCode, response, err := getUsagePerDay(env.Server.URL, client, token, "", "", "")
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per day: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		for _, entry := range response.Result {
			if entry.TotalBytes != 0 {
				t.Fatalf("expected no usage on %s, got %d bytes", entry.Date, entry.TotalBytes)
			}
		}
	})

	t.Run("2. Create Data Network, Policy, and Subscribers", func(t *testing.T) {
		err := createDataNetworkAndPolicy(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't create data network and policy: %s", err)
		}

		createSubscriberParams := &CreateSubscriberParams{
			Imsi:           imsi1,
			Key:            Key,
			Opc:            Opc,
			SequenceNumber: SequenceNumber,
			ProfileName:    TestProfileName,
		}

		_, _, err = createSubscriber(env.Server.URL, client, token, createSubscriberParams)
		if err != nil {
			t.Fatalf("couldn't create subscriber: %s", err)
		}

		createSubscriberParams = &CreateSubscriberParams{
			Imsi:           imsi2,
			Key:            Key,
			Opc:            Opc,
			SequenceNumber: SequenceNumber,
			ProfileName:    TestProfileName,
		}

		_, _, err = createSubscriber(env.Server.URL, client, token, createSubscriberParams)
		if err != nil {
			t.Fatalf("couldn't create subscriber: %s", err)
		}
	})

	t.Run("2. Add subscriber usage (directly through database)", func(t *testing.T) {
		date1 := "2025-11-14"
		date2 := "2025-11-19"

		date1Parsed, err := time.Parse("2006-01-02", date1)
		if err != nil {
			t.Fatalf("couldn't parse date %s: %s", date1, err)
		}

		date2Parsed, err := time.Parse("2006-01-02", date2)
		if err != nil {
			t.Fatalf("couldn't parse date %s: %s", date2, err)
		}

		err = env.DB.IncrementDailyUsage(context.Background(), db.DailyUsage{
			EpochDay:      db.DaysSinceEpoch(date1Parsed),
			IMSI:          imsi1,
			BytesUplink:   1500,
			BytesDownlink: 2500,
		})
		if err != nil {
			t.Fatalf("couldn't increment daily usage: %s", err)
		}

		err = env.DB.IncrementDailyUsage(context.Background(), db.DailyUsage{
			EpochDay:      db.DaysSinceEpoch(date2Parsed),
			IMSI:          imsi2,
			BytesUplink:   1222,
			BytesDownlink: 23222,
		})
		if err != nil {
			t.Fatalf("couldn't increment daily usage: %s", err)
		}
	})

	t.Run("3. Get subscriber usage per day", func(t *testing.T) {
		statusCode, response, err := getUsagePerDay(env.Server.URL, client, token, "2025-11-14T00:00:00Z", "2025-11-20T00:00:00Z", "")
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per day: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if len(response.Result) != 6 {
			t.Fatalf("expected 6 usage data entries, got %d entries", len(response.Result))
		}

		expectedDate1Key := "2025-11-14"
		if _, ok := usageForDate(response.Result, expectedDate1Key); !ok {
			t.Fatalf("expected an entry for date key %s, got %v", expectedDate1Key, response.Result)
		}

		expectedDate2Key := "2025-11-19"
		if _, ok := usageForDate(response.Result, expectedDate2Key); !ok {
			t.Fatalf("expected an entry for date key %s, got %v", expectedDate2Key, response.Result)
		}

		if day1, _ := usageForDate(response.Result, expectedDate1Key); day1.UplinkBytes != 1500 || day1.DownlinkBytes != 2500 || day1.TotalBytes != 4000 {
			t.Fatalf("unexpected usage data for date %s: %+v", expectedDate1Key, day1)
		}

		if day2, _ := usageForDate(response.Result, expectedDate2Key); day2.UplinkBytes != 1222 || day2.DownlinkBytes != 23222 || day2.TotalBytes != 24444 {
			t.Fatalf("unexpected usage data for date %s: %+v", expectedDate2Key, day2)
		}

		if idle, _ := usageForDate(response.Result, "2025-11-16"); idle.TotalBytes != 0 {
			t.Fatalf("expected a zero-filled entry for an idle day, got %+v", idle)
		}
	})

	t.Run("4. Get subscriber usage per day - subscriber filter", func(t *testing.T) {
		statusCode, response, err := getUsagePerDay(env.Server.URL, client, token, "2025-11-14T00:00:00Z", "2025-11-20T00:00:00Z", imsi2)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per day: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if len(response.Result) != 6 {
			t.Fatalf("expected 6 usage data entries, got %d entries", len(response.Result))
		}

		expectedDateKey := "2025-11-19"

		day, ok := usageForDate(response.Result, expectedDateKey)
		if !ok {
			t.Fatalf("expected an entry for date key %s, got %v", expectedDateKey, response.Result)
		}

		if day.UplinkBytes != 1222 || day.DownlinkBytes != 23222 || day.TotalBytes != 24444 {
			t.Fatalf("unexpected usage data for date %s: %+v", expectedDateKey, day)
		}

		if other, _ := usageForDate(response.Result, "2025-11-14"); other.TotalBytes != 0 {
			t.Fatalf("expected no usage for another subscriber's day, got %+v", other)
		}
	})

	t.Run("5. Reject an over-long day range", func(t *testing.T) {
		statusCode, response, err := getUsagePerDay(env.Server.URL, client, token, "0001-01-01T00:00:00Z", "", "")
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per day: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error == "" {
			t.Fatal("expected an error message, got none")
		}
	})

	t.Run("6. Clear subscriber usage data", func(t *testing.T) {
		statusCode, response, err := clearSubscriberUsage(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't clear subscriber usage data: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "All subscriber usage cleared successfully" {
			t.Fatalf("expected success message, got %s", response.Result.Message)
		}
	})

	t.Run("7. Verify cleared subscriber usage data", func(t *testing.T) {
		statusCode, response, err := getUsagePerDay(env.Server.URL, client, token, "", "", "")
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per day: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		for _, entry := range response.Result {
			if entry.TotalBytes != 0 {
				t.Fatalf("expected no usage on %s after clearing, got %d bytes", entry.Date, entry.TotalBytes)
			}
		}
	})
}

func TestAPISubscriberUsagePerSubscriberEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	imsi1 := "001010100007487"
	imsi2 := "001010100007488"

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Get subscriber usage per day - no usage", func(t *testing.T) {
		statusCode, response, err := getUsagePerSubscriber(env.Server.URL, client, token, "", "", "")
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per subscriber: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if len(response.Result) != 0 {
			t.Fatalf("expected no usage data, got %d entries", len(response.Result))
		}
	})

	t.Run("2. Create Data Network, Policy, and Subscribers", func(t *testing.T) {
		err := createDataNetworkAndPolicy(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't create data network and policy: %s", err)
		}

		createSubscriberParams := &CreateSubscriberParams{
			Imsi:           imsi1,
			Key:            Key,
			Opc:            Opc,
			SequenceNumber: SequenceNumber,
			ProfileName:    TestProfileName,
		}

		_, _, err = createSubscriber(env.Server.URL, client, token, createSubscriberParams)
		if err != nil {
			t.Fatalf("couldn't create subscriber: %s", err)
		}

		createSubscriberParams = &CreateSubscriberParams{
			Imsi:           imsi2,
			Key:            Key,
			Opc:            Opc,
			SequenceNumber: SequenceNumber,
			ProfileName:    TestProfileName,
		}

		_, _, err = createSubscriber(env.Server.URL, client, token, createSubscriberParams)
		if err != nil {
			t.Fatalf("couldn't create subscriber: %s", err)
		}
	})

	t.Run("2. Add subscriber usage (directly through database)", func(t *testing.T) {
		date1 := "2025-11-14"
		date2 := "2025-11-19"

		date1Parsed, err := time.Parse("2006-01-02", date1)
		if err != nil {
			t.Fatalf("couldn't parse date %s: %s", date1, err)
		}

		date2Parsed, err := time.Parse("2006-01-02", date2)
		if err != nil {
			t.Fatalf("couldn't parse date %s: %s", date2, err)
		}

		err = env.DB.IncrementDailyUsage(context.Background(), db.DailyUsage{
			EpochDay:      db.DaysSinceEpoch(date1Parsed),
			IMSI:          imsi1,
			BytesUplink:   1500,
			BytesDownlink: 2500,
		})
		if err != nil {
			t.Fatalf("couldn't increment daily usage: %s", err)
		}

		err = env.DB.IncrementDailyUsage(context.Background(), db.DailyUsage{
			EpochDay:      db.DaysSinceEpoch(date2Parsed),
			IMSI:          imsi2,
			BytesUplink:   1222,
			BytesDownlink: 23222,
		})
		if err != nil {
			t.Fatalf("couldn't increment daily usage: %s", err)
		}
	})

	t.Run("3. Get subscriber usage per day", func(t *testing.T) {
		statusCode, response, err := getUsagePerSubscriber(env.Server.URL, client, token, "2025-11-14T00:00:00Z", "2025-11-20T00:00:00Z", "")
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per day: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if len(response.Result) != 2 {
			t.Fatalf("expected 2 usage data entries, got %d entries", len(response.Result))
		}

		want := []PerSubscriberUsage{
			{IMSI: imsi2, UplinkBytes: 1222, DownlinkBytes: 23222, TotalBytes: 24444},
			{IMSI: imsi1, UplinkBytes: 1500, DownlinkBytes: 2500, TotalBytes: 4000},
		}

		for i, w := range want {
			if response.Result[i] != w {
				t.Fatalf("entry %d: got %+v, want %+v", i, response.Result[i], w)
			}
		}
	})

	t.Run("4. Get subscriber usage per subscriber - subscriber filter", func(t *testing.T) {
		statusCode, response, err := getUsagePerSubscriber(env.Server.URL, client, token, "2025-11-14T00:00:00Z", "2025-11-20T00:00:00Z", imsi2)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per subscriber: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if len(response.Result) != 1 {
			t.Fatalf("expected 1 usage data entries, got %d entries", len(response.Result))
		}

		want := PerSubscriberUsage{IMSI: imsi2, UplinkBytes: 1222, DownlinkBytes: 23222, TotalBytes: 24444}

		if response.Result[0] != want {
			t.Fatalf("got %+v, want %+v", response.Result[0], want)
		}
	})

	t.Run("5. Clear subscriber usage data", func(t *testing.T) {
		statusCode, response, err := clearSubscriberUsage(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't clear subscriber usage data: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "All subscriber usage cleared successfully" {
			t.Fatalf("expected success message, got %s", response.Result.Message)
		}
	})

	t.Run("6. Verify cleared subscriber usage data", func(t *testing.T) {
		statusCode, response, err := getUsagePerSubscriber(env.Server.URL, client, token, "", "", "")
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per subscriber: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		for _, entry := range response.Result {
			if entry.TotalBytes != 0 {
				t.Fatalf("expected no usage for %s after clearing, got %d bytes", entry.IMSI, entry.TotalBytes)
			}
		}
	})
}
