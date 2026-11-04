// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/pki"
)

const homeSEPPFQDN = "sepp1.5gc.mnc001.mcc001.3gppnetwork.org"

type roamingPartnerSEPPBody struct {
	FQDN           string `json:"fqdn"`
	N32cURI        string `json:"n32cURI"`
	N32fURI        string `json:"n32fURI,omitempty"`
	TrustAnchorPEM string `json:"trustAnchorPEM"`
	NRFAPIRoot     string `json:"nrfAPIRoot,omitempty"`
}

type roamingPLMNBody struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type roamingPartnerBody struct {
	ID             string                  `json:"id,omitempty"`
	Name           string                  `json:"name"`
	PLMNs          []roamingPLMNBody       `json:"plmns"`
	ProfileName    string                  `json:"profileName"`
	SEPP           *roamingPartnerSEPPBody `json:"sepp,omitempty"`
	DiameterPeerID string                  `json:"diameterPeerID,omitempty"`
}

type roamingPartnerEnvelope struct {
	Result roamingPartnerBody `json:"result"`
	Error  string             `json:"error,omitempty"`
}

type roamingPartnersEnvelope struct {
	Result struct {
		Items []roamingPartnerBody `json:"items"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type roamingDiameterPeerBody struct {
	ID               string `json:"id,omitempty"`
	DiameterIdentity string `json:"diameterIdentity,omitempty"`
	Address          string `json:"address"`
	Port             int    `json:"port,omitempty"`
}

type roamingDiameterPeerEnvelope struct {
	Result roamingDiameterPeerBody `json:"result"`
	Error  string                  `json:"error,omitempty"`
}

type roamingDiameterPeersEnvelope struct {
	Result struct {
		Items []roamingDiameterPeerBody `json:"items"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type seppCertificateBody struct {
	SEPPID              int      `json:"seppID"`
	Source              string   `json:"source"`
	FQDN                string   `json:"fqdn"`
	CertificateChainPEM string   `json:"certificateChainPEM"`
	TrustAnchorPEM      string   `json:"trustAnchorPEM"`
	Fingerprint         string   `json:"fingerprint"`
	NotAfter            string   `json:"notAfter"`
	Warnings            []string `json:"warnings"`
}

type seppCertificateEnvelope struct {
	Result seppCertificateBody `json:"result"`
	Error  string              `json:"error,omitempty"`
}

type roamingMessageEnvelope struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

const (
	roamingPartnersPath      = "/api/v1/operator/roaming/partners"
	roamingDiameterPeersPath = "/api/v1/operator/roaming/diameter-peers"
	seppCertificatePath      = "/api/v1/operator/roaming/sepp/nodes/1/certificate"
)

func selfSignedPEM(t *testing.T, notAfter time.Time) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "partner CA"},
		NotBefore:             notAfter.AddDate(-1, 0, 0),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return string(pki.EncodeCertPEM(cert))
}

func plmnList(pairs ...string) []roamingPLMNBody {
	out := make([]roamingPLMNBody, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, roamingPLMNBody{Mcc: pairs[i], Mnc: pairs[i+1]})
	}

	return out
}

func partnerSEPP(t *testing.T, mcc, mnc string) *roamingPartnerSEPPBody {
	t.Helper()

	fqdn := "sepp1." + pki.PLMN{MCC: mcc, MNC: mnc}.Domain()

	return &roamingPartnerSEPPBody{
		FQDN:           fqdn,
		N32cURI:        "https://" + fqdn + ":443",
		TrustAnchorPEM: selfSignedPEM(t, time.Now().AddDate(1, 0, 0)),
	}
}

func partner5G(t *testing.T, name string) roamingPartnerBody {
	t.Helper()

	return roamingPartnerBody{Name: name, PLMNs: plmnList("208", "93"), ProfileName: "default", SEPP: partnerSEPP(t, "208", "93")}
}

func TestRoamingPartners(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL

	if code, resp, err := apiDo[seppCertificateEnvelope](client, "GET", url+seppCertificatePath, token, nil); err != nil || code != http.StatusNotFound {
		t.Fatalf("GET SEPP certificate before any partner: code=%d err=%v (%q)", code, err, resp.Error)
	}

	code, peer, err := apiDo[roamingDiameterPeerEnvelope](client, "POST", url+roamingDiameterPeersPath, token, roamingDiameterPeerBody{DiameterIdentity: "dea.epc.mnc093.mcc208.3gppnetwork.org", Address: "192.0.2.10"})
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create Diameter peer: code=%d err=%v (%q)", code, err, peer.Error)
	}

	fourG := roamingPartnerBody{Name: "partner-4g", PLMNs: plmnList("310", "410"), ProfileName: "default", DiameterPeerID: peer.Result.ID}

	code, created4G, err := apiDo[roamingPartnerEnvelope](client, "POST", url+roamingPartnersPath, token, fourG)
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create 4G partner: code=%d err=%v (%q)", code, err, created4G.Error)
	}

	if code, resp, _ := apiDo[seppCertificateEnvelope](client, "GET", url+seppCertificatePath, token, nil); code != http.StatusNotFound {
		t.Fatalf("a 4G-only partner generated a SEPP certificate: code=%d (%+v)", code, resp.Result)
	}

	fiveG := partner5G(t, "partner-5g")

	code, created5G, err := apiDo[roamingPartnerEnvelope](client, "POST", url+roamingPartnersPath, token, fiveG)
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create 5G partner: code=%d err=%v (%q)", code, err, created5G.Error)
	}

	fiveG.ID = created5G.Result.ID
	if created5G.Result.ID == "" || created5G.Result.SEPP == nil || *created5G.Result.SEPP != *fiveG.SEPP || !slices.Equal(created5G.Result.PLMNs, fiveG.PLMNs) {
		t.Fatalf("created 5G partner = %+v, want %+v", created5G.Result, fiveG)
	}

	if code, _, _ := apiDo[seppCertificateEnvelope](client, "GET", url+seppCertificatePath, token, nil); code != http.StatusNotFound {
		t.Fatalf("a 5G partner generated a SEPP certificate: code=%d", code)
	}

	second := roamingPartnerBody{Name: "partner-5g-2", PLMNs: plmnList("234", "15"), ProfileName: "default", SEPP: partnerSEPP(t, "234", "15")}

	if code, resp, err := apiDo[roamingPartnerEnvelope](client, "POST", url+roamingPartnersPath, token, second); err != nil || code != http.StatusCreated {
		t.Fatalf("create second 5G partner: code=%d err=%v (%q)", code, err, resp.Error)
	}

	code, list, err := apiDo[roamingPartnersEnvelope](client, "GET", url+roamingPartnersPath, token, nil)
	if err != nil || code != http.StatusOK || len(list.Result.Items) != 3 {
		t.Fatalf("list partners: code=%d err=%v items=%+v", code, err, list.Result.Items)
	}

	if list.Result.Items[0].Name != "partner-4g" || list.Result.Items[0].DiameterPeerID != peer.Result.ID || list.Result.Items[0].SEPP != nil || list.Result.Items[0].ProfileName != "default" {
		t.Fatalf("listed 4G partner = %+v", list.Result.Items[0])
	}

	fiveG.Name = "partner-5g-renamed"
	fiveG.PLMNs = plmnList("208", "93", "208", "94")
	fiveG.SEPP.NRFAPIRoot = "http://nrf.5gc.mnc093.mcc208.3gppnetwork.org/"
	fiveG.DiameterPeerID = peer.Result.ID

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "PUT", url+roamingPartnersPath+"/"+fiveG.ID, token, fiveG); err != nil || code != http.StatusOK {
		t.Fatalf("update partner: code=%d err=%v (%q)", code, err, resp.Error)
	}

	code, got, err := apiDo[roamingPartnerEnvelope](client, "GET", url+roamingPartnersPath+"/"+fiveG.ID, token, nil)
	if err != nil || code != http.StatusOK {
		t.Fatalf("get partner: code=%d err=%v (%q)", code, err, got.Error)
	}

	if got.Result.Name != fiveG.Name || got.Result.SEPP.NRFAPIRoot != "http://nrf.5gc.mnc093.mcc208.3gppnetwork.org" || got.Result.DiameterPeerID != peer.Result.ID || !slices.Equal(got.Result.PLMNs, fiveG.PLMNs) {
		t.Fatalf("updated partner = %+v", got.Result)
	}

	ctx := context.Background()
	roamer := "208940000000001"

	server.SetRoamerWithdrawalTimeoutForTest(t, 50*time.Millisecond)

	if _, err := env.DB.RegisterVisitorUE(ctx, roamer, fiveG.ID, db.UERegistrationTypeAMF3GPPAccess, "node-1", ""); err != nil {
		t.Fatalf("RegisterVisitorUE: %s", err)
	}

	without5G := fiveG
	without5G.SEPP = nil

	withoutPLMN := fiveG
	withoutPLMN.PLMNs = plmnList("208", "93")

	for name, body := range map[string]roamingPartnerBody{"remove the 5G agreement": without5G, "remove the roamer's PLMN": withoutPLMN} {
		if code, resp, _ := apiDo[roamingMessageEnvelope](client, "PUT", url+roamingPartnersPath+"/"+fiveG.ID, token, body); code != http.StatusConflict || !strings.Contains(resp.Error, "roamers") {
			t.Fatalf("%s with an attached roamer: code=%d (%q), want 409", name, code, resp.Error)
		}
	}

	if code, resp, _ := apiDo[roamingMessageEnvelope](client, "DELETE", url+roamingPartnersPath+"/"+fiveG.ID, token, nil); code != http.StatusConflict || !strings.Contains(resp.Error, "roamers") {
		t.Fatalf("delete a partner with an attached roamer: code=%d (%q), want 409", code, resp.Error)
	}

	if p, err := env.DB.GetRoamingPartner(ctx, fiveG.ID); err != nil || len(p.Withdrawals) != 0 {
		t.Fatalf("withdrawals after a timed-out deletion = %+v, %v, want them cleared", p, err)
	}

	if err := env.DB.PurgeUERegistration(ctx, roamer, db.UERegistrationTypeAMF3GPPAccess, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	if _, err := env.DB.DeleteVisitorIfUnregistered(ctx, roamer); err != nil {
		t.Fatalf("DeleteVisitorIfUnregistered: %s", err)
	}

	if code, resp, _ := apiDo[roamingMessageEnvelope](client, "DELETE", url+roamingDiameterPeersPath+"/"+peer.Result.ID, token, nil); code != http.StatusConflict {
		t.Fatalf("delete a Diameter peer in use: code=%d (%q), want 409", code, resp.Error)
	}

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "DELETE", url+roamingPartnersPath+"/"+fiveG.ID, token, nil); err != nil || code != http.StatusOK {
		t.Fatalf("delete partner: code=%d err=%v (%q)", code, err, resp.Error)
	}

	for _, method := range []string{"GET", "DELETE"} {
		if code, _, _ := apiDo[roamingPartnerEnvelope](client, method, url+roamingPartnersPath+"/"+fiveG.ID, token, nil); code != http.StatusNotFound {
			t.Fatalf("%s a deleted partner: code=%d, want 404", method, code)
		}
	}

	if code, _, _ := apiDo[roamingMessageEnvelope](client, "PUT", url+roamingPartnersPath+"/"+fiveG.ID, token, fiveG); code != http.StatusNotFound {
		t.Fatalf("PUT a deleted partner: code=%d, want 404", code)
	}

	code, logs, err := listAuditLogs(url, client, token, 1, 50)
	if err != nil || code != http.StatusOK {
		t.Fatalf("list audit logs: code=%d err=%v", code, err)
	}

	actions := map[string]int{}
	for _, l := range logs.Result.Items {
		actions[l.Action]++
	}

	want := map[string]int{
		"create_roaming_diameter_peer": 1,
		"create_roaming_partner":       3,
		"update_roaming_partner":       1,
		"delete_roaming_partner":       1,
	}

	for action, n := range want {
		if actions[action] != n {
			t.Errorf("audit action %s logged %d times, want %d (%v)", action, actions[action], n, actions)
		}
	}
}

func TestRoamingPartnerValidation(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL

	valid := func() roamingPartnerBody {
		return partner5G(t, "partner")
	}

	if code, resp, err := apiDo[roamingPartnerEnvelope](client, "POST", url+roamingPartnersPath, token, valid()); err != nil || code != http.StatusCreated {
		t.Fatalf("create partner: code=%d err=%v (%q)", code, err, resp.Error)
	}

	cases := []struct {
		name   string
		mutate func(*roamingPartnerBody)
		want   int
	}{
		{"missing name", func(p *roamingPartnerBody) { p.Name = " " }, http.StatusBadRequest},
		{"no PLMN", func(p *roamingPartnerBody) { p.PLMNs = nil }, http.StatusBadRequest},
		{"bad MCC", func(p *roamingPartnerBody) { p.PLMNs[0].Mcc = "20" }, http.StatusBadRequest},
		{"bad MNC", func(p *roamingPartnerBody) { p.PLMNs[0].Mnc = "9a" }, http.StatusBadRequest},
		{"PLMN listed twice", func(p *roamingPartnerBody) { p.PLMNs = plmnList("208", "93", "208", "093") }, http.StatusBadRequest},
		{"well-known SEPP FQDN", func(p *roamingPartnerBody) { p.SEPP.FQDN = "sepp.5gc.mnc093.mcc208.3gppnetwork.org" }, http.StatusBadRequest},
		{"SEPP FQDN of another PLMN", func(p *roamingPartnerBody) { p.SEPP.FQDN = "sepp1.5gc.mnc095.mcc208.3gppnetwork.org" }, http.StatusBadRequest},
		{"non-3GPP SEPP FQDN", func(p *roamingPartnerBody) { p.SEPP.FQDN = "sepp.example.org" }, http.StatusBadRequest},
		{"N32-c URI with a bad port", func(p *roamingPartnerBody) { p.SEPP.N32cURI = "https://sepp.example.org:0" }, http.StatusBadRequest},
		{"CA-issued leaf as trust anchor", func(p *roamingPartnerBody) { p.SEPP.TrustAnchorPEM = leafPEM(t) }, http.StatusBadRequest},
		{"missing profile", func(p *roamingPartnerBody) { p.ProfileName = "" }, http.StatusBadRequest},
		{"unknown profile", func(p *roamingPartnerBody) { p.ProfileName = "nope" }, http.StatusNotFound},
		{"unknown Diameter peer", func(p *roamingPartnerBody) { p.DiameterPeerID = "01890000-0000-7000-8000-0000000000ff" }, http.StatusNotFound},
		{"SEPP without trust anchor", func(p *roamingPartnerBody) { p.SEPP.TrustAnchorPEM = "" }, http.StatusBadRequest},
		{"SEPP without N32-c URI", func(p *roamingPartnerBody) { p.SEPP.N32cURI = "" }, http.StatusBadRequest},
		{"N32-c URI over http", func(p *roamingPartnerBody) { p.SEPP.N32cURI = "http://sepp.example.org" }, http.StatusBadRequest},
		{"N32-c URI with a query", func(p *roamingPartnerBody) { p.SEPP.N32cURI = "https://sepp.example.org?a=b" }, http.StatusBadRequest},
		{"invalid SEPP FQDN", func(p *roamingPartnerBody) { p.SEPP.FQDN = "not a name" }, http.StatusBadRequest},
		{"expired trust anchor", func(p *roamingPartnerBody) { p.SEPP.TrustAnchorPEM = selfSignedPEM(t, time.Now().Add(-time.Hour)) }, http.StatusBadRequest},
		{"trust anchor with a key", func(p *roamingPartnerBody) {
			p.SEPP.TrustAnchorPEM += "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"
		}, http.StatusBadRequest},
		{"trust anchor garbage", func(p *roamingPartnerBody) { p.SEPP.TrustAnchorPEM = "garbage" }, http.StatusBadRequest},
		{"duplicate name", func(p *roamingPartnerBody) { p.PLMNs, p.SEPP = plmnList("310", "410"), nil }, http.StatusConflict},
		{"duplicate PLMN", func(p *roamingPartnerBody) { p.Name = "other" }, http.StatusConflict},
		{"zero-padded duplicate PLMN", func(p *roamingPartnerBody) { p.Name, p.PLMNs = "other", plmnList("208", "093") }, http.StatusConflict},
		{"ambiguous MNC", func(p *roamingPartnerBody) { p.Name, p.PLMNs, p.SEPP = "other", plmnList("208", "930"), nil }, http.StatusConflict},
		{"home PLMN", func(p *roamingPartnerBody) { p.Name, p.PLMNs, p.SEPP = "other", plmnList("001", "01"), nil }, http.StatusConflict},
		{"ambiguous with the home PLMN", func(p *roamingPartnerBody) { p.Name, p.PLMNs, p.SEPP = "other", plmnList("001", "012"), nil }, http.StatusConflict},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := valid()
			tc.mutate(&p)

			code, resp, err := apiDo[roamingPartnerEnvelope](client, "POST", url+roamingPartnersPath, token, p)
			if err != nil || code != tc.want {
				t.Fatalf("code=%d err=%v (%q), want %d", code, err, resp.Error, tc.want)
			}

			if resp.Error == "" {
				t.Fatal("no error message")
			}
		})
	}

	_, list, _ := apiDo[roamingPartnersEnvelope](client, "GET", url+roamingPartnersPath, token, nil)
	if len(list.Result.Items) != 1 {
		t.Fatalf("rejected requests created partners: %+v", list.Result.Items)
	}

	p := valid()
	p.PLMNs = plmnList("208", "93", "208", "94")

	if code, resp, _ := apiDo[roamingMessageEnvelope](client, "PUT", url+roamingPartnersPath+"/"+list.Result.Items[0].ID, token, p); code != http.StatusOK {
		t.Fatalf("add a PLMN: code=%d (%q), want 200", code, resp.Error)
	}

	if code, resp, _ := apiDo[roamingMessageEnvelope](client, "PUT", url+"/api/v1/operator/id", token, map[string]string{"mcc": "002", "mnc": "02"}); code != http.StatusConflict || !strings.Contains(resp.Error, "roaming partners") {
		t.Fatalf("change the operator PLMN with partners: code=%d (%q), want 409", code, resp.Error)
	}

	if code, resp, _ := apiDo[roamingMessageEnvelope](client, "DELETE", url+"/api/v1/profiles/default", token, nil); code != http.StatusConflict {
		t.Fatalf("delete the profile of a partner: code=%d (%q), want 409", code, resp.Error)
	}
}

func leafPEM(t *testing.T) string {
	t.Helper()

	leaf, _, _, err := pki.GenerateSEPPCert("sepp1.5gc.mnc093.mcc208.3gppnetwork.org", pki.PLMN{MCC: "208", MNC: "93"})
	if err != nil {
		t.Fatal(err)
	}

	return string(pki.EncodeCertPEM(leaf))
}

func TestRoamingDiameterPeers(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL

	code, created, err := apiDo[roamingDiameterPeerEnvelope](client, "POST", url+roamingDiameterPeersPath, token, roamingDiameterPeerBody{Address: "::ffff:192.0.2.1"})
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create: code=%d err=%v (%q)", code, err, created.Error)
	}

	if created.Result.Address != "192.0.2.1" || created.Result.Port != 3868 {
		t.Fatalf("created = %+v, want the canonical address and the default port", created.Result)
	}

	for _, body := range []roamingDiameterPeerBody{
		{Address: "hss.example.org"},
		{Address: "0.0.0.0"},
		{Address: "192.0.2.2", Port: 70000},
		{Address: "192.0.2.2", DiameterIdentity: "not a name"},
	} {
		if code, resp, _ := apiDo[roamingDiameterPeerEnvelope](client, "POST", url+roamingDiameterPeersPath, token, body); code != http.StatusBadRequest {
			t.Fatalf("create %+v: code=%d (%q), want 400", body, code, resp.Error)
		}
	}

	if code, resp, _ := apiDo[roamingDiameterPeerEnvelope](client, "POST", url+roamingDiameterPeersPath, token, roamingDiameterPeerBody{Address: "192.0.2.1"}); code != http.StatusConflict {
		t.Fatalf("create a duplicate address: code=%d (%q), want 409", code, resp.Error)
	}

	if code, resp, _ := apiDo[roamingMessageEnvelope](client, "PUT", url+roamingDiameterPeersPath+"/"+created.Result.ID, token, roamingDiameterPeerBody{Address: "192.0.2.3"}); code != http.StatusBadRequest {
		t.Fatalf("update without a port: code=%d (%q), want 400", code, resp.Error)
	}

	update := roamingDiameterPeerBody{DiameterIdentity: "hss.epc.mnc093.mcc208.3gppnetwork.org", Address: "192.0.2.3", Port: 3869}

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "PUT", url+roamingDiameterPeersPath+"/"+created.Result.ID, token, update); err != nil || code != http.StatusOK {
		t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
	}

	code, list, err := apiDo[roamingDiameterPeersEnvelope](client, "GET", url+roamingDiameterPeersPath, token, nil)
	if err != nil || code != http.StatusOK {
		t.Fatalf("list: code=%d err=%v", code, err)
	}

	update.ID = created.Result.ID
	if !slices.Equal(list.Result.Items, []roamingDiameterPeerBody{update}) {
		t.Fatalf("list = %+v, want [%+v]", list.Result.Items, update)
	}

	if code, _, _ := apiDo[roamingMessageEnvelope](client, "PUT", url+roamingDiameterPeersPath+"/01890000-0000-7000-8000-0000000000ff", token, update); code != http.StatusNotFound {
		t.Fatalf("update a missing peer: code=%d, want 404", code)
	}

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "DELETE", url+roamingDiameterPeersPath+"/"+created.Result.ID, token, nil); err != nil || code != http.StatusOK {
		t.Fatalf("delete: code=%d err=%v (%q)", code, err, resp.Error)
	}

	if code, _, _ := apiDo[roamingDiameterPeerEnvelope](client, "GET", url+roamingDiameterPeersPath+"/"+created.Result.ID, token, nil); code != http.StatusNotFound {
		t.Fatalf("get a deleted peer: code=%d, want 404", code)
	}
}

func TestSEPPCertificate(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL
	home := pki.PLMN{MCC: "001", MNC: "01"}

	code, generated, err := apiDo[seppCertificateEnvelope](client, "POST", url+seppCertificatePath+"/generate", token, nil)
	if err != nil || code != http.StatusCreated {
		t.Fatalf("generate: code=%d err=%v (%q)", code, err, generated.Error)
	}

	if generated.Result.FQDN != homeSEPPFQDN || generated.Result.SEPPID != 1 || generated.Result.Source != "generated" || !slices.Contains(generated.Result.Warnings, pki.SEPPWarningNoCRLDistributor) || len(generated.Result.Warnings) != 2 {
		t.Fatalf("generated = %+v", generated.Result)
	}

	chain, err := pki.ParseCertChainPEM([]byte(generated.Result.CertificateChainPEM))
	if err != nil || len(chain) != 2 {
		t.Fatalf("generated chain: %d certificates, %v", len(chain), err)
	}

	if string(pki.EncodeCertPEM(chain[1])) != generated.Result.TrustAnchorPEM || pki.Fingerprint(chain[0]) != generated.Result.Fingerprint {
		t.Fatal("trust anchor or fingerprint does not match the chain")
	}

	if notAfter, err := time.Parse(time.RFC3339, generated.Result.NotAfter); err != nil || !notAfter.Equal(chain[0].NotAfter) {
		t.Fatalf("notAfter = %q, want %s", generated.Result.NotAfter, chain[0].NotAfter)
	}

	custom := "sepp2.5gc.mnc001.mcc001.3gppnetwork.org"
	sepp2Path := "/api/v1/operator/roaming/sepp/nodes/2/certificate"

	code, second, err := apiDo[seppCertificateEnvelope](client, "POST", url+sepp2Path+"/generate", token, nil)
	if err != nil || code != http.StatusCreated || second.Result.FQDN != custom || second.Result.TrustAnchorPEM != generated.Result.TrustAnchorPEM {
		t.Fatalf("generate for SEPP 2: code=%d err=%v (%q) result=%+v", code, err, second.Error, second.Result)
	}

	for _, path := range []string{"/api/v1/operator/roaming/sepp/nodes/0/certificate", "/api/v1/operator/roaming/sepp/nodes/64/certificate", "/api/v1/operator/roaming/sepp/nodes/x/certificate"} {
		if code, _, _ := apiDo[seppCertificateEnvelope](client, "POST", url+path+"/generate", token, nil); code != http.StatusBadRequest {
			t.Fatalf("generate for %s: code=%d, want 400", path, code)
		}
	}

	code, regenerated, err := apiDo[struct {
		Result struct {
			TrustAnchor struct {
				PEM     string `json:"pem"`
				SEPPIDs []int  `json:"seppIDs"`
			} `json:"trustAnchor"`
			Reissued []int `json:"reissued"`
		} `json:"result"`
	}](client, "POST", url+"/api/v1/operator/roaming/sepp/ca/regenerate", token, nil)
	if err != nil || code != http.StatusCreated || !slices.Equal(regenerated.Result.Reissued, []int{1, 2}) || regenerated.Result.TrustAnchor.PEM == generated.Result.TrustAnchorPEM {
		t.Fatalf("regenerate the CA: code=%d err=%v result=%+v", code, err, regenerated.Result)
	}

	if code, got, _ := apiDo[seppCertificateEnvelope](client, "GET", url+sepp2Path, token, nil); code != http.StatusOK || got.Result.TrustAnchorPEM != regenerated.Result.TrustAnchor.PEM {
		t.Fatalf("SEPP 2 after a CA regeneration: code=%d result=%+v", code, got.Result)
	}

	leaf, key, ca, err := pki.GenerateSEPPCert(homeSEPPFQDN, home)
	if err != nil {
		t.Fatal(err)
	}

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		t.Fatal(err)
	}

	importBody := map[string]string{
		"certificateChainPEM": string(pki.EncodeCertPEM(leaf)) + string(pki.EncodeCertPEM(ca)),
		"privateKeyPEM":       string(keyPEM),
	}

	code, imported, err := apiDo[seppCertificateEnvelope](client, "PUT", url+seppCertificatePath, token, importBody)
	if err != nil || code != http.StatusOK {
		t.Fatalf("import: code=%d err=%v (%q)", code, err, imported.Error)
	}

	if imported.Result.FQDN != homeSEPPFQDN || imported.Result.Fingerprint != pki.Fingerprint(leaf) || imported.Result.TrustAnchorPEM != string(pki.EncodeCertPEM(ca)) || imported.Result.Source != "imported" || !slices.Equal(imported.Result.Warnings, []string{pki.SEPPWarningNoCRLDistributor, "The trust anchor of this node changed: every roaming partner must import the new trust anchor before N32 with this node works again"}) {
		t.Fatalf("imported = %+v", imported.Result)
	}

	if code, again, _ := apiDo[seppCertificateEnvelope](client, "PUT", url+seppCertificatePath, token, importBody); code != http.StatusOK || !slices.Equal(again.Result.Warnings, []string{pki.SEPPWarningNoCRLDistributor}) {
		t.Fatalf("re-import of the same trust anchor: code=%d warnings=%v", code, again.Result.Warnings)
	}

	stored, err := env.DB.GetSEPPIdentity(context.Background(), 1)
	if err != nil || stored.KeyPEM != string(keyPEM) || stored.FQDN != homeSEPPFQDN {
		t.Fatalf("stored identity does not hold the imported key: %v", err)
	}

	code, exported, err := apiDo[map[string]any](client, "GET", url+seppCertificatePath, token, nil)
	if err != nil || code != http.StatusOK {
		t.Fatalf("export: code=%d err=%v", code, err)
	}

	for _, v := range (*exported)["result"].(map[string]any) {
		if s, ok := v.(string); ok && strings.Contains(s, "PRIVATE KEY") {
			t.Fatal("the export holds the private key")
		}
	}

	otherKey, err := pki.EncodePrivateKeyPEM(mustP256(t))
	if err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]map[string]string{
		"mismatched key":    {"certificateChainPEM": importBody["certificateChainPEM"], "privateKeyPEM": string(otherKey)},
		"no key":            {"certificateChainPEM": importBody["certificateChainPEM"]},
		"no chain":          {"privateKeyPEM": string(keyPEM)},
		"wrong chain order": {"certificateChainPEM": string(pki.EncodeCertPEM(ca)) + string(pki.EncodeCertPEM(leaf)), "privateKeyPEM": string(keyPEM)},
	} {
		if code, resp, _ := apiDo[seppCertificateEnvelope](client, "PUT", url+seppCertificatePath, token, body); code != http.StatusBadRequest {
			t.Fatalf("import with %s: code=%d (%q), want 400", name, code, resp.Error)
		}
	}

	foreignLeaf, foreignKey, foreignCA, err := pki.GenerateSEPPCert("sepp1.5gc.mnc093.mcc208.3gppnetwork.org", pki.PLMN{MCC: "208", MNC: "93"})
	if err != nil {
		t.Fatal(err)
	}

	foreignKeyPEM, err := pki.EncodePrivateKeyPEM(foreignKey)
	if err != nil {
		t.Fatal(err)
	}

	if code, resp, _ := apiDo[seppCertificateEnvelope](client, "PUT", url+seppCertificatePath, token, map[string]string{
		"certificateChainPEM": string(pki.EncodeCertPEM(foreignLeaf)) + string(pki.EncodeCertPEM(foreignCA)),
		"privateKeyPEM":       string(foreignKeyPEM),
	}); code != http.StatusBadRequest || !strings.Contains(resp.Error, "another PLMN") {
		t.Fatalf("import a certificate of another PLMN: code=%d (%q), want 400", code, resp.Error)
	}

	longKey := mustP256(t)
	longTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7),
		Subject:      pkix.Name{CommonName: homeSEPPFQDN},
		DNSNames:     []string{homeSEPPFQDN},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	longDER, err := x509.CreateCertificate(rand.Reader, longTmpl, longTmpl, &longKey.PublicKey, longKey)
	if err != nil {
		t.Fatal(err)
	}

	longKeyPEM, err := pki.EncodePrivateKeyPEM(longKey)
	if err != nil {
		t.Fatal(err)
	}

	if code, resp, _ := apiDo[seppCertificateEnvelope](client, "PUT", url+seppCertificatePath, token, map[string]string{
		"certificateChainPEM": string(pki.EncodeCertPEM(&x509.Certificate{Raw: longDER})),
		"privateKeyPEM":       string(longKeyPEM),
	}); code != http.StatusBadRequest || !strings.Contains(resp.Error, "CA certificate") {
		t.Fatalf("import a self-signed leaf without a CA: code=%d (%q), want 400", code, resp.Error)
	}

	caKey := mustP256(t)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(8),
		Subject:               pkix.Name{CommonName: "operator CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(20, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	operatorCA, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	longTmpl.DNSNames = []string{homeSEPPFQDN, custom}
	longTmpl.Subject.CommonName = ""

	issuedDER, err := x509.CreateCertificate(rand.Reader, longTmpl, operatorCA, &longKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	issuedChain := string(pki.EncodeCertPEM(&x509.Certificate{Raw: issuedDER})) + string(pki.EncodeCertPEM(operatorCA))

	if code, resp, _ := apiDo[seppCertificateEnvelope](client, "PUT", url+seppCertificatePath, token, map[string]string{
		"certificateChainPEM": issuedChain,
		"privateKeyPEM":       string(longKeyPEM),
	}); code != http.StatusBadRequest || !strings.Contains(resp.Error, "give the fqdn") {
		t.Fatalf("import a certificate with two SEPP names and no fqdn: code=%d (%q), want 400", code, resp.Error)
	}

	if code, resp, _ := apiDo[seppCertificateEnvelope](client, "PUT", url+seppCertificatePath, token, map[string]string{
		"certificateChainPEM": issuedChain,
		"privateKeyPEM":       string(longKeyPEM),
		"fqdn":                custom,
	}); code != http.StatusConflict {
		t.Fatalf("import the FQDN of SEPP 2 for SEPP 1: code=%d (%q), want 409", code, resp.Error)
	}

	code, long, err := apiDo[seppCertificateEnvelope](client, "PUT", url+sepp2Path, token, map[string]string{
		"certificateChainPEM": issuedChain,
		"privateKeyPEM":       string(longKeyPEM),
		"fqdn":                custom,
	})
	if err != nil || code != http.StatusOK || long.Result.FQDN != custom || !slices.Contains(long.Result.Warnings, pki.SEPPWarningLongValidity) || !slices.Contains(long.Result.Warnings, pki.SEPPWarningNoEKU) {
		t.Fatalf("import a 10-year CA-issued certificate: code=%d err=%v (%q) result=%+v", code, err, long.Error, long.Result)
	}

	if long.Result.TrustAnchorPEM != string(pki.EncodeCertPEM(operatorCA)) {
		t.Fatal("the trust anchor is not the operator CA")
	}

	if code, got, _ := apiDo[seppCertificateEnvelope](client, "GET", url+sepp2Path, token, nil); code != http.StatusOK || got.Result.FQDN != custom || got.Result.Source != "imported" || !slices.Contains(got.Result.Warnings, pki.SEPPWarningLongValidity) {
		t.Fatalf("GET after import keeps the selected FQDN and the profile warnings: code=%d result=%+v", code, got.Result)
	}

	code, changed, err := apiDo[roamingMessageEnvelope](client, "PUT", url+"/api/v1/operator/id", token, map[string]string{"mcc": "002", "mnc": "02"})
	if err != nil || code != http.StatusCreated || !strings.Contains(changed.Result.Message, "SEPP certificate") {
		t.Fatalf("change the operator PLMN: code=%d err=%v (%q)", code, err, changed.Result.Message)
	}

	if code, _, _ := apiDo[seppCertificateEnvelope](client, "GET", url+seppCertificatePath, token, nil); code != http.StatusNotFound {
		t.Fatalf("GET SEPP certificate after a PLMN change: code=%d, want 404", code)
	}
}

func mustP256(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	return key
}

func TestRoamingPermissions(t *testing.T) {
	env, admin, adminToken := newAuthedTestEnv(t)
	url := env.Server.URL

	readOnly := newTestClient(env.Server)

	readOnlyToken, err := createUserAndLogin(url, adminToken, "roaming-ro@ellanetworks.com", RoleReadOnly, readOnly)
	if err != nil {
		t.Fatalf("create read-only user: %s", err)
	}

	manager := newTestClient(env.Server)

	managerToken, err := createUserAndLogin(url, adminToken, "roaming-nm@ellanetworks.com", RoleNetworkManager, manager)
	if err != nil {
		t.Fatalf("create network manager: %s", err)
	}

	if code, _, err := apiDo[roamingDiameterPeerEnvelope](admin, "POST", url+roamingDiameterPeersPath, adminToken, roamingDiameterPeerBody{Address: "192.0.2.1"}); err != nil || code != http.StatusCreated {
		t.Fatalf("admin create Diameter peer: code=%d err=%v", code, err)
	}

	for _, path := range []string{roamingPartnersPath, roamingDiameterPeersPath, "/api/v1/operator/roaming/sepp"} {
		if code, _, err := apiDo[map[string]any](readOnly, "GET", url+path, readOnlyToken, nil); err != nil || code != http.StatusOK {
			t.Fatalf("read-only GET %s: code=%d err=%v", path, code, err)
		}
	}

	writes := []struct {
		method, path string
		body         any
	}{
		{"POST", roamingPartnersPath, partner5G(t, "partner")},
		{"POST", roamingDiameterPeersPath, roamingDiameterPeerBody{Address: "192.0.2.2"}},
		{"POST", seppCertificatePath + "/generate", map[string]string{}},
	}

	for _, w := range writes {
		if code, _, err := apiDo[map[string]any](readOnly, w.method, url+w.path, readOnlyToken, w.body); err != nil || code != http.StatusForbidden {
			t.Fatalf("read-only %s %s: code=%d err=%v, want 403", w.method, w.path, code, err)
		}
	}

	for _, path := range []string{roamingPartnersPath + "/x", roamingDiameterPeersPath + "/x"} {
		for _, method := range []string{"PUT", "DELETE"} {
			if code, _, err := apiDo[map[string]any](readOnly, method, url+path, readOnlyToken, map[string]string{}); err != nil || code != http.StatusForbidden {
				t.Fatalf("read-only %s %s: code=%d err=%v, want 403", method, path, code, err)
			}
		}
	}

	if code, _, err := apiDo[map[string]any](readOnly, "PUT", url+seppCertificatePath, readOnlyToken, map[string]string{}); err != nil || code != http.StatusForbidden {
		t.Fatalf("read-only import SEPP certificate: code=%d err=%v, want 403", code, err)
	}

	for _, w := range writes {
		if code, resp, err := apiDo[map[string]any](manager, w.method, url+w.path, managerToken, w.body); err != nil || code != http.StatusCreated {
			t.Fatalf("network manager %s %s: code=%d err=%v (%v)", w.method, w.path, code, err, resp)
		}
	}

	if code, _, err := apiDo[map[string]any](readOnly, "GET", url+seppCertificatePath, readOnlyToken, nil); err != nil || code != http.StatusOK {
		t.Fatalf("read-only GET SEPP certificate: code=%d err=%v", code, err)
	}
}
