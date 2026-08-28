// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
)

type ForgetRadioResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func listRadiosWithStatus(url string, client *http.Client, token string, status string) (int, *ListRadiosResponse, error) {
	return listRadiosQuery(url, client, token, fmt.Sprintf("page=1&per_page=25&status=%s", status))
}

func listRadiosPage(url string, client *http.Client, token string, page, perPage int) (int, *ListRadiosResponse, error) {
	return listRadiosQuery(url, client, token, fmt.Sprintf("page=%d&per_page=%d", page, perPage))
}

func listRadiosQuery(url string, client *http.Client, token string, query string) (int, *ListRadiosResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/ran/radios?%s", url, query), nil)
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

	var response ListRadiosResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func getRadioDetail(url string, client *http.Client, token string, nodeType, id string) (int, *GetRadioResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/ran/radios/%s/%s", url, nodeType, id), nil)
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

	var response GetRadioResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func forgetRadio(url string, client *http.Client, token string, nodeType, id string) (int, *ForgetRadioResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", fmt.Sprintf("%s/api/v1/ran/radios/%s/%s", url, nodeType, id), nil)
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

	var response ForgetRadioResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func connectAPIRadio(amfInstance *amf.AMF, name string) *amf.Radio {
	radio := &amf.Radio{
		RanID:      &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: name}},
		RanPresent: amf.RanPresentGNbID,
	}
	amfInstance.UpdateRadioName(radio, name)
	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), radio)

	return radio
}

func findRadio(items []Radio, name string) (Radio, bool) {
	for _, r := range items {
		if r.Name == name {
			return r, true
		}
	}

	return Radio{}, false
}

func setupRadioPresenceTest(t *testing.T) (testEnv, *http.Client, string) {
	t.Helper()

	env, err := setupServer(filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	t.Cleanup(env.Server.Close)

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	return env, client, token
}

func TestListRadiosReportsPresence(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	connectAPIRadio(env.AMF, "gnb-online")
	env.AMF.DisconnectRadio(context.Background(), connectAPIRadio(env.AMF, "gnb-offline"))

	statusCode, response, err := listRadiosWithStatus(env.Server.URL, client, token, "")
	if err != nil {
		t.Fatalf("couldn't list radios: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 2 {
		t.Fatalf("expected 2 radios, got %d: %+v", len(response.Result.Items), response.Result.Items)
	}

	onlineRadio, ok := findRadio(response.Result.Items, "gnb-online")
	if !ok {
		t.Fatal("connected radio missing from the list")
	}

	if onlineRadio.Status != "online" {
		t.Errorf("connected radio status = %q, want online", onlineRadio.Status)
	}

	if onlineRadio.DisconnectedAt != "" {
		t.Errorf("connected radio disconnected_at = %q, want empty", onlineRadio.DisconnectedAt)
	}

	offlineRadio, ok := findRadio(response.Result.Items, "gnb-offline")
	if !ok {
		t.Fatal("disconnected radio missing from the list")
	}

	if offlineRadio.Status != "offline" {
		t.Errorf("disconnected radio status = %q, want offline", offlineRadio.Status)
	}

	if offlineRadio.DisconnectedAt == "" {
		t.Error("disconnected radio has an empty disconnected_at")
	}

	if offlineRadio.ID != "gnb-offline" {
		t.Errorf("disconnected radio id = %q, want its last known ID", offlineRadio.ID)
	}
}

func TestListRadiosStatusFilter(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	connectAPIRadio(env.AMF, "gnb-online")
	env.AMF.DisconnectRadio(context.Background(), connectAPIRadio(env.AMF, "gnb-offline"))

	for _, tc := range []struct {
		status string
		want   string
	}{
		{status: "online", want: "gnb-online"},
		{status: "offline", want: "gnb-offline"},
	} {
		statusCode, response, err := listRadiosWithStatus(env.Server.URL, client, token, tc.status)
		if err != nil {
			t.Fatalf("couldn't list %s radios: %s", tc.status, err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("status=%s: expected status %d, got %d", tc.status, http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 1 || response.Result.Items[0].Name != tc.want {
			t.Errorf("status=%s returned %+v, want only %q", tc.status, response.Result.Items, tc.want)
		}

		if response.Result.TotalCount != 1 {
			t.Errorf("status=%s: total_count = %d, want 1", tc.status, response.Result.TotalCount)
		}
	}
}

func TestListRadiosRejectsUnknownStatusFilter(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	statusCode, _, err := listRadiosWithStatus(env.Server.URL, client, token, "asleep")
	if err != nil {
		t.Fatalf("couldn't list radios: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Errorf("expected status %d for an unknown status filter, got %d", http.StatusBadRequest, statusCode)
	}
}

func TestGetRadioReportsPresence(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	env.AMF.DisconnectRadio(context.Background(), connectAPIRadio(env.AMF, "gnb-offline"))

	statusCode, response, err := getRadioDetail(env.Server.URL, client, token, "gNB", "gnb-offline")
	if err != nil {
		t.Fatalf("couldn't get radio: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Result.Status != "offline" {
		t.Errorf("status = %q, want offline", response.Result.Status)
	}

	if response.Result.DisconnectedAt == "" {
		t.Error("disconnected_at is empty on an offline radio")
	}
}

func TestSubscribersByOfflineRadio(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	env.AMF.DisconnectRadio(context.Background(), connectAPIRadio(env.AMF, "gnb-offline"))

	req, err := http.NewRequestWithContext(context.Background(), "GET",
		env.Server.URL+"/api/v1/subscribers?radio=gnb-offline", nil)
	if err != nil {
		t.Fatalf("couldn't build request: %s", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("couldn't list subscribers: %s", err)
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d for a known offline radio, got %d", http.StatusOK, res.StatusCode)
	}
}

func TestListRadiosPaginationIsStable(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	for i := range 6 {
		radio := connectAPIRadio(env.AMF, fmt.Sprintf("gnb-%d", i))
		if i%2 == 0 {
			env.AMF.DisconnectRadio(context.Background(), radio)
		}
	}

	var first []string

	for attempt := range 8 {
		var got []string

		for page := 1; page <= 3; page++ {
			_, response, err := listRadiosPage(env.Server.URL, client, token, page, 2)
			if err != nil {
				t.Fatalf("couldn't list radios: %s", err)
			}

			for _, radio := range response.Result.Items {
				got = append(got, radio.ID)
			}
		}

		if attempt == 0 {
			first = got

			if len(slices.Compact(slices.Sorted(slices.Values(got)))) != 6 {
				t.Fatalf("paging returned %v, want each of the 6 radios once", got)
			}

			continue
		}

		if !slices.Equal(got, first) {
			t.Fatalf("paging returned %v, want the same order as %v", got, first)
		}
	}
}

func TestForgetRadio(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	env.AMF.DisconnectRadio(context.Background(), connectAPIRadio(env.AMF, "gnb-offline"))

	statusCode, response, err := forgetRadio(env.Server.URL, client, token, "gNB", "gnb-offline")
	if err != nil {
		t.Fatalf("couldn't forget radio: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%q)", http.StatusOK, statusCode, response.Error)
	}

	_, listResponse, err := listRadiosWithStatus(env.Server.URL, client, token, "")
	if err != nil {
		t.Fatalf("couldn't list radios: %s", err)
	}

	if len(listResponse.Result.Items) != 0 {
		t.Errorf("expected the forgotten radio to be gone, got %+v", listResponse.Result.Items)
	}
}

func TestForgetRadioNotFound(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	statusCode, _, err := forgetRadio(env.Server.URL, client, token, "gNB", "gnb-nope")
	if err != nil {
		t.Fatalf("couldn't forget radio: %s", err)
	}

	if statusCode != http.StatusNotFound {
		t.Errorf("expected status %d for an unknown radio, got %d", http.StatusNotFound, statusCode)
	}
}

func TestForgetRadioWrongNodeType(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	env.AMF.DisconnectRadio(context.Background(), connectAPIRadio(env.AMF, "gnb-offline"))

	statusCode, _, err := forgetRadio(env.Server.URL, client, token, "eNB", "gnb-offline")
	if err != nil {
		t.Fatalf("couldn't forget radio: %s", err)
	}

	if statusCode != http.StatusNotFound {
		t.Errorf("expected status %d addressing a gNB as an eNB, got %d", http.StatusNotFound, statusCode)
	}

	_, listResponse, err := listRadiosWithStatus(env.Server.URL, client, token, "")
	if err != nil {
		t.Fatalf("couldn't list radios: %s", err)
	}

	if len(listResponse.Result.Items) != 1 {
		t.Errorf("expected the radio to survive a mistyped forget, got %+v", listResponse.Result.Items)
	}
}

func TestForgetRadioNodeTypeIsCaseInsensitive(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	env.AMF.DisconnectRadio(context.Background(), connectAPIRadio(env.AMF, "gnb-offline"))

	statusCode, response, err := forgetRadio(env.Server.URL, client, token, "gnb", "gnb-offline")
	if err != nil {
		t.Fatalf("couldn't forget radio: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%q)", http.StatusOK, statusCode, response.Error)
	}
}

func TestForgetRadioOnlineConflicts(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	connectAPIRadio(env.AMF, "gnb-online")

	statusCode, _, err := forgetRadio(env.Server.URL, client, token, "gNB", "gnb-online")
	if err != nil {
		t.Fatalf("couldn't forget radio: %s", err)
	}

	if statusCode != http.StatusConflict {
		t.Errorf("expected status %d for a connected radio, got %d", http.StatusConflict, statusCode)
	}

	_, listResponse, err := listRadiosWithStatus(env.Server.URL, client, token, "")
	if err != nil {
		t.Fatalf("couldn't list radios: %s", err)
	}

	if _, ok := findRadio(listResponse.Result.Items, "gnb-online"); !ok {
		t.Error("the connected radio was dropped by a refused forget")
	}
}

func TestForgetRadioIsAdminOnly(t *testing.T) {
	env, client, adminToken := setupRadioPresenceTest(t)

	for _, tc := range []struct {
		name   string
		email  string
		roleID RoleID
	}{
		{name: "read only", email: "readonly@ellanetworks.com", roleID: RoleReadOnly},
		{name: "network manager", email: "networkmanager@ellanetworks.com", roleID: RoleNetworkManager},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env.AMF.DisconnectRadio(context.Background(), connectAPIRadio(env.AMF, "gnb-offline"))

			roleClient := newTestClient(env.Server)

			roleToken, err := createUserAndLogin(env.Server.URL, adminToken, tc.email, tc.roleID, roleClient)
			if err != nil {
				t.Fatalf("couldn't create %s user: %s", tc.name, err)
			}

			statusCode, _, err := forgetRadio(env.Server.URL, roleClient, roleToken, "gNB", "gnb-offline")
			if err != nil {
				t.Fatalf("couldn't forget radio: %s", err)
			}

			if statusCode != http.StatusForbidden {
				t.Errorf("expected status %d for a %s user, got %d", http.StatusForbidden, tc.name, statusCode)
			}
		})
	}

	statusCode, _, err := forgetRadio(env.Server.URL, client, adminToken, "gNB", "gnb-offline")
	if err != nil {
		t.Fatalf("couldn't forget radio as admin: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("expected status %d for an admin, got %d", http.StatusOK, statusCode)
	}
}
