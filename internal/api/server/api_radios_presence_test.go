// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
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
	return apiDo[ListRadiosResponse](client, "GET", fmt.Sprintf("%s/api/v1/ran/radios?%s", url, query), token, nil)
}

const testRadioBitLength = 24

func radioRefFor(nodeType, id string) string {
	if strings.EqualFold(nodeType, "gNB") {
		return fmt.Sprintf("%s:001-01:%s@%d", nodeType, id, testRadioBitLength)
	}

	return fmt.Sprintf("%s:001-01:%s", nodeType, id)
}

func getRadioDetail(url string, client *http.Client, token string, nodeType, id string) (int, *GetRadioResponse, error) {
	return apiDo[GetRadioResponse](client, "GET", fmt.Sprintf("%s/api/v1/ran/radios/%s", url, radioRefFor(nodeType, id)), token, nil)
}

func forgetRadio(url string, client *http.Client, token string, nodeType, id string) (int, *ForgetRadioResponse, error) {
	return apiDo[ForgetRadioResponse](client, "DELETE", fmt.Sprintf("%s/api/v1/ran/radios/%s", url, radioRefFor(nodeType, id)), token, nil)
}

func connectAPIRadio(amfInstance *amf.AMF, name string) *amf.Radio {
	return connectAPIRadioID(amfInstance, name, gnbRanNodeID("001", "01", name, testRadioBitLength))
}

func connectAPIRadioID(amfInstance *amf.AMF, name string, ranID models.GlobalRanNodeID) *amf.Radio {
	radio := &amf.Radio{RanID: &ranID}
	amfInstance.UpdateRadioName(radio, name)
	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), radio)

	return radio
}

func gnbRanNodeID(mcc, mnc, value string, bitLength int32) models.GlobalRanNodeID {
	return models.GlobalRanNodeID{
		PlmnID: &models.PlmnID{Mcc: mcc, Mnc: mnc},
		GNbID:  &models.GNbID{BitLength: bitLength, GNBValue: value},
	}
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

// TS 29.571: a gNB ID is rendered zero-padded to its width, so the same hex at
// two widths — or under two PLMNs — is two radios. Each must be addressable on
// its own.
func TestForgetRadioAddressesOneIdentity(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	for _, radio := range []*amf.Radio{
		connectAPIRadioID(env.AMF, "gnb-22bit", gnbRanNodeID("001", "01", "00002a", 22)),
		connectAPIRadioID(env.AMF, "gnb-24bit", gnbRanNodeID("001", "01", "00002a", 24)),
		connectAPIRadioID(env.AMF, "gnb-visited", gnbRanNodeID("208", "93", "00002a", 24)),
	} {
		env.AMF.DisconnectRadio(context.Background(), radio)
	}

	statusCode, response, err := apiDo[ForgetRadioResponse](client, "DELETE",
		env.Server.URL+"/api/v1/ran/radios/gNB:001-01:00002a@22", token, nil)
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

	var names []string
	for _, radio := range listResponse.Result.Items {
		names = append(names, radio.Name)
	}

	slices.Sort(names)

	if !slices.Equal(names, []string{"gnb-24bit", "gnb-visited"}) {
		t.Errorf("radios after forgetting the 22-bit gNB = %v, want the 24-bit and the visited-PLMN one", names)
	}
}

func TestGetRadioAddressesOneIdentity(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	connectAPIRadioID(env.AMF, "gnb-22bit", gnbRanNodeID("001", "01", "00002a", 22))
	connectAPIRadioID(env.AMF, "gnb-24bit", gnbRanNodeID("001", "01", "00002a", 24))
	connectAPIRadioID(env.AMF, "gnb-visited", gnbRanNodeID("208", "93", "00002a", 24))

	for _, tc := range []struct{ path, want string }{
		{"gNB:001-01:00002a@22", "gnb-22bit"},
		{"gNB:001-01:00002a@24", "gnb-24bit"},
		{"gNB:208-93:00002a@24", "gnb-visited"},
	} {
		statusCode, response, err := apiDo[GetRadioResponse](client, "GET",
			env.Server.URL+"/api/v1/ran/radios/"+tc.path, token, nil)
		if err != nil {
			t.Fatalf("couldn't get radio %s: %s", tc.path, err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("%s: expected status %d, got %d", tc.path, http.StatusOK, statusCode)
		}

		if response.Result.Name != tc.want {
			t.Errorf("%s resolved to %q, want %q", tc.path, response.Result.Name, tc.want)
		}
	}
}

func TestGetRadioRejectsAGNBWithoutABitLength(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	connectAPIRadioID(env.AMF, "gnb-24bit", gnbRanNodeID("001", "01", "00002a", 24))

	statusCode, _, err := apiDo[GetRadioResponse](client, "GET",
		env.Server.URL+"/api/v1/ran/radios/gNB:001-01:00002a", token, nil)
	if err != nil {
		t.Fatalf("couldn't get radio: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Errorf("expected status %d addressing a gNB without its bit length, got %d", http.StatusBadRequest, statusCode)
	}
}

func TestListedRadioRefAddressesTheRadio(t *testing.T) {
	env, client, token := setupRadioPresenceTest(t)

	connectAPIRadio(env.AMF, "gnb-online")

	_, listResponse, err := listRadiosWithStatus(env.Server.URL, client, token, "")
	if err != nil {
		t.Fatalf("couldn't list radios: %s", err)
	}

	if len(listResponse.Result.Items) != 1 {
		t.Fatalf("expected 1 radio, got %+v", listResponse.Result.Items)
	}

	ref := listResponse.Result.Items[0].Ref
	if ref == "" {
		t.Fatal("listed radio carries no ref")
	}

	statusCode, response, err := apiDo[GetRadioResponse](client, "GET",
		env.Server.URL+"/api/v1/ran/radios/"+ref, token, nil)
	if err != nil {
		t.Fatalf("couldn't get radio by its ref: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("ref %q returned status %d, want %d", ref, statusCode, http.StatusOK)
	}

	if response.Result.Ref != ref {
		t.Errorf("detail ref = %q, want the listed %q", response.Result.Ref, ref)
	}
}
