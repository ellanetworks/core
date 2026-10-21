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
	Priority                int             `json:"priority"`
	TriggerPoint            IMSTriggerPoint `json:"trigger_point"`
	DefaultHandling         string          `json:"default_handling"`
	IncludeRegisterRequest  bool            `json:"include_register_request"`
	IncludeRegisterResponse bool            `json:"include_register_response"`
}

type IMSServiceItem struct {
	Name                  string                      `json:"name"`
	Kind                  string                      `json:"kind"`
	ApplicationServerName string                      `json:"application_server_name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type IMSServiceParams struct {
	Name                  string                      `json:"name,omitempty"`
	Kind                  string                      `json:"kind"`
	ApplicationServerName string                      `json:"application_server_name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria,omitempty"`
}

type IMSServiceResponse struct {
	Result IMSServiceItem `json:"result"`
	Error  string         `json:"error,omitempty"`
}

type ListIMSServicesResponse struct {
	Result struct {
		Items []IMSServiceItem `json:"items"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type ProfileIMSServicesBody struct {
	Services []string `json:"services"`
}

type ProfileIMSServicesResponse struct {
	Result ProfileIMSServicesBody `json:"result"`
	Error  string                 `json:"error,omitempty"`
}

type IMSMessageResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

const (
	imsASPath      = "/api/v1/operator/ims/application-servers"
	imsServicePath = "/api/v1/operator/ims/services"
)

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

func (f *fakeIMSProfileNotifier) ServiceChanged(_ context.Context, id string) {
	f.record("service:" + id)
}

func (f *fakeIMSProfileNotifier) ProfileChanged(_ context.Context, id string) {
	f.record("profile:" + id)
}

func (f *fakeIMSProfileNotifier) SubscriberChanged(_ context.Context, imsi string) {
	f.record("subscriber:" + imsi)
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

func createIMSService(url string, client *http.Client, token string, svc IMSServiceParams) (int, *IMSServiceResponse, error) {
	return apiDo[IMSServiceResponse](client, "POST", url+imsServicePath, token, svc)
}

func mcpttFilterCriteria() []IMSInitialFilterCriterion {
	register, originating, header, icsi := "REGISTER", "ORIGINATING_REGISTERED", "^P-Asserted-Service$", `^urn:urn-7:3gpp-service\.ims\.icsi\.mcptt$`

	return []IMSInitialFilterCriterion{
		{
			Priority:                0,
			TriggerPoint:            IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, Method: &register}}},
			DefaultHandling:         "SESSION_CONTINUED",
			IncludeRegisterRequest:  true,
			IncludeRegisterResponse: true,
		},
		{
			Priority: 1,
			TriggerPoint: IMSTriggerPoint{ConditionTypeCNF: true, SPT: []IMSServicePointTrigger{
				{Group: []int32{0}, SessionCase: &originating},
				{Group: []int32{1}, SIPHeader: &IMSHeaderMatch{Header: header, Content: &icsi}},
			}},
			DefaultHandling: "SESSION_TERMINATED",
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

	if status, resp, err := createIMSService(url, client, token, IMSServiceParams{Name: "mcptt", Kind: "mcptt", ApplicationServerName: "mcptt"}); err != nil || status != http.StatusCreated {
		t.Fatalf("create service: status %d, response %+v, err %v", status, resp, err)
	}

	status, del, err := apiDo[IMSMessageResponse](client, "DELETE", url+imsASPath+"/mcptt", token, nil)
	if err != nil || status != http.StatusConflict || del.Error != "Application server is used by an IMS service" {
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

func TestIMSServices(t *testing.T) {
	env, client, token, notifier := newIMSServicesTestEnv(t)
	url := env.Server.URL

	for _, as := range []IMSApplicationServerItem{
		{Name: "mcptt", ServerName: "sip:mcptt.ims.example.org"},
		{Name: "conf", ServerName: "sip:conf.ims.example.org"},
	} {
		if status, resp, err := createIMSAS(url, client, token, as); err != nil || status != http.StatusCreated {
			t.Fatalf("create %s: status %d, response %+v, err %v", as.Name, status, resp, err)
		}
	}

	status, resp, err := createIMSService(url, client, token, IMSServiceParams{Name: "mcptt", Kind: "mcptt", ApplicationServerName: "mcptt"})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create mcptt: status %d, response %+v, err %v", status, resp, err)
	}

	wantMCPTT := IMSServiceItem{Name: "mcptt", Kind: "mcptt", ApplicationServerName: "mcptt", InitialFilterCriteria: mcpttFilterCriteria()}

	if !reflect.DeepEqual(resp.Result, wantMCPTT) {
		t.Fatalf("created mcptt = %+v, want %+v", resp.Result, wantMCPTT)
	}

	status, resp, err = apiDo[IMSServiceResponse](client, "GET", url+imsServicePath+"/mcptt", token, nil)
	if err != nil || status != http.StatusOK || !reflect.DeepEqual(resp.Result, wantMCPTT) {
		t.Fatalf("get mcptt: status %d, response %+v, err %v", status, resp, err)
	}

	uri, invite := "^sip:conference@.*$", "INVITE"
	custom := []IMSInitialFilterCriterion{
		{
			Priority:        7,
			TriggerPoint:    IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, RequestURI: &uri}, {Group: []int32{1}, Method: &invite, ConditionNegated: true}}},
			DefaultHandling: "SESSION_TERMINATED",
		},
		{
			TriggerPoint:            IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, Method: strPtr("REGISTER"), RegistrationType: []string{"INITIAL_REGISTRATION"}}}},
			DefaultHandling:         "SESSION_CONTINUED",
			IncludeRegisterResponse: true,
		},
	}

	status, resp, err = createIMSService(url, client, token, IMSServiceParams{Name: "conf", Kind: "custom", ApplicationServerName: "conf", InitialFilterCriteria: custom})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create conf: status %d, response %+v, err %v", status, resp, err)
	}

	wantConf := IMSServiceItem{Name: "conf", Kind: "custom", ApplicationServerName: "conf", InitialFilterCriteria: slices.Clone(custom)}
	wantConf.InitialFilterCriteria[0].Priority = 0
	wantConf.InitialFilterCriteria[1].Priority = 1

	if !reflect.DeepEqual(resp.Result, wantConf) {
		t.Fatalf("created conf = %+v, want %+v", resp.Result, wantConf)
	}

	listStatus, list, err := apiDo[ListIMSServicesResponse](client, "GET", url+imsServicePath, token, nil)
	if err != nil || listStatus != http.StatusOK || !reflect.DeepEqual(list.Result.Items, []IMSServiceItem{wantConf, wantMCPTT}) {
		t.Fatalf("list: status %d, response %+v, err %v", listStatus, list, err)
	}

	for name, tc := range map[string]struct {
		svc    IMSServiceParams
		status int
		reason string
	}{
		"mcptt with iFCs":   {IMSServiceParams{Name: "x", Kind: "mcptt", ApplicationServerName: "mcptt", InitialFilterCriteria: custom}, http.StatusBadRequest, "an IMS service of kind mcptt takes no initial filter criteria"},
		"custom no iFCs":    {IMSServiceParams{Name: "x", Kind: "custom", ApplicationServerName: "conf"}, http.StatusBadRequest, "an IMS service of kind custom needs at least one initial filter criterion"},
		"unknown kind":      {IMSServiceParams{Name: "x", Kind: "voice", ApplicationServerName: "conf"}, http.StatusBadRequest, `unknown IMS service kind "voice"`},
		"no name":           {IMSServiceParams{Kind: "mcptt", ApplicationServerName: "mcptt"}, http.StatusBadRequest, "IMS service name is required"},
		"unknown AS":        {IMSServiceParams{Name: "x", Kind: "mcptt", ApplicationServerName: "missing"}, http.StatusBadRequest, "Application server not found"},
		"two conditions":    {IMSServiceParams{Name: "x", Kind: "custom", ApplicationServerName: "conf", InitialFilterCriteria: []IMSInitialFilterCriterion{{TriggerPoint: IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, Method: &invite, RequestURI: &uri}}}, DefaultHandling: "SESSION_CONTINUED"}}}, http.StatusBadRequest, ""},
		"bad handling":      {IMSServiceParams{Name: "x", Kind: "custom", ApplicationServerName: "conf", InitialFilterCriteria: []IMSInitialFilterCriterion{{TriggerPoint: IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, Method: &invite}}}, DefaultHandling: "MAYBE"}}}, http.StatusBadRequest, ""},
		"existing service":  {IMSServiceParams{Name: "mcptt", Kind: "mcptt", ApplicationServerName: "mcptt"}, http.StatusConflict, "IMS service already exists"},
		"registration type": {IMSServiceParams{Name: "x", Kind: "custom", ApplicationServerName: "conf", InitialFilterCriteria: []IMSInitialFilterCriterion{{TriggerPoint: IMSTriggerPoint{SPT: []IMSServicePointTrigger{{Group: []int32{0}, Method: &invite, RegistrationType: []string{"INITIAL_REGISTRATION"}}}}, DefaultHandling: "SESSION_CONTINUED"}}}, http.StatusBadRequest, ""},
	} {
		status, resp, err := createIMSService(url, client, token, tc.svc)
		if err != nil || status != tc.status || resp.Error == "" || (tc.reason != "" && !strings.HasSuffix(resp.Error, tc.reason)) {
			t.Fatalf("%s: status %d, response %+v, err %v; want %d %q", name, status, resp, err, tc.status, tc.reason)
		}
	}

	update := func(name string, svc IMSServiceParams) (int, *IMSMessageResponse) {
		t.Helper()

		status, resp, err := apiDo[IMSMessageResponse](client, "PUT", url+imsServicePath+"/"+name, token, svc)
		if err != nil {
			t.Fatalf("update %s: %v", name, err)
		}

		return status, resp
	}

	if status, resp := update("mcptt", IMSServiceParams{Kind: "mcptt", ApplicationServerName: "mcptt", InitialFilterCriteria: custom}); status != http.StatusBadRequest {
		t.Fatalf("update mcptt with iFCs: status %d, response %+v", status, resp)
	}

	if status, resp := update("missing", IMSServiceParams{Kind: "mcptt", ApplicationServerName: "mcptt"}); status != http.StatusNotFound || resp.Error != "IMS service not found" {
		t.Fatalf("update missing: status %d, response %+v", status, resp)
	}

	if status, resp := update("conf", IMSServiceParams{Kind: "mcptt", ApplicationServerName: "missing"}); status != http.StatusBadRequest || resp.Error != "Application server not found" {
		t.Fatalf("update to an unknown AS: status %d, response %+v", status, resp)
	}

	if got := notifier.take(); len(got) != 0 {
		t.Fatalf("failed updates notified %v", got)
	}

	if status, resp := update("conf", IMSServiceParams{Kind: "custom", ApplicationServerName: "mcptt", InitialFilterCriteria: custom[1:]}); status != http.StatusOK {
		t.Fatalf("update conf: status %d, response %+v", status, resp)
	}

	conf, err := env.DB.GetIMSService(context.Background(), "conf")
	if err != nil {
		t.Fatalf("get conf from db: %v", err)
	}

	if got := notifier.take(); !slices.Equal(got, []string{"service:" + conf.ID}) {
		t.Fatalf("notifications = %v, want the conf service", got)
	}

	status, resp, err = apiDo[IMSServiceResponse](client, "GET", url+imsServicePath+"/conf", token, nil)
	if err != nil || status != http.StatusOK || resp.Result.ApplicationServerName != "mcptt" || len(resp.Result.InitialFilterCriteria) != 1 || resp.Result.InitialFilterCriteria[0].Priority != 0 || !resp.Result.InitialFilterCriteria[0].IncludeRegisterResponse {
		t.Fatalf("get conf after update: status %d, response %+v, err %v", status, resp, err)
	}

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{Name: "mc", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	if status, _, err := apiDo[IMSMessageResponse](client, "PUT", url+"/api/v1/profiles/mc/ims-services", token, ProfileIMSServicesBody{Services: []string{"conf"}}); err != nil || status != http.StatusOK {
		t.Fatalf("assign conf: status %d, err %v", status, err)
	}

	status, del, err := apiDo[IMSMessageResponse](client, "DELETE", url+imsServicePath+"/conf", token, nil)
	if err != nil || status != http.StatusConflict || del.Error != "IMS service is assigned to a profile" {
		t.Fatalf("delete an assigned service: status %d, response %+v, err %v", status, del, err)
	}

	status, del, err = apiDo[IMSMessageResponse](client, "DELETE", url+imsServicePath+"/mcptt", token, nil)
	if err != nil || status != http.StatusOK {
		t.Fatalf("delete mcptt: status %d, response %+v, err %v", status, del, err)
	}

	status, del, err = apiDo[IMSMessageResponse](client, "DELETE", url+imsServicePath+"/mcptt", token, nil)
	if err != nil || status != http.StatusNotFound {
		t.Fatalf("delete mcptt again: status %d, response %+v, err %v", status, del, err)
	}

	status, resp, err = apiDo[IMSServiceResponse](client, "GET", url+imsServicePath+"/mcptt", token, nil)
	if err != nil || status != http.StatusNotFound {
		t.Fatalf("get deleted mcptt: status %d, response %+v, err %v", status, resp, err)
	}
}

func strPtr(s string) *string {
	return &s
}

func TestProfileIMSServices(t *testing.T) {
	env, client, token, notifier := newIMSServicesTestEnv(t)
	url := env.Server.URL
	path := url + "/api/v1/profiles/mc/ims-services"

	if status, resp, err := createIMSAS(url, client, token, IMSApplicationServerItem{Name: "as", ServerName: "sip:as.ims.example.org"}); err != nil || status != http.StatusCreated {
		t.Fatalf("create AS: status %d, response %+v, err %v", status, resp, err)
	}

	for _, name := range []string{"a", "b", "c"} {
		if status, resp, err := createIMSService(url, client, token, IMSServiceParams{Name: name, Kind: "mcptt", ApplicationServerName: "as"}); err != nil || status != http.StatusCreated {
			t.Fatalf("create service %s: status %d, response %+v, err %v", name, status, resp, err)
		}
	}

	status, resp, err := apiDo[ProfileIMSServicesResponse](client, "GET", path, token, nil)
	if err != nil || status != http.StatusNotFound {
		t.Fatalf("get of a missing profile: status %d, response %+v, err %v", status, resp, err)
	}

	if status, _, err := apiDo[IMSMessageResponse](client, "PUT", path, token, ProfileIMSServicesBody{Services: []string{"a"}}); err != nil || status != http.StatusNotFound {
		t.Fatalf("set on a missing profile: status %d, err %v", status, err)
	}

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{Name: "mc", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	profile, err := env.DB.GetProfile(context.Background(), "mc")
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	get := func(t *testing.T) []string {
		t.Helper()

		status, resp, err := apiDo[ProfileIMSServicesResponse](client, "GET", path, token, nil)
		if err != nil || status != http.StatusOK {
			t.Fatalf("get: status %d, response %+v, err %v", status, resp, err)
		}

		return resp.Result.Services
	}

	if got := get(t); got == nil || len(got) != 0 {
		t.Fatalf("services = %v, want an empty list", got)
	}

	for _, order := range [][]string{{"c", "a", "b"}, {"b", "c"}, {}} {
		status, resp, err := apiDo[IMSMessageResponse](client, "PUT", path, token, ProfileIMSServicesBody{Services: order})
		if err != nil || status != http.StatusOK {
			t.Fatalf("set %v: status %d, response %+v, err %v", order, status, resp, err)
		}

		if got := get(t); !slices.Equal(got, order) {
			t.Fatalf("services = %v, want %v", got, order)
		}

		if got := notifier.take(); !slices.Equal(got, []string{"profile:" + profile.ID}) {
			t.Fatalf("notifications = %v, want the profile", got)
		}
	}

	for name, tc := range map[string]struct {
		services []string
		reason   string
	}{
		"unknown":   {[]string{"a", "missing"}, "IMS service not found: missing"},
		"duplicate": {[]string{"a", "b", "a"}, "IMS service a is listed twice"},
	} {
		status, resp, err := apiDo[IMSMessageResponse](client, "PUT", path, token, ProfileIMSServicesBody{Services: tc.services})
		if err != nil || status != http.StatusBadRequest || resp.Error != tc.reason {
			t.Fatalf("%s: status %d, response %+v, err %v; want 400 %q", name, status, resp, err, tc.reason)
		}
	}

	if got := notifier.take(); len(got) != 0 {
		t.Fatalf("failed updates notified %v", got)
	}

	if got := get(t); len(got) != 0 {
		t.Fatalf("rejected updates changed the services to %v", got)
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

	for _, path := range []string{imsASPath, imsASPath + "/as", imsServicePath, "/api/v1/profiles/default/ims-services"} {
		status, _, err := apiDo[map[string]any](readOnly, "GET", url+path, readOnlyToken, nil)
		if err != nil || status != http.StatusOK {
			t.Fatalf("read-only GET %s: status %d, err %v", path, status, err)
		}
	}

	if status, _, err := createIMSAS(url, readOnly, readOnlyToken, IMSApplicationServerItem{Name: "ro", ServerName: "sip:ro.ims.example.org"}); err != nil || status != http.StatusForbidden {
		t.Fatalf("read-only create AS: status %d, err %v", status, err)
	}

	if status, _, err := createIMSService(url, readOnly, readOnlyToken, IMSServiceParams{Name: "ro", Kind: "mcptt", ApplicationServerName: "as"}); err != nil || status != http.StatusForbidden {
		t.Fatalf("read-only create service: status %d, err %v", status, err)
	}

	if status, _, err := apiDo[IMSMessageResponse](readOnly, "PUT", url+"/api/v1/profiles/default/ims-services", readOnlyToken, ProfileIMSServicesBody{Services: []string{}}); err != nil || status != http.StatusForbidden {
		t.Fatalf("read-only set profile services: status %d, err %v", status, err)
	}

	if status, _, err := createIMSService(url, manager, managerToken, IMSServiceParams{Name: "svc", Kind: "mcptt", ApplicationServerName: "as"}); err != nil || status != http.StatusCreated {
		t.Fatalf("network manager create service: status %d, err %v", status, err)
	}

	if status, _, err := apiDo[IMSMessageResponse](manager, "PUT", url+"/api/v1/profiles/default/ims-services", managerToken, ProfileIMSServicesBody{Services: []string{"svc"}}); err != nil || status != http.StatusOK {
		t.Fatalf("network manager set profile services: status %d, err %v", status, err)
	}
}
