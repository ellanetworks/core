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
)

type SubscriberUsage struct {
	UplinkBytes   int64 `json:"uplink_bytes"`
	DownlinkBytes int64 `json:"downlink_bytes"`
	TotalBytes    int64 `json:"total_bytes"`
}

type GetSubscriberUsageResponse struct {
	Result []map[string]SubscriberUsage `json:"result,omitempty"`
	Error  string                       `json:"error,omitempty"`
}

type GetSubscriberUsagesRetentionPolicyResponseResult struct {
	Days int `json:"days"`
}

type GetSubscriberUsageRetentionPolicyResponse struct {
	Result *GetSubscriberUsagesRetentionPolicyResponseResult `json:"result,omitempty"`
	Error  string                                            `json:"error,omitempty"`
}

type UpdateSubscriberUsagePolicyResponseResult struct {
	Message string `json:"message"`
}

type UpdateSubscriberUsageRetentionPolicyResponse struct {
	Result *UpdateSubscriberUsagePolicyResponseResult `json:"result,omitempty"`
	Error  string                                     `json:"error,omitempty"`
}

type UpdateSubscriberUsageRetentionPolicyParams struct {
	Days int `json:"days"`
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
	GroupBySubscriber         = "subscriber"
)

func getSubscriberUsage(url string, client *http.Client, token string, startDate string, endDate string, subscriber string, groupBy GroupBy) (int, *GetSubscriberUsageResponse, error) {
	var queryParams []string

	queryParams = append(queryParams, fmt.Sprintf("start=%s", startDate))
	queryParams = append(queryParams, fmt.Sprintf("end=%s", endDate))
	queryParams = append(queryParams, fmt.Sprintf("group_by=%s", groupBy))

	if subscriber != "" {
		queryParams = append(queryParams, fmt.Sprintf("subscriber=%s", subscriber))
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/subscriber-usage?%s", url, strings.Join(queryParams, "&")), nil)
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

	var subscriberUsageResponse GetSubscriberUsageResponse

	if err := json.NewDecoder(res.Body).Decode(&subscriberUsageResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &subscriberUsageResponse, nil
}

func clearSubscriberUsage(url string, client *http.Client, token string) (int, *DeleteSubscriberUsageResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", fmt.Sprintf("%s/api/v1/subscriber-usage", url), nil)
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

	var subscriberUsageResponse DeleteSubscriberUsageResponse

	if err := json.NewDecoder(res.Body).Decode(&subscriberUsageResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &subscriberUsageResponse, nil
}

func getSubscriberUsageRetentionPolicy(url string, client *http.Client, token string) (int, *GetSubscriberUsageRetentionPolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/subscriber-usage/retention", nil)
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

	var retentionPolicyResponse GetSubscriberUsageRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&retentionPolicyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &retentionPolicyResponse, nil
}

func editSubscriberUsageRetentionPolicy(url string, client *http.Client, token string, data *UpdateSubscriberUsageRetentionPolicyParams) (int, *UpdateSubscriberUsageRetentionPolicyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/subscriber-usage/retention", strings.NewReader(string(body)))
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
	var updateResponse UpdateSubscriberUsageRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func TestAPISubscriberUsagePerDayEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, database, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Get subscriber usage per day - no usage", func(t *testing.T) {
		statusCode, response, err := getSubscriberUsage(ts.URL, client, token, "", "", "", GroupByDay)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per day: %s", err)
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

		err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
			EpochDay:      db.DaysSinceEpoch(date1Parsed),
			IMSI:          "test_imsi_1",
			BytesUplink:   1500,
			BytesDownlink: 2500,
		})
		if err != nil {
			t.Fatalf("couldn't increment daily usage: %s", err)
		}

		err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
			EpochDay:      db.DaysSinceEpoch(date2Parsed),
			IMSI:          "test_imsi_2",
			BytesUplink:   1222,
			BytesDownlink: 23222,
		})
		if err != nil {
			t.Fatalf("couldn't increment daily usage: %s", err)
		}
	})

	t.Run("3. Get subscriber usage per day", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -14).Format("2006-01-02")
		endDate := time.Now().Format("2006-01-02")
		statusCode, response, err := getSubscriberUsage(ts.URL, client, token, startDate, endDate, "", GroupByDay)
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

		expectedDate1Key := "2025-11-14"
		if _, ok := response.Result[0][expectedDate1Key]; !ok {
			t.Fatalf("expected first entry to have date key %s, got %v", expectedDate1Key, response.Result[0])
		}

		expectedDate2Key := "2025-11-19"
		if _, ok := response.Result[1][expectedDate2Key]; !ok {
			t.Fatalf("expected second entry to have date key %s, got %v", expectedDate2Key, response.Result[1])
		}

		if response.Result[0][expectedDate1Key].UplinkBytes != 1500 || response.Result[0][expectedDate1Key].DownlinkBytes != 2500 || response.Result[0][expectedDate1Key].TotalBytes != 4000 {
			t.Fatalf("unexpected usage data for date %s: %+v", expectedDate1Key, response.Result[0][expectedDate1Key])
		}

		if response.Result[1][expectedDate2Key].UplinkBytes != 1222 || response.Result[1][expectedDate2Key].DownlinkBytes != 23222 || response.Result[1][expectedDate2Key].TotalBytes != 24444 {
			t.Fatalf("unexpected usage data for date %s: %+v", expectedDate2Key, response.Result[1][expectedDate2Key])
		}
	})

	t.Run("4. Get subscriber usage per day - subscriber filter", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -14).Format("2006-01-02")
		endDate := time.Now().Format("2006-01-02")
		statusCode, response, err := getSubscriberUsage(ts.URL, client, token, startDate, endDate, "test_imsi_2", GroupByDay)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per day: %s", err)
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

		expectedDateKey := "2025-11-19"
		if _, ok := response.Result[0][expectedDateKey]; !ok {
			t.Fatalf("expected first entry to have date key %s, got %v", expectedDateKey, response.Result[0])
		}

		if response.Result[0][expectedDateKey].UplinkBytes != 1222 || response.Result[0][expectedDateKey].DownlinkBytes != 23222 || response.Result[0][expectedDateKey].TotalBytes != 24444 {
			t.Fatalf("unexpected usage data for date %s: %+v", expectedDateKey, response.Result[0][expectedDateKey])
		}
	})

	t.Run("5. Clear subscriber usage data", func(t *testing.T) {
		statusCode, response, err := clearSubscriberUsage(ts.URL, client, token)
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
		statusCode, response, err := getSubscriberUsage(ts.URL, client, token, "", "", "", GroupByDay)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage per day: %s", err)
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
}

func TestAPISubscriberUsagePerSubscriberEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, database, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Get subscriber usage per day - no usage", func(t *testing.T) {
		statusCode, response, err := getSubscriberUsage(ts.URL, client, token, "", "", "", GroupBySubscriber)
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

		err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
			EpochDay:      db.DaysSinceEpoch(date1Parsed),
			IMSI:          "test_imsi_1",
			BytesUplink:   1500,
			BytesDownlink: 2500,
		})
		if err != nil {
			t.Fatalf("couldn't increment daily usage: %s", err)
		}

		err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
			EpochDay:      db.DaysSinceEpoch(date2Parsed),
			IMSI:          "test_imsi_2",
			BytesUplink:   1222,
			BytesDownlink: 23222,
		})
		if err != nil {
			t.Fatalf("couldn't increment daily usage: %s", err)
		}
	})

	t.Run("3. Get subscriber usage per day", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -14).Format("2006-01-02")
		endDate := time.Now().Format("2006-01-02")
		statusCode, response, err := getSubscriberUsage(ts.URL, client, token, startDate, endDate, "", GroupBySubscriber)
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

		expectedDate1Key := "test_imsi_2"
		if _, ok := response.Result[0][expectedDate1Key]; !ok {
			t.Fatalf("expected first entry to have date key %s, got %v", expectedDate1Key, response.Result[0])
		}

		expectedDate2Key := "test_imsi_1"
		if _, ok := response.Result[1][expectedDate2Key]; !ok {
			t.Fatalf("expected second entry to have date key %s, got %v", expectedDate2Key, response.Result[1])
		}

		if response.Result[0][expectedDate1Key].UplinkBytes != 1222 || response.Result[0][expectedDate1Key].DownlinkBytes != 23222 || response.Result[0][expectedDate1Key].TotalBytes != 24444 {
			t.Fatalf("unexpected usage data for date %s: %+v", expectedDate1Key, response.Result[0][expectedDate1Key])
		}

		if response.Result[1][expectedDate2Key].UplinkBytes != 1500 || response.Result[1][expectedDate2Key].DownlinkBytes != 2500 || response.Result[1][expectedDate2Key].TotalBytes != 4000 {
			t.Fatalf("unexpected usage data for date %s: %+v", expectedDate2Key, response.Result[1][expectedDate2Key])
		}
	})

	t.Run("4. Get subscriber usage per subscriber - subscriber filter", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -14).Format("2006-01-02")
		endDate := time.Now().Format("2006-01-02")
		statusCode, response, err := getSubscriberUsage(ts.URL, client, token, startDate, endDate, "test_imsi_2", GroupBySubscriber)
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

		expectedDateKey := "test_imsi_2"
		if _, ok := response.Result[0][expectedDateKey]; !ok {
			t.Fatalf("expected first entry to have date key %s, got %v", expectedDateKey, response.Result[0])
		}

		if response.Result[0][expectedDateKey].UplinkBytes != 1222 || response.Result[0][expectedDateKey].DownlinkBytes != 23222 || response.Result[0][expectedDateKey].TotalBytes != 24444 {
			t.Fatalf("unexpected usage data for date %s: %+v", expectedDateKey, response.Result[0][expectedDateKey])
		}
	})

	t.Run("5. Clear subscriber usage data", func(t *testing.T) {
		statusCode, response, err := clearSubscriberUsage(ts.URL, client, token)
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
		statusCode, response, err := getSubscriberUsage(ts.URL, client, token, "", "", "", GroupBySubscriber)
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
}

func TestAPISubscriberUsageRetentionPolicyEndToEnd(t *testing.T) {
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

	t.Run("1. Get subscriber usage retention policy", func(t *testing.T) {
		statusCode, response, err := getSubscriberUsageRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 365 {
			t.Fatalf("expected default subscriber usage retention policy to be 365 days, got %d", response.Result.Days)
		}
	})

	t.Run("2. Update subscriber usage retention policy", func(t *testing.T) {
		updateSubscriberUsageRetentionPolicyParams := &UpdateSubscriberUsageRetentionPolicyParams{
			Days: 15,
		}
		statusCode, response, err := editSubscriberUsageRetentionPolicy(ts.URL, client, token, updateSubscriberUsageRetentionPolicyParams)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "Subscriber usage retention policy updated successfully" {
			t.Fatalf("expected success message, got %s", response.Result.Message)
		}
	})

	t.Run("3. Verify updated subscriber usage retention policy", func(t *testing.T) {
		statusCode, response, err := getSubscriberUsageRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 15 {
			t.Fatalf("expected updated subscriber usage retention policy to be 15 days, got %d", response.Result.Days)
		}
	})
}

func TestUpdateSubscriberUsageRetentionPolicyInvalidInput(t *testing.T) {
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
		testName string
		days     int
		error    string
	}{
		{
			testName: "Negative days",
			days:     -1,
			error:    "retention days must be greater than 0",
		},
		{
			testName: "0 days",
			days:     0,
			error:    "retention days must be greater than 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			updateParams := &UpdateSubscriberUsageRetentionPolicyParams{
				Days: tt.days,
			}
			statusCode, response, err := editSubscriberUsageRetentionPolicy(ts.URL, client, token, updateParams)
			if err != nil {
				t.Fatalf("couldn't edit subscriber usage retention policy: %s", err)
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
