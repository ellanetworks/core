// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

type IMSApplicationServerItem struct {
	Name                    string   `json:"name"`
	ServerName              string   `json:"server_name"`
	PublicServiceIdentities []string `json:"public_service_identities"`
}

type IMSApplicationServerResponse struct {
	Result IMSApplicationServerItem `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type ListIMSApplicationServersResponse struct {
	Result struct {
		Items []IMSApplicationServerItem `json:"items"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type IMSHeaderMatch struct {
	Header  string  `json:"header"`
	Content *string `json:"content,omitempty"`
}

type IMSServicePointTrigger struct {
	ConditionNegated bool            `json:"condition_negated,omitempty"`
	Group            []int32         `json:"group"`
	RequestURI       *string         `json:"request_uri,omitempty"`
	Method           *string         `json:"method,omitempty"`
	SIPHeader        *IMSHeaderMatch `json:"sip_header,omitempty"`
	SessionCase      *string         `json:"session_case,omitempty"`
	RegistrationType []string        `json:"registration_type,omitempty"`
}

type IMSTriggerPoint struct {
	ConditionTypeCNF bool                     `json:"condition_type_cnf"`
	SPT              []IMSServicePointTrigger `json:"spt"`
}

type IMSInitialFilterCriterion struct {
	Priority                int              `json:"priority"`
	ApplicationServerName   string           `json:"application_server_name"`
	TriggerPoint            *IMSTriggerPoint `json:"trigger_point,omitempty"`
	ProfilePart             *string          `json:"profile_part,omitempty"`
	DefaultHandling         *string          `json:"default_handling,omitempty"`
	ServiceInfo             *string          `json:"service_info,omitempty"`
	IncludeRegisterRequest  bool             `json:"include_register_request"`
	IncludeRegisterResponse bool             `json:"include_register_response"`
}

type ProfileIMSFilterCriteriaBody struct {
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type ProfileIMSFilterCriteriaResponse struct {
	Result ProfileIMSFilterCriteriaBody `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

type IMSMessageResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

const imsASPath = "/api/v1/operator/ims/application-servers"

type fakeIMSProfileNotifier struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeIMSProfileNotifier) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, call)
}

func (f *fakeIMSProfileNotifier) take() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	calls := f.calls
	f.calls = nil

	return calls
}

func (f *fakeIMSProfileNotifier) ApplicationServerChanged(_ context.Context, id string) {
	f.record("as:" + id)
}

func (f *fakeIMSProfileNotifier) ProfileChanged(_ context.Context, id string) {
	f.record("profile:" + id)
}

func (f *fakeIMSProfileNotifier) SubscriberChanged(_ context.Context, imsi string) {
	f.record("subscriber:" + imsi)
}

func (f *fakeIMSProfileNotifier) SubscriberIdentitiesChanged(_ context.Context, imsi, previousMSISDN string) {
	f.record("identities:" + imsi + ":" + previousMSISDN)
}

func newIMSServicesTestEnv(t *testing.T) (testEnv, *http.Client, string, *fakeIMSProfileNotifier) {
	t.Helper()

	testdb, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %s", err)
	}

	notifier := &fakeIMSProfileNotifier{}

	env, err := buildTestEnvWith(testdb, nil, notifier)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	t.Cleanup(env.Server.Close)

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	return env, client, token, notifier
}

func createIMSAS(url string, client *http.Client, token string, as IMSApplicationServerItem) (int, *IMSApplicationServerResponse, error) {
	return apiDo[IMSApplicationServerResponse](client, "POST", url+imsASPath, token, as)
}

func mcpttFilterCriteria(as string) []IMSInitialFilterCriterion {
	register, originating, header, icsi := "REGISTER", "ORIGINATING_REGISTERED", "^P-Asserted-Service$", `^urn:urn-7:3gpp-service\.ims\.icsi\.mcptt$`

	return []IMSInitialFilterCriterion{
		{
			Priority:                0,
			ApplicationServerName:   as,
			TriggerPoint:            &IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, Method: &register}}},
			DefaultHandling:         strPtr("SESSION_CONTINUED"),
			IncludeRegisterRequest:  true,
			IncludeRegisterResponse: true,
		},
		{
			Priority:              1,
			ApplicationServerName: as,
			TriggerPoint: &IMSTriggerPoint{ConditionTypeCNF: true, SPT: []IMSServicePointTrigger{
				{Group: []int32{0}, SessionCase: &originating},
				{Group: []int32{1}, SIPHeader: &IMSHeaderMatch{Header: header, Content: &icsi}},
			}},
			DefaultHandling: strPtr("SESSION_TERMINATED"),
		},
	}
}

func TestIMSApplicationServers(t *testing.T) {
	env, client, token, notifier := newIMSServicesTestEnv(t)
	url := env.Server.URL

	list := func(t *testing.T) []IMSApplicationServerItem {
		t.Helper()

		status, resp, err := apiDo[ListIMSApplicationServersResponse](client, "GET", url+imsASPath, token, nil)
		if err != nil || status != http.StatusOK {
			t.Fatalf("list: status %d, err %v", status, err)
		}

		return resp.Result.Items
	}

	if got := list(t); got == nil || len(got) != 0 {
		t.Fatalf("application servers = %+v, want an empty list", got)
	}

	status, resp, err := createIMSAS(url, client, token, IMSApplicationServerItem{Name: "mcptt", ServerName: "sip:mcptt.ims.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"}})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create mcptt: status %d, response %+v, err %v", status, resp, err)
	}

	if resp.Result.Name != "mcptt" || resp.Result.ServerName != "sip:mcptt.ims.example.org" || !slices.Equal(resp.Result.PublicServiceIdentities, []string{"sip:!.*!@mcptt.ims.example.org"}) {
		t.Fatalf("created = %+v", resp.Result)
	}

	status, resp, err = createIMSAS(url, client, token, IMSApplicationServerItem{Name: "conf", ServerName: "sip:conf.ims.example.org"})
	if err != nil || status != http.StatusCreated || resp.Result.PublicServiceIdentities == nil || len(resp.Result.PublicServiceIdentities) != 0 {
		t.Fatalf("create conf without PSIs: status %d, response %+v, err %v", status, resp, err)
	}

	if got := list(t); len(got) != 2 || got[0].Name != "conf" || got[1].Name != "mcptt" || got[0].PublicServiceIdentities == nil {
		t.Fatalf("application servers = %+v, want conf then mcptt", got)
	}

	status, resp, err = apiDo[IMSApplicationServerResponse](client, "GET", url+imsASPath+"/mcptt", token, nil)
	if err != nil || status != http.StatusOK || resp.Result.Name != "mcptt" {
		t.Fatalf("get mcptt: status %d, response %+v, err %v", status, resp, err)
	}

	status, resp, err = apiDo[IMSApplicationServerResponse](client, "GET", url+imsASPath+"/missing", token, nil)
	if err != nil || status != http.StatusNotFound || resp.Error != "Application server not found" {
		t.Fatalf("get missing: status %d, response %+v, err %v", status, resp, err)
	}

	for name, tc := range map[string]struct {
		as     IMSApplicationServerItem
		status int
		reason string
	}{
		"no name":         {IMSApplicationServerItem{ServerName: "sip:x.example.org"}, http.StatusBadRequest, "application server name is required"},
		"bad server name": {IMSApplicationServerItem{Name: "x", ServerName: "not a uri"}, http.StatusBadRequest, ""},
		"duplicate PSI":   {IMSApplicationServerItem{Name: "x", ServerName: "sip:x.example.org", PublicServiceIdentities: []string{"sip:a@example.org", "sip:a@example.org"}}, http.StatusBadRequest, ""},
		"existing name":   {IMSApplicationServerItem{Name: "conf", ServerName: "sip:conf2.ims.example.org"}, http.StatusConflict, "Application server already exists"},
		"PSI conflict":    {IMSApplicationServerItem{Name: "x", ServerName: "sip:x.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"}}, http.StatusConflict, ""},
	} {
		status, resp, err := createIMSAS(url, client, token, tc.as)
		if err != nil || status != tc.status || resp.Error == "" || (tc.reason != "" && resp.Error != tc.reason) {
			t.Fatalf("%s: status %d, response %+v, err %v; want %d %q", name, status, resp, err, tc.status, tc.reason)
		}

		if name == "PSI conflict" && !strings.HasPrefix(resp.Error, "public service identity conflict: ") {
			t.Fatalf("PSI conflict reason = %q", resp.Error)
		}
	}

	update := UpdateIMSAS{ServerName: "sip:conf2.ims.example.org", PublicServiceIdentities: []string{"sip:conference@ims.example.org"}}

	upd, err := updateIMSAS(url, client, token, "conf", update)
	if err != nil || upd != http.StatusOK {
		t.Fatalf("update conf: status %d, err %v", upd, err)
	}

	conf, err := env.DB.GetIMSApplicationServer(context.Background(), "conf")
	if err != nil {
		t.Fatalf("get conf from db: %v", err)
	}

	if got := notifier.take(); !slices.Equal(got, []string{"as:" + conf.ID}) {
		t.Fatalf("notifications = %v, want the conf application server", got)
	}

	if conf.ServerName != "sip:conf2.ims.example.org" || !slices.Equal(conf.PublicServiceIdentities, []string{"sip:conference@ims.example.org"}) {
		t.Fatalf("conf = %+v", conf)
	}

	if status, err := updateIMSAS(url, client, token, "missing", update); err != nil || status != http.StatusNotFound {
		t.Fatalf("update missing: status %d, err %v", status, err)
	}

	if status, err := updateIMSAS(url, client, token, "conf", UpdateIMSAS{ServerName: "sip:conf2.ims.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"}}); err != nil || status != http.StatusConflict {
		t.Fatalf("update with a PSI conflict: status %d, err %v", status, err)
	}

	if got := notifier.take(); len(got) != 0 {
		t.Fatalf("failed updates notified %v", got)
	}

	if status, resp, err := apiDo[IMSMessageResponse](client, "PUT", url+"/api/v1/profiles/default/ims-filter-criteria", token, ProfileIMSFilterCriteriaBody{InitialFilterCriteria: mcpttFilterCriteria("mcptt")}); err != nil || status != http.StatusOK {
		t.Fatalf("give the default profile MCPTT iFCs: status %d, response %+v, err %v", status, resp, err)
	}

	status, del, err := apiDo[IMSMessageResponse](client, "DELETE", url+imsASPath+"/mcptt", token, nil)
	if err != nil || status != http.StatusConflict || del.Error != "Application server is used by a profile" {
		t.Fatalf("delete an application server in use: status %d, response %+v, err %v", status, del, err)
	}

	status, del, err = apiDo[IMSMessageResponse](client, "DELETE", url+imsASPath+"/conf", token, nil)
	if err != nil || status != http.StatusOK {
		t.Fatalf("delete conf: status %d, response %+v, err %v", status, del, err)
	}

	status, del, err = apiDo[IMSMessageResponse](client, "DELETE", url+imsASPath+"/conf", token, nil)
	if err != nil || status != http.StatusNotFound {
		t.Fatalf("delete conf again: status %d, response %+v, err %v", status, del, err)
	}
}

type UpdateIMSAS struct {
	ServerName              string   `json:"server_name"`
	PublicServiceIdentities []string `json:"public_service_identities"`
}

func updateIMSAS(url string, client *http.Client, token, name string, as UpdateIMSAS) (int, error) {
	status, _, err := apiDo[IMSMessageResponse](client, "PUT", url+imsASPath+"/"+name, token, as)
	return status, err
}

func strPtr(s string) *string {
	return &s
}

func TestProfileIMSFilterCriteria_TS29228_B22(t *testing.T) {
	env, client, token, notifier := newIMSServicesTestEnv(t)
	url := env.Server.URL
	path := url + "/api/v1/profiles/mc/ims-filter-criteria"

	for _, as := range []IMSApplicationServerItem{{Name: "mcptt", ServerName: "sip:mcptt.ims.example.org"}, {Name: "conf", ServerName: "sip:conf.ims.example.org"}} {
		if status, resp, err := createIMSAS(url, client, token, as); err != nil || status != http.StatusCreated {
			t.Fatalf("create %s: status %d, response %+v, err %v", as.Name, status, resp, err)
		}
	}

	status, resp, err := apiDo[ProfileIMSFilterCriteriaResponse](client, "GET", path, token, nil)
	if err != nil || status != http.StatusNotFound {
		t.Fatalf("get of a missing profile: status %d, response %+v, err %v", status, resp, err)
	}

	if status, _, err := apiDo[IMSMessageResponse](client, "PUT", path, token, ProfileIMSFilterCriteriaBody{InitialFilterCriteria: mcpttFilterCriteria("mcptt")}); err != nil || status != http.StatusNotFound {
		t.Fatalf("set on a missing profile: status %d, err %v", status, err)
	}

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{Name: "mc", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	profile, err := env.DB.GetProfile(context.Background(), "mc")
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	get := func(t *testing.T) []IMSInitialFilterCriterion {
		t.Helper()

		status, resp, err := apiDo[ProfileIMSFilterCriteriaResponse](client, "GET", path, token, nil)
		if err != nil || status != http.StatusOK {
			t.Fatalf("get: status %d, response %+v, err %v", status, resp, err)
		}

		return resp.Result.InitialFilterCriteria
	}

	if got := get(t); got == nil || len(got) != 0 {
		t.Fatalf("iFCs = %v, want an empty list", got)
	}

	uri, invite := "^sip:conference@.*$", "INVITE"
	conf := []IMSInitialFilterCriterion{
		{
			Priority:              7,
			ApplicationServerName: "conf",
			TriggerPoint:          &IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, RequestURI: &uri}, {Group: []int32{1}, Method: &invite, ConditionNegated: true}}},
			ProfilePart:           strPtr("REGISTERED"),
			DefaultHandling:       strPtr("SESSION_TERMINATED"),
		},
		{
			ApplicationServerName:   "conf",
			TriggerPoint:            &IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, Method: strPtr("REGISTER"), RegistrationType: []string{"INITIAL_REGISTRATION"}}}},
			DefaultHandling:         strPtr("SESSION_CONTINUED"),
			ServiceInfo:             strPtr("conference"),
			IncludeRegisterResponse: true,
		},
		{ApplicationServerName: "conf", ProfilePart: strPtr("UNREGISTERED")},
	}

	for _, ifcs := range [][]IMSInitialFilterCriterion{append(slices.Clone(conf), mcpttFilterCriteria("mcptt")...), mcpttFilterCriteria("mcptt"), {}} {
		status, resp, err := apiDo[IMSMessageResponse](client, "PUT", path, token, ProfileIMSFilterCriteriaBody{InitialFilterCriteria: ifcs})
		if err != nil || status != http.StatusOK {
			t.Fatalf("set %d iFCs: status %d, response %+v, err %v", len(ifcs), status, resp, err)
		}

		want := slices.Clone(ifcs)
		for i := range want {
			want[i].Priority = i
		}

		if got := get(t); !reflect.DeepEqual(got, want) {
			t.Fatalf("iFCs = %+v, want %+v", got, want)
		}

		if got := notifier.take(); !slices.Equal(got, []string{"profile:" + profile.ID}) {
			t.Fatalf("notifications = %v, want the profile", got)
		}
	}

	for name, tc := range map[string]struct {
		ifcs   []IMSInitialFilterCriterion
		reason string
	}{
		"no application server":      {[]IMSInitialFilterCriterion{{}}, "Each initial filter criterion needs an application server"},
		"unknown application server": {[]IMSInitialFilterCriterion{{ApplicationServerName: "missing"}}, "Application server not found: missing"},
		"two conditions":             {[]IMSInitialFilterCriterion{{ApplicationServerName: "conf", TriggerPoint: &IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, Method: &invite, RequestURI: &uri}}}}}, ""},
		"bad handling":               {[]IMSInitialFilterCriterion{{ApplicationServerName: "conf", DefaultHandling: strPtr("MAYBE")}}, `unknown default handling "MAYBE"`},
		"bad profile part":           {[]IMSInitialFilterCriterion{{ApplicationServerName: "conf", ProfilePart: strPtr("BOTH")}}, `unknown profile part "BOTH"`},
		"registration type":          {[]IMSInitialFilterCriterion{{ApplicationServerName: "conf", TriggerPoint: &IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, Method: &invite, RegistrationType: []string{"INITIAL_REGISTRATION"}}}}}}, ""},
	} {
		status, resp, err := apiDo[IMSMessageResponse](client, "PUT", path, token, ProfileIMSFilterCriteriaBody{InitialFilterCriteria: tc.ifcs})
		if err != nil || status != http.StatusBadRequest || resp.Error == "" || (tc.reason != "" && !strings.HasSuffix(resp.Error, tc.reason)) {
			t.Fatalf("%s: status %d, response %+v, err %v; want 400 %q", name, status, resp, err, tc.reason)
		}
	}

	if got := notifier.take(); len(got) != 0 {
		t.Fatalf("failed updates notified %v", got)
	}

	if got := get(t); len(got) != 0 {
		t.Fatalf("rejected updates changed the iFCs to %v", got)
	}
}

func TestSubscriberProfileChangeNotifiesIMS(t *testing.T) {
	env, client, token, notifier := newIMSServicesTestEnv(t)
	url := env.Server.URL

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{Name: "mc", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	if status, resp, err := createPolicy(url, client, token, &CreatePolicyParams{
		Name: "mc", ProfileName: "mc", SliceName: DefaultSliceName, DataNetworkName: "internet",
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 3,
	}); err != nil || status != http.StatusCreated {
		t.Fatalf("create policy: status %d, response %+v, err %v", status, resp, err)
	}

	if status, resp, err := createSubscriber(url, client, token, &CreateSubscriberParams{Imsi: Imsi, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: DefaultProfileName}); err != nil || status != http.StatusCreated {
		t.Fatalf("create subscriber: status %d, response %+v, err %v", status, resp, err)
	}

	if status, resp, err := updateSubscriber(url, client, token, Imsi, &UpdateSubscriberParams{Imsi: Imsi, ProfileName: DefaultProfileName, Description: "same profile"}); err != nil || status != http.StatusOK {
		t.Fatalf("update without a profile change: status %d, response %+v, err %v", status, resp, err)
	}

	if got := notifier.take(); len(got) != 0 {
		t.Fatalf("an update keeping the profile notified %v", got)
	}

	if status, resp, err := updateSubscriber(url, client, token, Imsi, &UpdateSubscriberParams{Imsi: Imsi, ProfileName: "mc"}); err != nil || status != http.StatusOK {
		t.Fatalf("update to mc: status %d, response %+v, err %v", status, resp, err)
	}

	if got := notifier.take(); !slices.Equal(got, []string{"subscriber:" + Imsi}) {
		t.Fatalf("notifications = %v, want the subscriber", got)
	}

	if status, resp, err := updateSubscriber(url, client, token, Imsi, &UpdateSubscriberParams{Imsi: Imsi, ProfileName: "mc", Msisdn: "+15550100"}); err != nil || status != http.StatusOK {
		t.Fatalf("add an MSISDN: status %d, response %+v, err %v", status, resp, err)
	}

	if status, resp, err := updateSubscriber(url, client, token, Imsi, &UpdateSubscriberParams{Imsi: Imsi, ProfileName: DefaultProfileName, Msisdn: "+15550199"}); err != nil || status != http.StatusOK {
		t.Fatalf("change the MSISDN and the profile: status %d, response %+v, err %v", status, resp, err)
	}

	if got := notifier.take(); !slices.Equal(got, []string{"identities:" + Imsi + ":", "identities:" + Imsi + ":15550100"}) {
		t.Fatalf("notifications = %v, want two identity changes", got)
	}
}

func TestIMSServicesPermissions(t *testing.T) {
	env, err := setupServer(filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	t.Cleanup(env.Server.Close)

	url := env.Server.URL
	admin := newTestClient(env.Server)

	adminToken, err := initializeAndRefresh(url, admin)
	if err != nil {
		t.Fatalf("login: %s", err)
	}

	if status, resp, err := createIMSAS(url, admin, adminToken, IMSApplicationServerItem{Name: "as", ServerName: "sip:as.ims.example.org"}); err != nil || status != http.StatusCreated {
		t.Fatalf("create AS: status %d, response %+v, err %v", status, resp, err)
	}

	readOnly := newTestClient(env.Server)

	readOnlyToken, err := createUserAndLogin(url, adminToken, "readonly@ellanetworks.com", RoleReadOnly, readOnly)
	if err != nil {
		t.Fatalf("create read-only user: %s", err)
	}

	manager := newTestClient(env.Server)

	managerToken, err := createUserAndLogin(url, adminToken, "manager@ellanetworks.com", RoleNetworkManager, manager)
	if err != nil {
		t.Fatalf("create network manager: %s", err)
	}

	for _, path := range []string{imsASPath, imsASPath + "/as", "/api/v1/profiles/default/ims-filter-criteria"} {
		status, _, err := apiDo[map[string]any](readOnly, "GET", url+path, readOnlyToken, nil)
		if err != nil || status != http.StatusOK {
			t.Fatalf("read-only GET %s: status %d, err %v", path, status, err)
		}
	}

	if status, _, err := createIMSAS(url, readOnly, readOnlyToken, IMSApplicationServerItem{Name: "ro", ServerName: "sip:ro.ims.example.org"}); err != nil || status != http.StatusForbidden {
		t.Fatalf("read-only create AS: status %d, err %v", status, err)
	}

	if status, _, err := apiDo[IMSMessageResponse](readOnly, "PUT", url+"/api/v1/profiles/default/ims-filter-criteria", readOnlyToken, ProfileIMSFilterCriteriaBody{InitialFilterCriteria: []IMSInitialFilterCriterion{}}); err != nil || status != http.StatusForbidden {
		t.Fatalf("read-only set profile iFCs: status %d, err %v", status, err)
	}

	if status, _, err := apiDo[IMSMessageResponse](manager, "PUT", url+"/api/v1/profiles/default/ims-filter-criteria", managerToken, ProfileIMSFilterCriteriaBody{InitialFilterCriteria: mcpttFilterCriteria("as")}); err != nil || status != http.StatusOK {
		t.Fatalf("network manager set profile iFCs: status %d, err %v", status, err)
	}
}

func TestIMSNamesAreBounded(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL
	long := strings.Repeat("a", 256)

	status, _, err := apiDo[map[string]any](client, "POST", url+"/api/v1/operator/ims/application-servers", token, map[string]any{"name": long, "server_name": "sip:as.example.org"})
	if err != nil || status != http.StatusBadRequest {
		t.Fatalf("create an application server with a 256-character name: status %d, err %v", status, err)
	}
}
