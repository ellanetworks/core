// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/sepp"
)

type fakeSEPP struct {
	mu          sync.Mutex
	contexts    map[string]sepp.ContextStatus
	rehandshake []string
	err         error
	listener    sepp.ListenerStatus
}

func (f *fakeSEPP) Status() sepp.NodeStatus {
	f.mu.Lock()
	defer f.mu.Unlock()

	contexts := map[string]sepp.ContextStatus{}
	for k, v := range f.contexts {
		contexts[k] = v
	}

	return sepp.NodeStatus{SEPPID: 1, FQDN: ownSEPPFQDN, Listener: f.listener, Contexts: contexts}
}

func (f *fakeSEPP) Rehandshake(id string) (sepp.ContextStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return sepp.ContextStatus{}, f.err
	}

	f.rehandshake = append(f.rehandshake, id)

	return sepp.ContextStatus{State: sepp.StateEstablishing, Since: time.Unix(1700000000, 0)}, nil
}

type roamingPartnerN32Body struct {
	State     string `json:"state"`
	Peer      string `json:"peer"`
	LastError string `json:"lastError"`
}

type partnerWithN32 struct {
	ID  string                 `json:"id"`
	N32 *roamingPartnerN32Body `json:"n32"`
}

type seppStatusBody struct {
	TrustAnchors []struct {
		Source      string `json:"source"`
		Fingerprint string `json:"fingerprint"`
		PEM         string `json:"pem"`
		SEPPIDs     []int  `json:"seppIDs"`
	} `json:"trustAnchors"`
	Listener struct {
		SEPPID     int    `json:"seppID"`
		Configured bool   `json:"configured"`
		Address    string `json:"address"`
		Port       int    `json:"port"`
		Listening  bool   `json:"listening"`
		Error      string `json:"error"`
	} `json:"listener"`
	Certificates []struct {
		SEPPID                 int      `json:"seppID"`
		FQDN                   string   `json:"fqdn"`
		Source                 string   `json:"source"`
		SANs                   []string `json:"sans"`
		Fingerprint            string   `json:"fingerprint"`
		TrustAnchorFingerprint string   `json:"trustAnchorFingerprint"`
		NotAfter               string   `json:"notAfter"`
	} `json:"certificates"`
}

func newSEPPTestEnv(t *testing.T, f *fakeSEPP) (testEnv, *http.Client, string) {
	t.Helper()

	testdb, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}

	env, err := buildTestEnvWith(testdb, nil, nil, func(c *server.HandlerConfig) { c.SEPP = f })
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(env.Server.Close)

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatal(err)
	}

	return env, client, token
}

func TestRoamingPartnerN32Status(t *testing.T) {
	f := &fakeSEPP{contexts: map[string]sepp.ContextStatus{}, listener: sepp.ListenerStatus{Configured: true, Listening: true}}
	env, client, token := newSEPPTestEnv(t, f)
	url := env.Server.URL

	code, created, err := apiDo[roamingPartnerEnvelope](client, "POST", url+roamingPartnersPath, token, partner5G(t, "p5g"))
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create: %d %v", code, err)
	}

	id := created.Result.ID

	code, fourG, err := apiDo[roamingDiameterPeerEnvelope](client, "POST", url+roamingDiameterPeersPath, token, roamingDiameterPeerBody{Address: "192.0.2.10"})
	if err != nil || code != http.StatusCreated {
		t.Fatalf("diameter peer: %d %v", code, err)
	}

	code, p4g, err := apiDo[roamingPartnerEnvelope](client, "POST", url+roamingPartnersPath, token, roamingPartnerBody{Name: "p4g", PLMNs: plmnList("310", "410"), ProfileName: "default", DiameterPeerID: fourG.Result.ID})
	if err != nil || code != http.StatusCreated {
		t.Fatalf("4G partner: %d %v", code, err)
	}

	since := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

	f.mu.Lock()
	f.contexts[id] = sepp.ContextStatus{State: sepp.StateEstablished, Since: since, Initiator: sepp.InitiatorPeer, Peer: "sepp2.5gc.mnc093.mcc208.3gppnetwork.org"}
	f.mu.Unlock()

	code, got, err := apiDo[struct {
		Result partnerWithN32 `json:"result"`
	}](client, "GET", url+roamingPartnersPath+"/"+id, token, nil)
	if err != nil || code != http.StatusOK || got.Result.N32 == nil {
		t.Fatalf("get: %d %v %+v", code, err, got)
	}

	if want := (roamingPartnerN32Body{State: "established", Peer: "sepp2.5gc.mnc093.mcc208.3gppnetwork.org"}); *got.Result.N32 != want {
		t.Fatalf("n32 = %+v, want %+v", *got.Result.N32, want)
	}

	code, list, err := apiDo[struct {
		Result struct {
			Items []partnerWithN32 `json:"items"`
		} `json:"result"`
	}](client, "GET", url+roamingPartnersPath, token, nil)
	if err != nil || code != http.StatusOK || len(list.Result.Items) != 2 {
		t.Fatalf("list: %d %v", code, err)
	}

	for _, item := range list.Result.Items {
		if (item.ID == id) != (item.N32 != nil) {
			t.Fatalf("listed partner %s n32 = %+v", item.ID, item.N32)
		}
	}

	code, hs, err := apiDo[struct {
		Result struct {
			N32 roamingPartnerN32Body `json:"n32"`
		} `json:"result"`
	}](client, "POST", url+roamingPartnersPath+"/"+id+"/handshake", token, nil)
	if err != nil || code != http.StatusAccepted || hs.Result.N32.State != "establishing" || len(f.rehandshake) != 1 || f.rehandshake[0] != id {
		t.Fatalf("handshake: %d %v %+v %v", code, err, hs, f.rehandshake)
	}

	if code, _, _ := apiDo[map[string]any](client, "POST", url+roamingPartnersPath+"/"+p4g.Result.ID+"/handshake", token, nil); code != http.StatusConflict {
		t.Fatalf("handshake 4G-only partner: %d", code)
	}

	if code, _, _ := apiDo[map[string]any](client, "POST", url+roamingPartnersPath+"/missing/handshake", token, nil); code != http.StatusNotFound {
		t.Fatalf("handshake unknown partner: %d", code)
	}

	f.mu.Lock()
	f.err = sepp.ErrNotReady
	f.mu.Unlock()

	code, failed, _ := apiDo[struct {
		Result struct {
			N32 roamingPartnerN32Body `json:"n32"`
		} `json:"result"`
	}](client, "POST", url+roamingPartnersPath+"/"+id+"/handshake", token, nil)
	if code != http.StatusAccepted || failed.Result.N32.State != "unknown" || failed.Result.N32.LastError == "" {
		t.Fatalf("handshake on a stopped SEPP: %d %+v", code, failed.Result)
	}

	f.mu.Lock()
	f.listener.Configured = false
	f.mu.Unlock()

	if code, _, _ := apiDo[map[string]any](client, "POST", url+roamingPartnersPath+"/"+id+"/handshake", token, nil); code != http.StatusConflict {
		t.Fatalf("handshake without interfaces.n32: %d", code)
	}

	readOnly := newTestClient(env.Server)

	roToken, err := createUserAndLogin(url, token, "ro-n32@ellanetworks.com", RoleReadOnly, readOnly)
	if err != nil {
		t.Fatal(err)
	}

	if code, _, _ := apiDo[map[string]any](readOnly, "POST", url+roamingPartnersPath+"/"+id+"/handshake", roToken, nil); code != http.StatusForbidden {
		t.Fatalf("read-only handshake: %d", code)
	}

	if code, _, _ := apiDo[map[string]any](readOnly, "GET", url+"/api/v1/integrations/roaming/sepp", roToken, nil); code != http.StatusOK {
		t.Fatalf("read-only SEPP status: %d", code)
	}

	if code, _, _ := apiDo[map[string]any](readOnly, "POST", url+"/api/v1/integrations/roaming/sepp/ca/regenerate", roToken, nil); code != http.StatusForbidden {
		t.Fatalf("read-only CA regeneration: %d", code)
	}
}

func TestSEPPStatus(t *testing.T) {
	f := &fakeSEPP{listener: sepp.ListenerStatus{Configured: true, Address: "192.0.2.5", Port: 443, Listening: true}}
	env, client, token := newSEPPTestEnv(t, f)
	url := env.Server.URL

	code, st, err := apiDo[struct {
		Result seppStatusBody `json:"result"`
	}](client, "GET", url+"/api/v1/integrations/roaming/sepp", token, nil)
	if err != nil || code != http.StatusOK || len(st.Result.TrustAnchors) != 0 || len(st.Result.Certificates) != 0 || st.Result.Listener.SEPPID != 1 {
		t.Fatalf("before identity: %d %v %+v", code, err, st)
	}

	for _, seppID := range []int{1, 5} {
		if err := env.DB.PutGeneratedSEPPIdentity(context.Background(), seppID); err != nil {
			t.Fatalf("PutGeneratedSEPPIdentity: %s", err)
		}
	}

	code, st, err = apiDo[struct {
		Result seppStatusBody `json:"result"`
	}](client, "GET", url+"/api/v1/integrations/roaming/sepp", token, nil)
	if err != nil || code != http.StatusOK {
		t.Fatalf("status: %d %v", code, err)
	}

	r := st.Result
	if len(r.TrustAnchors) != 1 || r.TrustAnchors[0].Source != "generated" || r.TrustAnchors[0].PEM == "" || len(r.TrustAnchors[0].SEPPIDs) != 2 {
		t.Fatalf("trust anchors = %+v", r.TrustAnchors)
	}

	if l := r.Listener; l.SEPPID != 1 || !l.Configured || l.Address != "192.0.2.5" || l.Port != 443 || !l.Listening {
		t.Fatalf("listener = %+v", l)
	}

	c := r.Certificates
	if len(c) != 2 || c[0].SEPPID != 1 || c[0].FQDN != ownSEPPFQDN || c[0].Source != "generated" || len(c[0].SANs) == 0 || c[0].SANs[0] != ownSEPPFQDN ||
		c[0].TrustAnchorFingerprint != r.TrustAnchors[0].Fingerprint || c[0].Fingerprint == c[0].TrustAnchorFingerprint || c[0].NotAfter == "" ||
		c[1].SEPPID != 5 || c[1].Source != "generated" || c[1].Fingerprint == "" {
		t.Fatalf("certificates = %+v", c)
	}

	if code, _, err := apiDo[map[string]any](client, "DELETE", url+seppCertificatePath, token, nil); err != nil || code != http.StatusOK {
		t.Fatalf("delete: %d %v", code, err)
	}

	if code, _, _ := apiDo[map[string]any](client, "DELETE", url+seppCertificatePath, token, nil); code != http.StatusNotFound {
		t.Fatalf("delete again: %d", code)
	}

	if _, err := env.DB.GetSEPPIdentity(context.Background(), 1); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("identity after the delete: %v", err)
	}
}
