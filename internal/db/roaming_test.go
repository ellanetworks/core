// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/supportbundle"
)

const (
	roamingPartnerID = "01890000-0000-7000-8000-0000000000a1"
	roamingPeerID    = "01890000-0000-7000-8000-0000000000b1"
	visitorIMSI      = "208930000000001"
)

type roamingFixture struct {
	t         *testing.T
	db        *db.Database
	profileID string
	otherID   string
}

func newRoamingFixture(t *testing.T) *roamingFixture {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	if err := database.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %s", err)
	}

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	other := &db.Profile{Name: "roamers", UeAmbrUplink: "10 Mbps", UeAmbrDownlink: "10 Mbps", Allow4G: true, Allow5G: true}
	if err := database.CreateProfile(ctx, other); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	createdOther, err := database.GetProfile(ctx, other.Name)
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	return &roamingFixture{t: t, db: database, profileID: profile.ID, otherID: createdOther.ID}
}

func (f *roamingFixture) createPeer(t *testing.T) {
	t.Helper()

	if err := f.db.CreateRoamingDiameterPeer(context.Background(), &db.RoamingDiameterPeer{ID: roamingPeerID, DiameterIdentity: "dea.example.org", Address: "10.0.0.7", Port: 3868}); err != nil {
		t.Fatalf("CreateRoamingDiameterPeer: %s", err)
	}
}

func plmns(pairs ...string) []db.RoamingPLMN {
	out := make([]db.RoamingPLMN, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, db.RoamingPLMN{Mcc: pairs[i], Mnc: pairs[i+1]})
	}

	return out
}

func (f *roamingFixture) partner() *db.RoamingPartner {
	return &db.RoamingPartner{ID: roamingPartnerID, Name: "partner", PLMNs: plmns("208", "93"), ProfileID: f.profileID}
}

func (f *roamingFixture) newPartner(id, name string, pairs ...string) *db.RoamingPartner {
	return &db.RoamingPartner{ID: id, Name: name, PLMNs: plmns(pairs...), ProfileID: f.profileID}
}

func seppIdentity(t *testing.T, plmn pki.PLMN, fqdn string, seppID ...int) db.SEPPIdentity {
	t.Helper()

	leaf, key, ca, err := pki.GenerateSEPPCert(fqdn, plmn)
	if err != nil {
		t.Fatalf("GenerateSEPPCert: %s", err)
	}

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		t.Fatalf("EncodePrivateKeyPEM: %s", err)
	}

	id := 1
	if len(seppID) > 0 {
		id = seppID[0]
	}

	return db.SEPPIdentity{SEPPID: id, FQDN: fqdn, Source: db.SEPPIdentityImported, CertChainPEM: string(pki.EncodeCertPEM(leaf)) + string(pki.EncodeCertPEM(ca)), KeyPEM: string(keyPEM)}
}

func ownSEPPIdentity(t *testing.T, id string) db.SEPPIdentity {
	t.Helper()

	own := pki.PLMN{MCC: db.InitialMcc, MNC: db.InitialMnc}

	n, err := strconv.Atoi(id)
	if err != nil {
		t.Fatalf("SEPP ID %q: %s", id, err)
	}

	return seppIdentity(t, own, "sepp"+id+"."+own.Domain(), n)
}

func (f *roamingFixture) with5G(p *db.RoamingPartner, fqdn string) *db.RoamingPartner {
	anchor := selfSignedPEM(f.t)
	p.SEPPFQDN = ptr(fqdn)
	p.SEPPN32cURI = ptr("https://" + fqdn + ":443")
	p.SEPPTrustAnchorPEM = &anchor

	return p
}

func (f *roamingFixture) createPartner(t *testing.T) {
	t.Helper()

	if err := f.db.CreateRoamingPartner(context.Background(), f.partner()); err != nil {
		t.Fatalf("CreateRoamingPartner: %s", err)
	}
}

func (f *roamingFixture) registerVisitor(t *testing.T, imsi string) {
	t.Helper()

	if _, err := f.db.RegisterVisitorUE(context.Background(), imsi, roamingPartnerID, db.UERegistrationTypeMME, "node-1", ""); err != nil {
		t.Fatalf("RegisterVisitorUE: %s", err)
	}
}

func (f *roamingFixture) createOwnSubscriber(t *testing.T, imsi string) {
	t.Helper()

	sub := &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      f.profileID,
		Msisdn:         "15550001",
	}
	if err := f.db.CreateSubscriber(context.Background(), sub); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}
}

func selfSignedPEM(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %s", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "sepp.example.org"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %s", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

const partnerSEPPFQDN = "sepp1.5gc.mnc093.mcc208.3gppnetwork.org"

func TestRoamingPartnerRoundTrip(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPeer(t)

	p := f.with5G(f.newPartner(roamingPartnerID, "partner", "208", "94", "208", "93"), partnerSEPPFQDN)
	p.NRFAPIRoots = []db.RoamingNRFAPIRoot{{RoamingPLMN: db.RoamingPLMN{Mcc: "208", Mnc: "93"}, APIRoot: "http://nrf.5gc.mnc093.mcc208.3gppnetwork.org"}}
	p.DiameterPeerID = ptr(roamingPeerID)

	if err := f.db.CreateRoamingPartner(ctx, p); err != nil {
		t.Fatalf("CreateRoamingPartner: %s", err)
	}

	got, err := f.db.GetRoamingPartner(ctx, roamingPartnerID)
	if err != nil {
		t.Fatalf("GetRoamingPartner: %s", err)
	}

	if got.Name != "partner" || !slices.Equal(got.PLMNs, plmns("208", "93", "208", "94")) || *got.SEPPFQDN != *p.SEPPFQDN || got.SEPPN32fURI != nil || *got.DiameterPeerID != roamingPeerID || !slices.Equal(got.NRFAPIRoots, p.NRFAPIRoots) {
		t.Fatalf("partner = %+v", got)
	}

	got.NRFAPIRoots = []db.RoamingNRFAPIRoot{{RoamingPLMN: db.RoamingPLMN{Mcc: "208", Mnc: "94"}, APIRoot: "https://nrf.example.org"}}

	if err := f.db.UpdateRoamingPartner(ctx, got); err != nil {
		t.Fatalf("UpdateRoamingPartner of the NRF API roots: %s", err)
	}

	moved, err := f.db.GetRoamingPartner(ctx, roamingPartnerID)
	if err != nil {
		t.Fatalf("GetRoamingPartner: %s", err)
	}

	if !slices.Equal(moved.NRFAPIRoots, got.NRFAPIRoots) || moved.NRFAPIRootOf(db.RoamingPLMN{Mcc: "208", Mnc: "93"}) != nil {
		t.Fatalf("NRF API roots = %+v, want %+v", moved.NRFAPIRoots, got.NRFAPIRoots)
	}

	got.Name = "renamed"
	got.PLMNs = plmns("208", "93", "209", "01")
	got.SEPPFQDN, got.SEPPN32cURI, got.SEPPTrustAnchorPEM, got.NRFAPIRoots = nil, nil, nil, nil

	if err := f.db.UpdateRoamingPartner(ctx, got); err != nil {
		t.Fatalf("UpdateRoamingPartner: %s", err)
	}

	partners, err := f.db.ListRoamingPartners(ctx)
	if err != nil {
		t.Fatalf("ListRoamingPartners: %s", err)
	}

	if len(partners) != 1 || partners[0].Name != "renamed" || partners[0].SEPPFQDN != nil || !slices.Equal(partners[0].PLMNs, plmns("208", "93", "209", "01")) {
		t.Fatalf("partners = %+v", partners)
	}

	if err := f.db.CreateRoamingPartner(ctx, f.newPartner("p2", "p2", "208", "94")); err != nil {
		t.Fatalf("PLMN freed by an update: %s", err)
	}

	if err := f.db.DeleteRoamingPartner(ctx, roamingPartnerID); err != nil {
		t.Fatalf("DeleteRoamingPartner: %s", err)
	}

	if _, err := f.db.GetRoamingPartner(ctx, roamingPartnerID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("GetRoamingPartner after delete: err = %v, want ErrNotFound", err)
	}

	if err := f.db.CreateRoamingPartner(ctx, f.newPartner("p3", "p3", "209", "01")); err != nil {
		t.Fatalf("PLMN freed by a delete: %s", err)
	}

	if err := f.db.DeleteRoamingPartner(ctx, roamingPartnerID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("second DeleteRoamingPartner: err = %v, want ErrNotFound", err)
	}

	if err := f.db.UpdateRoamingPartner(ctx, f.partner()); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("UpdateRoamingPartner of a missing partner: err = %v, want ErrNotFound", err)
	}
}

func TestRoamingPartnerValidate(t *testing.T) {
	anchor := selfSignedPEM(t)

	valid5G := func() db.RoamingPartner {
		return db.RoamingPartner{
			ID: "id", Name: "p", PLMNs: plmns("208", "93", "208", "94"), ProfileID: "pr",
			SEPPFQDN: ptr(partnerSEPPFQDN), SEPPN32cURI: ptr("https://sepp.example.org/prefix"), SEPPTrustAnchorPEM: &anchor,
		}
	}

	if err := valid5G().Validate(); err != nil {
		t.Fatalf("valid 5G partner rejected: %s", err)
	}

	if err := (db.RoamingPartner{ID: "id", Name: "p", PLMNs: plmns("208", "930"), ProfileID: "pr", DiameterPeerID: ptr("peer")}).Validate(); err != nil {
		t.Fatalf("valid 4G partner rejected: %s", err)
	}

	ipx := valid5G()
	ipx.SEPPFQDN = ptr("sepp1.5gc.mnc094.mcc208.ipxa.ipxnetwork.org")

	if err := ipx.Validate(); err != nil {
		t.Fatalf("partner with an IPX SEPP FQDN rejected: %s", err)
	}

	cases := map[string]func(p *db.RoamingPartner){
		"no id":           func(p *db.RoamingPartner) { p.ID = "" },
		"no name":         func(p *db.RoamingPartner) { p.Name = " " },
		"no PLMN":         func(p *db.RoamingPartner) { p.PLMNs = nil },
		"short mcc":       func(p *db.RoamingPartner) { p.PLMNs[1].Mcc = "20" },
		"letter mcc":      func(p *db.RoamingPartner) { p.PLMNs[1].Mcc = "20a" },
		"one digit mnc":   func(p *db.RoamingPartner) { p.PLMNs[1].Mnc = "9" },
		"four digit mnc":  func(p *db.RoamingPartner) { p.PLMNs[1].Mnc = "9300" },
		"duplicate PLMN":  func(p *db.RoamingPartner) { p.PLMNs[1] = p.PLMNs[0] },
		"zero-padded MNC": func(p *db.RoamingPartner) { p.PLMNs[1].Mnc = "093" },
		"prefix MNC":      func(p *db.RoamingPartner) { p.PLMNs[1].Mnc = "930" },
		"no profile":      func(p *db.RoamingPartner) { p.ProfileID = "" },
		"fqdn only":       func(p *db.RoamingPartner) { p.SEPPN32cURI, p.SEPPTrustAnchorPEM = nil, nil },
		"no trust anchor": func(p *db.RoamingPartner) { p.SEPPTrustAnchorPEM = nil },
		"n32f without 5G": func(p *db.RoamingPartner) {
			p.SEPPFQDN, p.SEPPN32cURI, p.SEPPTrustAnchorPEM, p.SEPPN32fURI = nil, nil, nil, ptr("https://f.example.org")
		},
		"nrf without 5G": func(p *db.RoamingPartner) {
			p.SEPPFQDN, p.SEPPN32cURI, p.SEPPTrustAnchorPEM = nil, nil, nil
			p.NRFAPIRoots = []db.RoamingNRFAPIRoot{{RoamingPLMN: p.PLMNs[0], APIRoot: "http://nrf.example.org"}}
		},
		"bad fqdn":           func(p *db.RoamingPartner) { p.SEPPFQDN = ptr("sepp") },
		"non-3GPP fqdn":      func(p *db.RoamingPartner) { p.SEPPFQDN = ptr("sepp.example.org") },
		"well-known fqdn":    func(p *db.RoamingPartner) { p.SEPPFQDN = ptr("sepp.5gc.mnc093.mcc208.3gppnetwork.org") },
		"fqdn of other PLMN": func(p *db.RoamingPartner) { p.SEPPFQDN = ptr("sepp1.5gc.mnc095.mcc208.3gppnetwork.org") },
		"fqdn with _":        func(p *db.RoamingPartner) { p.SEPPFQDN = ptr("sepp_1.5gc.mnc093.mcc208.3gppnetwork.org") },
		"http n32c":          func(p *db.RoamingPartner) { p.SEPPN32cURI = ptr("http://sepp.example.org") },
		"n32c with query":    func(p *db.RoamingPartner) { p.SEPPN32cURI = ptr("https://sepp.example.org?x=1") },
		"n32c without host":  func(p *db.RoamingPartner) { p.SEPPN32cURI = ptr("https://") },
		"n32c with bad port": func(p *db.RoamingPartner) { p.SEPPN32cURI = ptr("https://sepp.example.org:99999") },
		"n32c with no port":  func(p *db.RoamingPartner) { p.SEPPN32cURI = ptr("https://sepp.example.org:") },
		"n32c ending with /": func(p *db.RoamingPartner) { p.SEPPN32cURI = ptr("https://sepp.example.org/") },
		"ftp nrf": func(p *db.RoamingPartner) {
			p.NRFAPIRoots = []db.RoamingNRFAPIRoot{{RoamingPLMN: p.PLMNs[0], APIRoot: "ftp://nrf.example.org"}}
		},
		"nrf of other PLMN": func(p *db.RoamingPartner) {
			p.NRFAPIRoots = []db.RoamingNRFAPIRoot{{RoamingPLMN: db.RoamingPLMN{Mcc: "208", Mnc: "95"}, APIRoot: "http://nrf.example.org"}}
		},
		"duplicate nrf": func(p *db.RoamingPartner) {
			p.NRFAPIRoots = []db.RoamingNRFAPIRoot{{RoamingPLMN: p.PLMNs[0], APIRoot: "http://a.example.org"}, {RoamingPLMN: p.PLMNs[0], APIRoot: "http://b.example.org"}}
		},
		"garbage anchor": func(p *db.RoamingPartner) { p.SEPPTrustAnchorPEM = ptr("not a certificate") },
		"empty peer id":  func(p *db.RoamingPartner) { p.DiameterPeerID = ptr("") },
	}

	for name, mutate := range cases {
		p := valid5G()
		mutate(&p)

		if err := p.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	if got := db.NormalizeAPIRoot(" https://sepp.example.org/prefix// "); got != "https://sepp.example.org/prefix" {
		t.Errorf("NormalizeAPIRoot = %q", got)
	}
}

func TestRoamingPartnerConflicts(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)

	first := f.with5G(f.newPartner(roamingPartnerID, "partner", "208", "93", "208", "94"), partnerSEPPFQDN)
	if err := f.db.CreateRoamingPartner(ctx, first); err != nil {
		t.Fatalf("CreateRoamingPartner: %s", err)
	}

	if err := f.db.CreateSMSCPeer(ctx, &db.SMSCPeer{ID: "smsc", Address: "10.0.0.5", Port: 3868, ServiceCentres: []string{"15550000000"}}); err != nil {
		t.Fatalf("CreateSMSCPeer: %s", err)
	}

	cases := map[string]struct {
		partner *db.RoamingPartner
		want    error
	}{
		"duplicate name":             {f.newPartner("p2", "partner", "208", "95"), db.ErrAlreadyExists},
		"duplicate PLMN":             {f.newPartner("p2", "p2", "208", "93"), db.ErrRoamingConflict},
		"duplicate second PLMN":      {f.newPartner("p2", "p2", "208", "95", "208", "94"), db.ErrRoamingConflict},
		"zero-padded MNC":            {f.newPartner("p2", "p2", "208", "093"), db.ErrRoamingConflict},
		"3-digit under 2-digit":      {f.newPartner("p2", "p2", "208", "930"), db.ErrRoamingConflict},
		"home PLMN":                  {f.newPartner("p2", "p2", db.InitialMcc, db.InitialMnc), db.ErrRoamingConflict},
		"zero-padded home PLMN":      {f.newPartner("p2", "p2", db.InitialMcc, "001"), db.ErrRoamingConflict},
		"3-digit under home 2-digit": {f.newPartner("p2", "p2", db.InitialMcc, "012"), db.ErrRoamingConflict},
		"missing profile":            {&db.RoamingPartner{ID: "p2", Name: "p2", PLMNs: plmns("208", "95"), ProfileID: "missing"}, db.ErrRoamingConflict},
		"missing peer":               {&db.RoamingPartner{ID: "p2", Name: "p2", PLMNs: plmns("208", "95"), ProfileID: f.profileID, DiameterPeerID: ptr("missing")}, db.ErrRoamingConflict},
		"smsc peer":                  {&db.RoamingPartner{ID: "p2", Name: "p2", PLMNs: plmns("208", "95"), ProfileID: f.profileID, DiameterPeerID: ptr("smsc")}, db.ErrRoamingConflict},
	}

	for name, tc := range cases {
		if err := f.db.CreateRoamingPartner(ctx, tc.partner); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}

	if err := f.db.CreateRoamingPartner(ctx, f.newPartner("p2", "p2", "209", "930")); err != nil {
		t.Fatalf("same MNC under another MCC: %s", err)
	}

	if err := f.db.CreateRoamingPartner(ctx, f.newPartner("p3", "p3", "209", "93")); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("2-digit prefix of an existing 3-digit MNC: err = %v, want ErrRoamingConflict", err)
	}

	for _, pairs := range [][]string{{"209", "930", "208", "94"}, {"209", "930", "208", "930"}, {"209", "930", db.InitialMcc, db.InitialMnc}} {
		if err := f.db.UpdateRoamingPartner(ctx, f.newPartner("p2", "p2", pairs...)); !errors.Is(err, db.ErrRoamingConflict) {
			t.Fatalf("update of the PLMNs to %v: err = %v, want ErrRoamingConflict", pairs, err)
		}
	}

	if err := f.db.UpdateRoamingPartner(ctx, f.newPartner("p2", "p2-renamed", "209", "930", "210", "01")); err != nil {
		t.Fatalf("update adding a PLMN: %s", err)
	}

	if err := f.db.UpdateOperatorID(ctx, db.InitialMcc, db.InitialMnc); err != nil {
		t.Fatalf("operator PLMN unchanged with partners: %s", err)
	}

	if err := f.db.UpdateOperatorID(ctx, "208", "99"); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("operator PLMN changed with partners: err = %v, want ErrRoamingConflict", err)
	}

	for _, id := range []string{roamingPartnerID, "p2"} {
		if err := f.db.DeleteRoamingPartner(ctx, id); err != nil {
			t.Fatalf("DeleteRoamingPartner: %s", err)
		}
	}

	identity := ownSEPPIdentity(t, "1")
	if err := f.db.PutSEPPIdentity(ctx, &identity); err != nil {
		t.Fatalf("PutSEPPIdentity: %s", err)
	}

	if deleted, err := f.db.ChangeOperatorID(ctx, db.InitialMcc, db.InitialMnc); err != nil || deleted {
		t.Fatalf("operator PLMN unchanged = %v, %v; want the SEPP identity kept", deleted, err)
	}

	if deleted, err := f.db.ChangeOperatorID(ctx, "208", "99"); err != nil || !deleted {
		t.Fatalf("operator PLMN changed without partners = %v, %v; want the SEPP identity deleted", deleted, err)
	}

	if _, err := f.db.GetSEPPIdentity(ctx, 1); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("SEPP identity after a PLMN change: err = %v, want ErrNotFound", err)
	}

	if deleted, err := f.db.ChangeOperatorID(ctx, "208", "98"); err != nil || deleted {
		t.Fatalf("operator PLMN changed without an identity = %v, %v", deleted, err)
	}
}

func TestDeleteProfileOfRoamingPartnerIsInUse(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)

	p := f.partner()
	p.ProfileID = f.otherID

	if err := f.db.CreateRoamingPartner(ctx, p); err != nil {
		t.Fatalf("CreateRoamingPartner: %s", err)
	}

	if err := f.db.DeleteProfile(ctx, "roamers"); !errors.Is(err, db.ErrInUse) {
		t.Fatalf("DeleteProfile of a partner profile: err = %v, want ErrInUse", err)
	}

	if err := f.db.DeleteRoamingPartner(ctx, roamingPartnerID); err != nil {
		t.Fatalf("DeleteRoamingPartner: %s", err)
	}

	if err := f.db.DeleteProfile(ctx, "roamers"); err != nil {
		t.Fatalf("DeleteProfile after partner delete: %s", err)
	}
}

func TestRoamingDiameterPeers(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)

	if err := f.db.CreateSMSCPeer(ctx, &db.SMSCPeer{ID: "smsc", DiameterIdentity: "smsc.example.org", Address: "10.0.0.5", Port: 3868, ServiceCentres: []string{"15550000000"}}); err != nil {
		t.Fatalf("CreateSMSCPeer: %s", err)
	}

	f.createPeer(t)

	smsc, err := f.db.ListSMSCPeers(ctx)
	if err != nil || len(smsc) != 1 || smsc[0].ID != "smsc" {
		t.Fatalf("ListSMSCPeers = %+v, %v", smsc, err)
	}

	roaming, err := f.db.ListRoamingDiameterPeers(ctx)
	if err != nil || len(roaming) != 1 || roaming[0].ID != roamingPeerID {
		t.Fatalf("ListRoamingDiameterPeers = %+v, %v", roaming, err)
	}

	if _, err := f.db.GetSMSCPeer(ctx, roamingPeerID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("GetSMSCPeer of a roaming peer: err = %v, want ErrNotFound", err)
	}

	if _, err := f.db.GetRoamingDiameterPeer(ctx, "smsc"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("GetRoamingDiameterPeer of an SMSC peer: err = %v, want ErrNotFound", err)
	}

	if err := f.db.UpdateRoamingDiameterPeer(ctx, &db.RoamingDiameterPeer{ID: "smsc", Address: "10.0.0.9", Port: 3868}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("UpdateRoamingDiameterPeer of an SMSC peer: err = %v, want ErrNotFound", err)
	}

	if err := f.db.DeleteRoamingDiameterPeer(ctx, "smsc"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("DeleteRoamingDiameterPeer of an SMSC peer: err = %v, want ErrNotFound", err)
	}

	if err := f.db.DeleteSMSCPeer(ctx, roamingPeerID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("DeleteSMSCPeer of a roaming peer: err = %v, want ErrNotFound", err)
	}

	conflicts := map[string]db.RoamingDiameterPeer{
		"smsc identity":   {ID: "r2", DiameterIdentity: "SMSC.example.org", Address: "10.0.0.8", Port: 3868},
		"smsc address":    {ID: "r2", Address: "10.0.0.5", Port: 3868},
		"roaming address": {ID: "r2", DiameterIdentity: "other.example.org", Address: "10.0.0.7", Port: 3868},
	}

	for name, peer := range conflicts {
		if err := f.db.CreateRoamingDiameterPeer(ctx, &peer); !errors.Is(err, db.ErrRoamingConflict) {
			t.Errorf("%s: err = %v, want ErrRoamingConflict", name, err)
		}
	}

	if err := f.db.CreateSMSCPeer(ctx, &db.SMSCPeer{ID: "smsc2", DiameterIdentity: "dea.example.org", Address: "10.0.0.9", Port: 3868, ServiceCentres: []string{"15550000001"}}); !errors.Is(err, db.ErrSMSCPeerConflict) {
		t.Fatalf("SMSC peer with a roaming peer's identity: err = %v, want ErrSMSCPeerConflict", err)
	}

	if err := f.db.UpdateRoamingDiameterPeer(ctx, &db.RoamingDiameterPeer{ID: roamingPeerID, DiameterIdentity: "hss.example.org", Address: "10.0.0.7", Port: 3869}); err != nil {
		t.Fatalf("UpdateRoamingDiameterPeer: %s", err)
	}

	if peer, err := f.db.GetRoamingDiameterPeer(ctx, roamingPeerID); err != nil || peer.DiameterIdentity != "hss.example.org" || peer.Port != 3869 {
		t.Fatalf("GetRoamingDiameterPeer = %+v, %v", peer, err)
	}

	p := f.partner()
	p.DiameterPeerID = ptr(roamingPeerID)

	if err := f.db.CreateRoamingPartner(ctx, p); err != nil {
		t.Fatalf("CreateRoamingPartner: %s", err)
	}

	if err := f.db.DeleteRoamingDiameterPeer(ctx, roamingPeerID); !errors.Is(err, db.ErrInUse) {
		t.Fatalf("DeleteRoamingDiameterPeer in use: err = %v, want ErrInUse", err)
	}

	if err := f.db.DeleteRoamingPartner(ctx, roamingPartnerID); err != nil {
		t.Fatalf("DeleteRoamingPartner: %s", err)
	}

	if err := f.db.DeleteRoamingDiameterPeer(ctx, roamingPeerID); err != nil {
		t.Fatalf("DeleteRoamingDiameterPeer: %s", err)
	}

	if err := (db.RoamingDiameterPeer{ID: "x", Address: "10.0.0.1", Port: 0}).Validate(); err == nil {
		t.Fatal("port 0 accepted")
	}

	if err := (db.RoamingDiameterPeer{ID: "x", Address: "hss.example.org", Port: 3868}).Validate(); err == nil {
		t.Fatal("hostname address accepted")
	}
}

func TestSEPPIdentity(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)

	if _, err := f.db.GetSEPPIdentity(ctx, 1); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("GetSEPPIdentity on a fresh database: err = %v, want ErrNotFound", err)
	}

	valid := ownSEPPIdentity(t, "1")
	other := ownSEPPIdentity(t, "2")

	invalid := map[string]db.SEPPIdentity{
		"no key":          {SEPPID: 1, Source: db.SEPPIdentityImported, FQDN: valid.FQDN, CertChainPEM: valid.CertChainPEM},
		"garbage chain":   {SEPPID: 1, Source: db.SEPPIdentityImported, FQDN: valid.FQDN, CertChainPEM: "chain", KeyPEM: valid.KeyPEM},
		"mismatched key":  {SEPPID: 1, Source: db.SEPPIdentityImported, FQDN: valid.FQDN, CertChainPEM: valid.CertChainPEM, KeyPEM: other.KeyPEM},
		"FQDN not in SAN": {SEPPID: 1, Source: db.SEPPIdentityImported, FQDN: other.FQDN, CertChainPEM: valid.CertChainPEM, KeyPEM: valid.KeyPEM},
		"no FQDN":         {SEPPID: 1, Source: db.SEPPIdentityImported, CertChainPEM: valid.CertChainPEM, KeyPEM: valid.KeyPEM},
		"uppercase FQDN":  {SEPPID: 1, Source: db.SEPPIdentityImported, FQDN: strings.ToUpper(valid.FQDN), CertChainPEM: valid.CertChainPEM, KeyPEM: valid.KeyPEM},
		"SEPP ID 0":       {SEPPID: 0, Source: db.SEPPIdentityImported, FQDN: valid.FQDN, CertChainPEM: valid.CertChainPEM, KeyPEM: valid.KeyPEM},
		"SEPP ID 64":      {SEPPID: 64, Source: db.SEPPIdentityImported, FQDN: valid.FQDN, CertChainPEM: valid.CertChainPEM, KeyPEM: valid.KeyPEM},
		"no source":       {SEPPID: 1, FQDN: valid.FQDN, CertChainPEM: valid.CertChainPEM, KeyPEM: valid.KeyPEM},
	}

	for name, identity := range invalid {
		if err := f.db.PutSEPPIdentity(ctx, &identity); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	foreign := seppIdentity(t, pki.PLMN{MCC: "208", MNC: "93"}, partnerSEPPFQDN)
	if err := f.db.PutSEPPIdentity(ctx, &foreign); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("identity of another PLMN: err = %v, want ErrRoamingConflict", err)
	}

	generatedWithoutCA := valid
	generatedWithoutCA.Source = db.SEPPIdentityGenerated

	if err := f.db.PutSEPPIdentity(ctx, &generatedWithoutCA); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("generated identity without a CA: err = %v, want ErrRoamingConflict", err)
	}

	for _, want := range []db.SEPPIdentity{valid, other} {
		if err := f.db.PutSEPPIdentity(ctx, &want); err != nil {
			t.Fatalf("PutSEPPIdentity: %s", err)
		}

		got, err := f.db.GetSEPPIdentity(ctx, want.SEPPID)
		if err != nil || *got != want {
			t.Fatalf("GetSEPPIdentity = %+v, %v; want %+v", got, err, want)
		}
	}

	sameFQDN := ownSEPPIdentity(t, "1")
	sameFQDN.SEPPID = 3

	if err := f.db.PutSEPPIdentity(ctx, &sameFQDN); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("FQDN of another node: err = %v, want ErrRoamingConflict", err)
	}

	all, err := f.db.ListSEPPIdentities(ctx)
	if err != nil || len(all) != 2 || all[0].SEPPID != 1 || all[1].SEPPID != 2 {
		t.Fatalf("ListSEPPIdentities = %+v, %v", all, err)
	}
}

func TestSEPPIdentityProvisioning(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	own := pki.PLMN{MCC: db.InitialMcc, MNC: db.InitialMnc}

	if err := f.db.PutGeneratedSEPPIdentity(ctx, 2); err != nil {
		t.Fatalf("PutGeneratedSEPPIdentity: %s", err)
	}

	ca, err := f.db.GetSEPPCA(ctx)
	if err != nil {
		t.Fatalf("GetSEPPCA: %s", err)
	}

	caCert, err := ca.Certificate()
	if err != nil {
		t.Fatalf("CA certificate: %s", err)
	}

	two, err := f.db.GetSEPPIdentity(ctx, 2)
	if err != nil || two.Source != db.SEPPIdentityGenerated || two.FQDN != own.SEPPFQDN(2) {
		t.Fatalf("generated identity = %+v, %v", two, err)
	}

	cert, err := pki.ParseSEPPCertificate([]byte(two.CertChainPEM), two.FQDN)
	if err != nil || !cert.TrustAnchor().Equal(caCert) {
		t.Fatalf("generated chain does not end with the cluster SEPP CA: %v", err)
	}

	if err := f.db.PutGeneratedSEPPIdentity(ctx, 2); err != nil {
		t.Fatalf("PutGeneratedSEPPIdentity again: %s", err)
	}

	if renewed, err := f.db.GetSEPPIdentity(ctx, 2); err != nil || renewed.CertChainPEM == two.CertChainPEM {
		t.Fatalf("renewed identity = %+v, %v; want a new certificate", renewed, err)
	}

	if err := f.db.PutGeneratedSEPPIdentity(ctx, 3); err != nil {
		t.Fatalf("PutGeneratedSEPPIdentity: %s", err)
	}

	imported := ownSEPPIdentity(t, "1")
	if err := f.db.PutSEPPIdentity(ctx, &imported); err != nil {
		t.Fatalf("PutSEPPIdentity: %s", err)
	}

	if err := f.db.PutGeneratedSEPPIdentity(ctx, 1); err != nil {
		t.Fatalf("PutGeneratedSEPPIdentity over an imported one: %s", err)
	}

	if got, err := f.db.GetSEPPIdentity(ctx, 1); err != nil || *got != imported {
		t.Fatalf("identity 1 = %+v, %v; want the import kept", got, err)
	}

	stale := *two

	if _, err := f.db.RegenerateSEPPCA(ctx); err != nil {
		t.Fatalf("RegenerateSEPPCA: %s", err)
	}

	all, err := f.db.ListSEPPIdentities(ctx)
	if err != nil || len(all) != 1 || all[0] != imported {
		t.Fatalf("identities after a CA regeneration = %+v, %v; want only the imported one", all, err)
	}

	if err := f.db.PutSEPPIdentity(ctx, &stale); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("leaf of the previous CA: err = %v, want ErrRoamingConflict", err)
	}

	if err := f.db.DeleteSEPPIdentity(ctx, 1); err != nil {
		t.Fatalf("DeleteSEPPIdentity: %s", err)
	}

	if err := f.db.DeleteSEPPIdentity(ctx, 1); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("DeleteSEPPIdentity again: %v, want ErrNotFound", err)
	}

	if err := f.db.PutGeneratedSEPPIdentity(ctx, 1); err != nil {
		t.Fatalf("PutGeneratedSEPPIdentity after the delete: %s", err)
	}

	if got, err := f.db.GetSEPPIdentity(ctx, 1); err != nil || got.Source != db.SEPPIdentityGenerated {
		t.Fatalf("identity 1 = %+v, %v; want generated", got, err)
	}

	if err := f.db.PutGeneratedSEPPIdentity(ctx, 0); err == nil {
		t.Fatal("PutGeneratedSEPPIdentity accepted SEPP ID 0")
	}

	if deleted, err := f.db.ChangeOperatorID(ctx, "208", "99"); err != nil || !deleted {
		t.Fatalf("operator PLMN change = %v, %v; want the identities deleted", deleted, err)
	}

	if _, err := f.db.GetSEPPCA(ctx); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("SEPP CA after a PLMN change: err = %v, want ErrNotFound", err)
	}
}

func TestRoamingDailyUsage(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPartner(t)

	today := db.DaysSinceEpoch(time.Now())
	old := db.DaysSinceEpoch(time.Now().AddDate(0, 0, -10))

	roamer := func(day int64, imsi, mcc, mnc string, up, down int64) db.DailyUsage {
		return db.DailyUsage{EpochDay: day, IMSI: imsi, Mcc: &mcc, Mnc: &mnc, BytesUplink: up, BytesDownlink: down}
	}

	rows := []db.DailyUsage{
		roamer(today, visitorIMSI, "208", "93", 1, 2),
		roamer(today, visitorIMSI, "208", "93", 10, 20),
		roamer(today, "208930000000002", "208", "93", 100, 200),
		roamer(today, "310410000000001", "310", "410", 5, 5),
		roamer(old, visitorIMSI, "208", "93", 7, 7),
	}

	if err := f.db.IncrementDailyUsageBatch(ctx, rows); err != nil {
		t.Fatalf("IncrementDailyUsageBatch: %s", err)
	}

	all := db.NewDayRange(time.Now().AddDate(0, 0, -30), time.Now().AddDate(0, 0, 1))
	roamers := true

	usage, err := f.db.GetUsagePerSubscriber(ctx, "", all, &roamers, db.NoUsageLimit)
	if err != nil {
		t.Fatalf("GetUsagePerSubscriber: %s", err)
	}

	type plmnUsage struct {
		imsi, mcc, mnc string
		up, down       int64
	}

	got := make([]plmnUsage, 0, len(usage))
	for _, u := range usage {
		got = append(got, plmnUsage{u.IMSI, *u.Mcc, *u.Mnc, u.BytesUplink, u.BytesDownlink})
	}

	want := []plmnUsage{
		{"208930000000002", "208", "93", 100, 200},
		{visitorIMSI, "208", "93", 18, 29},
		{"310410000000001", "310", "410", 5, 5},
	}

	if !slices.Equal(got, want) {
		t.Fatalf("usage = %+v, want %+v", got, want)
	}

	if err := f.db.DeleteOldDailyUsage(ctx, 5); err != nil {
		t.Fatalf("DeleteOldDailyUsage: %s", err)
	}

	perDay, err := f.db.GetUsagePerDay(ctx, "", all, &roamers)
	if err != nil || len(perDay) != 1 || perDay[0].BytesUplink != 116 {
		t.Fatalf("GetUsagePerDay after retention = %+v, %v", perDay, err)
	}

	if err := f.db.DeleteRoamingPartner(ctx, roamingPartnerID); err != nil {
		t.Fatalf("DeleteRoamingPartner: %s", err)
	}

	perDay, err = f.db.GetUsagePerDay(ctx, "", all, &roamers)
	if err != nil || len(perDay) != 1 || perDay[0].BytesUplink != 116 {
		t.Fatalf("GetUsagePerDay after partner delete = %+v, %v; want the usage kept", perDay, err)
	}
}

func TestRegisterVisitorUE(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)

	if err := f.db.CreateRoamingPartner(ctx, f.newPartner(roamingPartnerID, "partner", "208", "93", "208", "940")); err != nil {
		t.Fatalf("CreateRoamingPartner: %s", err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, "208940000000001", roamingPartnerID, db.UERegistrationTypeMME, "node-1", ""); err != nil {
		t.Fatalf("RegisterVisitorUE in the second PLMN: %s", err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, "208094000000001", roamingPartnerID, db.UERegistrationTypeMME, "node-1", ""); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("visitor matching no PLMN of the partner: err = %v, want ErrRoamingConflict", err)
	}

	version, err := f.db.RegisterVisitorUE(ctx, visitorIMSI, roamingPartnerID, db.UERegistrationTypeMME, "node-1", "")
	if err != nil {
		t.Fatalf("RegisterVisitorUE: %s", err)
	}

	visitor, err := f.db.GetSubscriberOrVisitor(ctx, visitorIMSI)
	if err != nil {
		t.Fatalf("GetSubscriberOrVisitor: %s", err)
	}

	if visitor.RoamingPartnerID == nil || *visitor.RoamingPartnerID != roamingPartnerID || visitor.ProfileID != f.profileID ||
		visitor.PermanentKey != "" || visitor.Opc != "" || visitor.SequenceNumber != "" || visitor.Msisdn != "" || visitor.ID != "" {
		t.Fatalf("visitor = %+v", visitor)
	}

	reg, err := f.db.GetUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME)
	if err != nil || reg.NodeID != "node-1" || reg.Purged || reg.Version != version {
		t.Fatalf("visitor registration = %+v, %v", reg, err)
	}

	again, err := f.db.RegisterVisitorUE(ctx, visitorIMSI, roamingPartnerID, db.UERegistrationTypeAMF3GPPAccess, "node-2", db.UERegistrationTypeMME)
	if err != nil {
		t.Fatalf("RegisterVisitorUE on an existing visitor: %s", err)
	}

	if again <= version {
		t.Fatalf("version %d after re-registration, want above %d", again, version)
	}

	if reg, err := f.db.GetUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME); err != nil || !reg.Purged {
		t.Fatalf("cancelled MME registration = %+v, %v", reg, err)
	}

	row, err := f.db.GetVisitor(ctx, visitorIMSI)
	if err != nil || row.PartnerID != roamingPartnerID || row.ProfileID != f.profileID || row.Since == 0 {
		t.Fatalf("GetVisitor = %+v, %v", row, err)
	}

	if err := f.db.SetVisitorMSISDN(ctx, visitorIMSI, "33612345678"); err != nil {
		t.Fatalf("SetVisitorMSISDN: %s", err)
	}

	if err := f.db.SetVisitorMSISDN(ctx, visitorIMSI, "+33612345678"); err == nil {
		t.Fatal("non-digit visitor MSISDN accepted")
	}

	if err := f.db.SetVisitorMSISDN(ctx, "208930000000099", "33612345678"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("SetVisitorMSISDN of an unknown visitor: err = %v, want ErrNotFound", err)
	}

	if v, err := f.db.GetSubscriberOrVisitor(ctx, visitorIMSI); err != nil || v.Msisdn != "33612345678" {
		t.Fatalf("visitor MSISDN = %+v, %v", v, err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, visitorIMSI, roamingPartnerID, db.UERegistrationTypeMME, "node-1", db.UERegistrationTypeAMF3GPPAccess); err != nil {
		t.Fatalf("RegisterVisitorUE again: %s", err)
	}

	if again2, err := f.db.GetVisitor(ctx, visitorIMSI); err != nil || again2.Since != row.Since || again2.Msisdn != "33612345678" {
		t.Fatalf("visitor row replaced: %+v, %v", again2, err)
	}

	err = f.db.CreateSubscriber(ctx, &db.Subscriber{Imsi: visitorIMSI, SequenceNumber: "000000000001", PermanentKey: "00112233445566778899aabbccddeeff", Opc: "00112233445566778899aabbccddeeff", ProfileID: f.profileID})
	if !errors.Is(err, db.ErrAlreadyExists) {
		t.Fatalf("home subscriber over a visitor: err = %v, want ErrAlreadyExists", err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, "208930000000009", "missing", db.UERegistrationTypeMME, "node-1", ""); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("visitor of a missing partner: err = %v, want ErrNotFound", err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, "310410000000001", roamingPartnerID, db.UERegistrationTypeMME, "node-1", ""); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("visitor outside the partner PLMN: err = %v, want ErrRoamingConflict", err)
	}

	f.createOwnSubscriber(t, "208930000000005")

	if _, err := f.db.RegisterVisitorUE(ctx, "208930000000005", roamingPartnerID, db.UERegistrationTypeMME, "node-1", ""); !errors.Is(err, db.ErrAlreadyExists) {
		t.Fatalf("visitor over a home subscriber: err = %v, want ErrAlreadyExists", err)
	}

	if own, err := f.db.GetSubscriber(ctx, "208930000000005"); err != nil || own.PermanentKey == "" {
		t.Fatalf("home subscriber touched: %+v, %v", own, err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, visitorIMSI, roamingPartnerID, "bogus", "node-1", ""); err == nil {
		t.Fatal("invalid registration type accepted")
	}

	if _, err := f.db.RegisterVisitorUE(ctx, visitorIMSI, "", db.UERegistrationTypeMME, "node-1", ""); err == nil {
		t.Fatal("missing partner ID accepted")
	}
}

func TestPurgingTheLastRegistrationDeletesTheVisitor(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.registerVisitor(t, visitorIMSI)

	if _, err := f.db.RegisterVisitorUE(ctx, visitorIMSI, roamingPartnerID, db.UERegistrationTypeAMF3GPPAccess, "node-2", ""); err != nil {
		t.Fatalf("RegisterVisitorUE over 5G: %s", err)
	}

	f.createOwnSubscriber(t, "001010000000001")

	if _, err := f.db.RegisterUE(ctx, "001010000000001", db.UERegistrationTypeMME, "node-1", ""); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if err := f.db.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration over 4G: %s", err)
	}

	if _, err := f.db.GetVisitor(ctx, visitorIMSI); err != nil {
		t.Fatalf("visitor still registered over 5G: %s", err)
	}

	if err := f.db.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeAMF3GPPAccess, "node-2"); err != nil {
		t.Fatalf("PurgeUERegistration over 5G: %s", err)
	}

	if _, err := f.db.GetSubscriberOrVisitor(ctx, visitorIMSI); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("visitor after its last purge: err = %v, want ErrNotFound", err)
	}

	if _, err := f.db.GetUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("visitor registration after delete: err = %v, want ErrNotFound", err)
	}

	if err := f.db.PurgeUERegistration(ctx, "001010000000001", db.UERegistrationTypeMME, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration of a home subscriber: %s", err)
	}

	if _, err := f.db.GetSubscriber(ctx, "001010000000001"); err != nil {
		t.Fatalf("home subscriber after its purge: %s", err)
	}
}

func TestRoamingPartnerProfileChangeAndDeleteRestrict(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPeer(t)

	p := f.with5G(f.newPartner(roamingPartnerID, "partner", "208", "93", "208", "94"), partnerSEPPFQDN)
	p.DiameterPeerID = ptr(roamingPeerID)

	if err := f.db.CreateRoamingPartner(ctx, p); err != nil {
		t.Fatalf("CreateRoamingPartner: %s", err)
	}

	f.registerVisitor(t, visitorIMSI)
	f.createOwnSubscriber(t, "001010000000001")

	p.ProfileID = f.otherID
	if err := f.db.UpdateRoamingPartner(ctx, p); err != nil {
		t.Fatalf("UpdateRoamingPartner: %s", err)
	}

	if visitor, err := f.db.GetSubscriberOrVisitor(ctx, visitorIMSI); err != nil || visitor.ProfileID != f.otherID {
		t.Fatalf("visitor after partner profile change = %+v, %v", visitor, err)
	}

	if own, err := f.db.GetSubscriber(ctx, "001010000000001"); err != nil || own.ProfileID != f.profileID {
		t.Fatalf("home subscriber after partner profile change = %+v, %v", own, err)
	}

	if err := f.db.DeleteProfile(ctx, "roamers"); !errors.Is(err, db.ErrInUse) {
		t.Fatalf("DeleteProfile of a roaming partner profile: err = %v, want ErrInUse", err)
	}

	refused := map[string]func(q *db.RoamingPartner){
		"remove the roamer's PLMN": func(q *db.RoamingPartner) {
			q.PLMNs = plmns("208", "94")
			q.SEPPFQDN, q.SEPPN32cURI, q.SEPPTrustAnchorPEM = nil, nil, nil
		},
		"remove the 4G agreement": func(q *db.RoamingPartner) { q.DiameterPeerID = nil },
	}

	for name, mutate := range refused {
		q := *p
		q.PLMNs = slices.Clone(p.PLMNs)
		mutate(&q)

		if err := f.db.UpdateRoamingPartner(ctx, &q); !errors.Is(err, db.ErrRoamingConflict) {
			t.Errorf("%s with an attached roamer: err = %v, want ErrRoamingConflict", name, err)
		}
	}

	allowed := *p
	allowed.PLMNs = plmns("208", "93")
	allowed.SEPPFQDN, allowed.SEPPN32cURI, allowed.SEPPTrustAnchorPEM = nil, nil, nil

	if err := f.db.UpdateRoamingPartner(ctx, &allowed); err != nil {
		t.Fatalf("removing the 5G agreement and an unused PLMN with a 4G roamer: %s", err)
	}

	if err := f.db.DeleteRoamingPartner(ctx, roamingPartnerID); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("DeleteRoamingPartner with an attached roamer: err = %v, want ErrRoamingConflict", err)
	}

	if _, err := f.db.GetSubscriberOrVisitor(ctx, visitorIMSI); err != nil {
		t.Fatalf("visitor after a refused partner delete: %s", err)
	}

	if err := f.db.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	if err := f.db.DeleteRoamingPartner(ctx, roamingPartnerID); err != nil {
		t.Fatalf("DeleteRoamingPartner without roamers: %s", err)
	}

	if _, err := f.db.GetSubscriber(ctx, "001010000000001"); err != nil {
		t.Fatalf("home subscriber after partner delete: %s", err)
	}
}

func TestSubscriberReadsWithVisitors(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.createOwnSubscriber(t, "001010000000001")
	f.registerVisitor(t, visitorIMSI)

	if _, err := f.db.GetSubscriber(ctx, visitorIMSI); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("GetSubscriber of a visitor: err = %v, want ErrNotFound", err)
	}

	if own, err := f.db.GetSubscriberOrVisitor(ctx, "001010000000001"); err != nil || own.RoamingPartnerID != nil {
		t.Fatalf("GetSubscriberOrVisitor of a home subscriber = %+v, %v", own, err)
	}

	subs, total, err := f.db.ListSubscribersPage(ctx, nil, 1, 10)
	if err != nil || total != 1 || len(subs) != 1 || subs[0].Imsi != "001010000000001" {
		t.Fatalf("ListSubscribersPage = %+v, %d, %v", subs, total, err)
	}

	visitors, total, err := f.db.ListVisitorsPage(ctx, nil, 1, 10)
	if err != nil || total != 1 || len(visitors) != 1 || visitors[0].Imsi != visitorIMSI || visitors[0].PartnerID != roamingPartnerID || visitors[0].ProfileID != f.profileID {
		t.Fatalf("ListVisitorsPage = %+v, %d, %v", visitors, total, err)
	}

	search, partnerID, other := "20893", roamingPartnerID, "other"

	if visitors, total, err := f.db.ListVisitorsPage(ctx, &db.VisitorFilters{Search: &search, PartnerID: &partnerID}, 1, 10); err != nil || total != 1 || len(visitors) != 1 {
		t.Fatalf("ListVisitorsPage searching a visitor = %+v, %d, %v", visitors, total, err)
	}

	if visitors, total, err := f.db.ListVisitorsPage(ctx, &db.VisitorFilters{PartnerID: &other}, 1, 10); err != nil || total != 0 || len(visitors) != 0 {
		t.Fatalf("ListVisitorsPage of another partner = %+v, %d, %v", visitors, total, err)
	}

	if visitors, total, err := f.db.ListVisitorsPage(ctx, nil, 2, 10); err != nil || total != 1 || len(visitors) != 0 {
		t.Fatalf("ListVisitorsPage past the end = %+v, %d, %v", visitors, total, err)
	}

	if subs, total, err := f.db.ListSubscribersPage(ctx, &db.SubscriberFilters{Search: &search}, 1, 10); err != nil || total != 0 || len(subs) != 0 {
		t.Fatalf("ListSubscribersPage searching a visitor = %+v, %d, %v", subs, total, err)
	}

	if n, err := f.db.CountSubscribers(ctx); err != nil || n != 1 {
		t.Fatalf("CountSubscribers = %d, %v; want 1", n, err)
	}

	if n, err := f.db.CountSubscribersAndVisitors(ctx); err != nil || n != 2 {
		t.Fatalf("CountSubscribersAndVisitors = %d, %v; want 2", n, err)
	}

	if n, err := f.db.CountSubscribersInProfile(ctx, f.profileID); err != nil || n != 1 {
		t.Fatalf("CountSubscribersInProfile = %d, %v; want 1", n, err)
	}

	usage, err := f.db.GetUsagePerSubscriber(ctx, "", db.NewDayRange(time.Now().AddDate(0, 0, -1), time.Now()), nil, db.NoUsageLimit)
	if err != nil || len(usage) != 1 || usage[0].IMSI != "001010000000001" {
		t.Fatalf("GetUsagePerSubscriber = %+v, %v", usage, err)
	}

	if _, err := f.db.AdvanceSubscriberSQN(ctx, visitorIMSI, "", ""); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("AdvanceSubscriberSQN of a visitor: err = %v, want ErrNotFound", err)
	}

	if err := f.db.UpdateSubscriberProfile(ctx, &db.Subscriber{Imsi: visitorIMSI, ProfileID: f.otherID}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("UpdateSubscriberProfile of a visitor: err = %v, want ErrNotFound", err)
	}

	if err := f.db.DeleteSubscriber(ctx, visitorIMSI); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("DeleteSubscriber of a visitor: err = %v, want ErrNotFound", err)
	}

	if imsis, err := f.db.IMSRegisteredSubscribersOfProfiles(ctx, []string{f.profileID}); err != nil || len(imsis) != 0 {
		t.Fatalf("IMSRegisteredSubscribersOfProfiles = %v, %v", imsis, err)
	}

	if visitors, err := f.db.ListVisitors(ctx); err != nil || len(visitors) != 1 || visitors[0].Imsi != visitorIMSI {
		t.Fatalf("ListVisitors = %+v, %v", visitors, err)
	}

	if _, err := f.db.GetSessionPolicy(ctx, visitorIMSI, db.InitialSliceSst, "", db.InitialDataNetworkName); err != nil {
		t.Fatalf("GetSessionPolicy of a visitor: %s", err)
	}
}

func TestSupportBundleRedactsSEPPKey(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.createPeer(t)

	stored := ownSEPPIdentity(t, "1")
	if err := f.db.PutSEPPIdentity(ctx, &stored); err != nil {
		t.Fatalf("PutSEPPIdentity: %s", err)
	}

	if err := f.db.PutGeneratedSEPPIdentity(ctx, 2); err != nil {
		t.Fatalf("PutGeneratedSEPPIdentity: %s", err)
	}

	out, err := f.db.ExportSupportData(ctx, supportbundle.ExportOptions{})
	if err != nil {
		t.Fatalf("ExportSupportData: %s", err)
	}

	identity, ok := out["sepp_identities"].([]map[string]any)
	if !ok || len(identity) != 2 || identity[0]["KeyPEM"] != "*" || identity[1]["KeyPEM"] != "*" || identity[0]["CertChainPEM"] != stored.CertChainPEM || identity[0]["FQDN"] != stored.FQDN {
		t.Fatalf("sepp_identities = %#v", out["sepp_identities"])
	}

	ca, ok := out["sepp_ca"].([]map[string]any)
	if !ok || len(ca) != 1 || ca[0]["KeyPEM"] != "*" || ca[0]["CertPEM"] == "" {
		t.Fatalf("sepp_ca = %#v", out["sepp_ca"])
	}

	if partners, ok := out["roaming_partners"].([]db.RoamingPartner); !ok || len(partners) != 1 {
		t.Fatalf("roaming_partners = %#v", out["roaming_partners"])
	}

	if peers, ok := out["roaming_diameter_peers"].([]db.RoamingDiameterPeer); !ok || len(peers) != 1 {
		t.Fatalf("roaming_diameter_peers = %#v", out["roaming_diameter_peers"])
	}
}

func TestRoamingWritesReplicate(t *testing.T) {
	ctx := context.Background()
	database := setupRaftTestDB(t)

	createSubscriber(t, database, "001010000000001")

	own, err := database.GetSubscriber(ctx, "001010000000001")
	if err != nil {
		t.Fatalf("GetSubscriber: %s", err)
	}

	roaming, stopRoaming := database.Changefeed().Wakeup(db.TopicRoaming)
	defer stopRoaming()

	registrations, stopRegistrations := database.Changefeed().Wakeup(db.TopicUERegistrations)
	defer stopRegistrations()

	partner := &db.RoamingPartner{ID: roamingPartnerID, Name: "partner", PLMNs: plmns("208", "93"), ProfileID: own.ProfileID}

	expectOneRaftEntry(t, database, "CreateRoamingPartner", func() error { return database.CreateRoamingPartner(ctx, partner) })
	awaitWakeup(t, roaming, "CreateRoamingPartner")

	expectOneRaftEntry(t, database, "RegisterVisitorUE", func() error {
		_, err := database.RegisterVisitorUE(ctx, visitorIMSI, roamingPartnerID, db.UERegistrationTypeAMF3GPPAccess, "node-1", "")
		return err
	})
	awaitWakeup(t, registrations, "RegisterVisitorUE")

	if visitor, err := database.GetSubscriberOrVisitor(ctx, visitorIMSI); err != nil || visitor.RoamingPartnerID == nil {
		t.Fatalf("visitor = %+v, %v", visitor, err)
	}

	identity := ownSEPPIdentity(t, "1")

	expectOneRaftEntry(t, database, "SEPP identity", func() error {
		return database.PutSEPPIdentity(ctx, &identity)
	})

	if err := database.DeleteRoamingPartner(ctx, roamingPartnerID); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("DeleteRoamingPartner with a roamer: err = %v, want ErrRoamingConflict", err)
	}

	expectOneRaftEntry(t, database, "SetRoamingPartnerWithdrawing", func() error {
		return database.SetRoamingPartnerWithdrawing(ctx, roamingPartnerID, time.Now().Add(time.Minute))
	})
	awaitWakeup(t, roaming, "SetRoamingPartnerWithdrawing")

	expectOneRaftEntry(t, database, "SetVisitorDeregistering", func() error {
		return database.SetVisitorDeregistering(ctx, visitorIMSI, time.Now().Add(time.Minute))
	})

	expectOneRaftEntry(t, database, "PurgeUERegistration", func() error {
		return database.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeAMF3GPPAccess, "node-1")
	})

	expectOneRaftEntry(t, database, "DeleteRoamingPartner", func() error { return database.DeleteRoamingPartner(ctx, roamingPartnerID) })
	awaitWakeup(t, roaming, "DeleteRoamingPartner")

	if _, err := database.GetSubscriberOrVisitor(ctx, visitorIMSI); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("visitor after partner delete: err = %v, want ErrNotFound", err)
	}
}

func TestDeletingAClusterMemberRemovesItsGeneratedSEPPIdentity(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)

	for _, nodeID := range []string{"node-1", "node-2", "node-3"} {
		if err := f.db.UpsertClusterMember(ctx, &db.ClusterMember{NodeID: nodeID, APIAddress: "10.0.0.1:8443"}); err != nil {
			t.Fatalf("UpsertClusterMember: %s", err)
		}
	}

	members, err := f.db.ListClusterMembers(ctx)
	if err != nil || len(members) != 3 {
		t.Fatalf("ListClusterMembers = %+v, %v", members, err)
	}

	pointers := map[string]int{}
	for _, m := range members {
		pointers[m.NodeID] = m.AMFPointer
	}

	for _, nodeID := range []string{"node-1", "node-2"} {
		if err := f.db.PutGeneratedSEPPIdentity(ctx, pointers[nodeID]); err != nil {
			t.Fatalf("PutGeneratedSEPPIdentity: %s", err)
		}
	}

	imported := ownSEPPIdentity(t, strconv.Itoa(pointers["node-3"]))
	if err := f.db.PutSEPPIdentity(ctx, &imported); err != nil {
		t.Fatalf("PutSEPPIdentity: %s", err)
	}

	before, err := f.db.GetSEPPCA(ctx)
	if err != nil {
		t.Fatalf("GetSEPPCA: %s", err)
	}

	for _, nodeID := range []string{"node-2", "node-3"} {
		if err := f.db.DeleteClusterMember(ctx, nodeID); err != nil {
			t.Fatalf("DeleteClusterMember: %s", err)
		}
	}

	if _, err := f.db.GetSEPPIdentity(ctx, pointers["node-2"]); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("generated identity of the removed node: %v, want it deleted", err)
	}

	if got, err := f.db.GetSEPPIdentity(ctx, pointers["node-3"]); err != nil || got.Source != db.SEPPIdentityImported {
		t.Fatalf("imported identity of the removed node = %+v, %v; want it kept", got, err)
	}

	if _, err := f.db.GetSEPPIdentity(ctx, pointers["node-1"]); err != nil {
		t.Fatalf("identity of the remaining node: %v", err)
	}

	if after, err := f.db.GetSEPPCA(ctx); err != nil || after.CertPEM != before.CertPEM {
		t.Fatalf("the SEPP CA changed with the member removal: %v", err)
	}

	if err := f.db.UpsertClusterMember(ctx, &db.ClusterMember{NodeID: "node-4", APIAddress: "10.0.0.1:8443"}); err != nil {
		t.Fatalf("UpsertClusterMember: %s", err)
	}

	joined, err := f.db.GetClusterMember(ctx, "node-4")
	if err != nil || joined.AMFPointer != pointers["node-2"] {
		t.Fatalf("the new member = %+v, %v; want the released AMF Pointer %d", joined, err, pointers["node-2"])
	}

	if err := f.db.PutGeneratedSEPPIdentity(ctx, joined.AMFPointer); err != nil {
		t.Fatalf("PutGeneratedSEPPIdentity for the new member: %v", err)
	}
}
