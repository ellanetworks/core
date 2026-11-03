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

func homeSEPPIdentity(t *testing.T, id string) db.SEPPIdentity {
	t.Helper()

	home := pki.PLMN{MCC: db.InitialMcc, MNC: db.InitialMnc}

	n, err := strconv.Atoi(id)
	if err != nil {
		t.Fatalf("SEPP ID %q: %s", id, err)
	}

	return seppIdentity(t, home, "sepp"+id+"."+home.Domain(), n)
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

func (f *roamingFixture) createHomeSubscriber(t *testing.T, imsi string) {
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
	p.NRFAPIRoot = ptr("http://nrf.5gc.mnc093.mcc208.3gppnetwork.org")
	p.DiameterPeerID = ptr(roamingPeerID)

	if err := f.db.CreateRoamingPartner(ctx, p); err != nil {
		t.Fatalf("CreateRoamingPartner: %s", err)
	}

	got, err := f.db.GetRoamingPartner(ctx, roamingPartnerID)
	if err != nil {
		t.Fatalf("GetRoamingPartner: %s", err)
	}

	if got.Name != "partner" || !slices.Equal(got.PLMNs, plmns("208", "93", "208", "94")) || *got.SEPPFQDN != *p.SEPPFQDN || got.SEPPN32fURI != nil || *got.DiameterPeerID != roamingPeerID {
		t.Fatalf("partner = %+v", got)
	}

	got.Name = "renamed"
	got.PLMNs = plmns("208", "93", "209", "01")
	got.SEPPFQDN, got.SEPPN32cURI, got.SEPPTrustAnchorPEM, got.NRFAPIRoot = nil, nil, nil, nil

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
			p.SEPPFQDN, p.SEPPN32cURI, p.SEPPTrustAnchorPEM, p.NRFAPIRoot = nil, nil, nil, ptr("http://nrf.example.org")
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
		"ftp nrf":            func(p *db.RoamingPartner) { p.NRFAPIRoot = ptr("ftp://nrf.example.org") },
		"garbage anchor":     func(p *db.RoamingPartner) { p.SEPPTrustAnchorPEM = ptr("not a certificate") },
		"empty peer id":      func(p *db.RoamingPartner) { p.DiameterPeerID = ptr("") },
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

	identity := homeSEPPIdentity(t, "1")
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

	valid := homeSEPPIdentity(t, "1")
	other := homeSEPPIdentity(t, "2")

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

	sameFQDN := homeSEPPIdentity(t, "1")
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
	home := pki.PLMN{MCC: db.InitialMcc, MNC: db.InitialMnc}

	created, err := f.db.EnsureSEPPIdentity(ctx, 2)
	if err != nil || !created {
		t.Fatalf("EnsureSEPPIdentity = %v, %v; want true, nil", created, err)
	}

	created, err = f.db.EnsureSEPPIdentity(ctx, 2)
	if err != nil || created {
		t.Fatalf("EnsureSEPPIdentity again = %v, %v; want false, nil", created, err)
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
	if err != nil || two.Source != db.SEPPIdentityGenerated || two.FQDN != home.SEPPFQDN(2) {
		t.Fatalf("generated identity = %+v, %v", two, err)
	}

	cert, err := pki.ParseSEPPCertificate([]byte(two.CertChainPEM), two.FQDN)
	if err != nil || !cert.TrustAnchor().Equal(caCert) {
		t.Fatalf("generated chain does not end with the cluster SEPP CA: %v", err)
	}

	if _, err := f.db.GenerateSEPPIdentity(ctx, 3); err != nil {
		t.Fatalf("GenerateSEPPIdentity: %s", err)
	}

	imported := homeSEPPIdentity(t, "1")
	if err := f.db.PutSEPPIdentity(ctx, &imported); err != nil {
		t.Fatalf("PutSEPPIdentity: %s", err)
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

	if _, err := f.db.GenerateSEPPIdentity(ctx, 1); err != nil {
		t.Fatalf("GenerateSEPPIdentity over an imported one: %s", err)
	}

	if got, err := f.db.GetSEPPIdentity(ctx, 1); err != nil || got.Source != db.SEPPIdentityGenerated {
		t.Fatalf("identity 1 = %+v, %v; want generated", got, err)
	}

	if _, err := f.db.EnsureSEPPIdentity(ctx, 0); err == nil {
		t.Fatal("EnsureSEPPIdentity accepted SEPP ID 0")
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

	rows := []db.RoamingDailyUsage{
		{EpochDay: today, IMSI: visitorIMSI, Mcc: "208", Mnc: "93", BytesUplink: 1, BytesDownlink: 2},
		{EpochDay: today, IMSI: visitorIMSI, Mcc: "208", Mnc: "93", BytesUplink: 10, BytesDownlink: 20},
		{EpochDay: today, IMSI: "208930000000002", Mcc: "208", Mnc: "93", BytesUplink: 100, BytesDownlink: 200},
		{EpochDay: today, IMSI: "310410000000001", Mcc: "310", Mnc: "410", BytesUplink: 5, BytesDownlink: 5},
		{EpochDay: old, IMSI: visitorIMSI, Mcc: "208", Mnc: "93", BytesUplink: 7, BytesDownlink: 7},
	}

	if err := f.db.IncrementRoamingDailyUsageBatch(ctx, rows); err != nil {
		t.Fatalf("IncrementRoamingDailyUsageBatch: %s", err)
	}

	for name, bad := range map[string]db.RoamingDailyUsage{
		"IMSI outside the PLMN": {EpochDay: today, IMSI: "310410000000001", Mcc: "208", Mnc: "93"},
		"bad MNC":               {EpochDay: today, IMSI: "208930000000001", Mcc: "208", Mnc: "9"},
		"negative bytes":        {EpochDay: today, IMSI: "208930000000001", Mcc: "208", Mnc: "93", BytesUplink: -1},
	} {
		if err := f.db.IncrementRoamingDailyUsageBatch(ctx, []db.RoamingDailyUsage{bad}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	all := db.NewDayRange(time.Now().AddDate(0, 0, -30), time.Now().AddDate(0, 0, 1))

	usage, err := f.db.ListRoamingUsage(ctx, "", all)
	if err != nil {
		t.Fatalf("ListRoamingUsage: %s", err)
	}

	want := []db.RoamingUsage{
		{EpochDay: old, Mcc: "208", Mnc: "93", BytesUplink: 7, BytesDownlink: 7},
		{EpochDay: today, Mcc: "208", Mnc: "93", BytesUplink: 111, BytesDownlink: 222},
		{EpochDay: today, Mcc: "310", Mnc: "410", BytesUplink: 5, BytesDownlink: 5},
	}

	if !slices.Equal(usage, want) {
		t.Fatalf("usage = %+v, want %+v", usage, want)
	}

	usage, err = f.db.ListRoamingUsage(ctx, visitorIMSI, db.NewDayRange(time.Now(), time.Now().AddDate(0, 0, 1)))
	if err != nil || len(usage) != 1 || usage[0].BytesUplink != 11 || usage[0].BytesDownlink != 22 {
		t.Fatalf("ListRoamingUsage for one IMSI today = %+v, %v", usage, err)
	}

	if err := f.db.DeleteOldDailyUsage(ctx, 5); err != nil {
		t.Fatalf("DeleteOldDailyUsage: %s", err)
	}

	usage, err = f.db.ListRoamingUsage(ctx, "", all)
	if err != nil || len(usage) != 2 {
		t.Fatalf("ListRoamingUsage after retention = %+v, %v", usage, err)
	}

	if err := f.db.DeleteRoamingPartner(ctx, roamingPartnerID); err != nil {
		t.Fatalf("DeleteRoamingPartner: %s", err)
	}

	usage, err = f.db.ListRoamingUsage(ctx, "", all)
	if err != nil || len(usage) != 2 {
		t.Fatalf("ListRoamingUsage after partner delete = %+v, %v; want the usage kept", usage, err)
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
		visitor.PermanentKey != "" || visitor.Opc != "" || visitor.SequenceNumber != "" || visitor.Msisdn != "" || visitor.ID == "" {
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

	again2, err := f.db.GetSubscriberOrVisitor(ctx, visitorIMSI)
	if err != nil || again2.ID != visitor.ID {
		t.Fatalf("visitor row replaced: %+v, %v", again2, err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, "208930000000009", "missing", db.UERegistrationTypeMME, "node-1", ""); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("visitor of a missing partner: err = %v, want ErrNotFound", err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, "310410000000001", roamingPartnerID, db.UERegistrationTypeMME, "node-1", ""); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("visitor outside the partner PLMN: err = %v, want ErrRoamingConflict", err)
	}

	f.createHomeSubscriber(t, "208930000000005")

	if _, err := f.db.RegisterVisitorUE(ctx, "208930000000005", roamingPartnerID, db.UERegistrationTypeMME, "node-1", ""); !errors.Is(err, db.ErrAlreadyExists) {
		t.Fatalf("visitor over a home subscriber: err = %v, want ErrAlreadyExists", err)
	}

	if home, err := f.db.GetSubscriber(ctx, "208930000000005"); err != nil || home.PermanentKey == "" {
		t.Fatalf("home subscriber touched: %+v, %v", home, err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, visitorIMSI, roamingPartnerID, "bogus", "node-1", ""); err == nil {
		t.Fatal("invalid registration type accepted")
	}

	if _, err := f.db.RegisterVisitorUE(ctx, visitorIMSI, "", db.UERegistrationTypeMME, "node-1", ""); err == nil {
		t.Fatal("missing partner ID accepted")
	}
}

func TestVisitorDeletion(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.registerVisitor(t, visitorIMSI)
	f.registerVisitor(t, "208930000000002")

	deleted, err := f.db.DeleteVisitorIfUnregistered(ctx, visitorIMSI)
	if err != nil || deleted {
		t.Fatalf("DeleteVisitorIfUnregistered while registered = %v, %v", deleted, err)
	}

	if err := f.db.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	deleted, err = f.db.DeleteVisitorIfUnregistered(ctx, visitorIMSI)
	if err != nil || !deleted {
		t.Fatalf("DeleteVisitorIfUnregistered after purge = %v, %v", deleted, err)
	}

	if _, err := f.db.GetSubscriberOrVisitor(ctx, visitorIMSI); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("visitor after delete: err = %v, want ErrNotFound", err)
	}

	if _, err := f.db.GetUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("visitor registration after delete: err = %v, want ErrNotFound", err)
	}

	f.createHomeSubscriber(t, "001010000000001")

	if deleted, err := f.db.DeleteVisitorIfUnregistered(ctx, "001010000000001"); err != nil || deleted {
		t.Fatalf("DeleteVisitorIfUnregistered of a home subscriber = %v, %v", deleted, err)
	}

	if n, err := f.db.DeleteUnregisteredVisitors(ctx); err != nil || n != 0 {
		t.Fatalf("DeleteUnregisteredVisitors with a registered visitor = %d, %v", n, err)
	}

	if err := f.db.PurgeUERegistration(ctx, "208930000000002", db.UERegistrationTypeMME, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	if n, err := f.db.DeleteUnregisteredVisitors(ctx); err != nil || n != 1 {
		t.Fatalf("DeleteUnregisteredVisitors = %d, %v; want 1", n, err)
	}

	if visitors, err := f.db.ListVisitors(ctx); err != nil || len(visitors) != 0 {
		t.Fatalf("ListVisitors after reconcile = %+v, %v", visitors, err)
	}

	if _, err := f.db.GetSubscriber(ctx, "001010000000001"); err != nil {
		t.Fatalf("home subscriber after reconcile: %s", err)
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
	f.createHomeSubscriber(t, "001010000000001")

	p.ProfileID = f.otherID
	if err := f.db.UpdateRoamingPartner(ctx, p); err != nil {
		t.Fatalf("UpdateRoamingPartner: %s", err)
	}

	if visitor, err := f.db.GetSubscriberOrVisitor(ctx, visitorIMSI); err != nil || visitor.ProfileID != f.otherID {
		t.Fatalf("visitor after partner profile change = %+v, %v", visitor, err)
	}

	if home, err := f.db.GetSubscriber(ctx, "001010000000001"); err != nil || home.ProfileID != f.profileID {
		t.Fatalf("home subscriber after partner profile change = %+v, %v", home, err)
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

	if deleted, err := f.db.DeleteVisitorIfUnregistered(ctx, visitorIMSI); err != nil || !deleted {
		t.Fatalf("DeleteVisitorIfUnregistered = %v, %v", deleted, err)
	}

	if err := f.db.DeleteRoamingPartner(ctx, roamingPartnerID); err != nil {
		t.Fatalf("DeleteRoamingPartner without roamers: %s", err)
	}

	if _, err := f.db.GetSubscriber(ctx, "001010000000001"); err != nil {
		t.Fatalf("home subscriber after partner delete: %s", err)
	}
}

func TestHomeSubscriberReadsExcludeVisitors(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.createHomeSubscriber(t, "001010000000001")
	f.registerVisitor(t, visitorIMSI)

	if _, err := f.db.GetSubscriber(ctx, visitorIMSI); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("GetSubscriber of a visitor: err = %v, want ErrNotFound", err)
	}

	if home, err := f.db.GetSubscriberOrVisitor(ctx, "001010000000001"); err != nil || home.RoamingPartnerID != nil {
		t.Fatalf("GetSubscriberOrVisitor of a home subscriber = %+v, %v", home, err)
	}

	subs, total, err := f.db.ListSubscribersPage(ctx, nil, 1, 10)
	if err != nil || total != 1 || len(subs) != 1 || subs[0].Imsi != "001010000000001" {
		t.Fatalf("ListSubscribersPage = %+v, %d, %v", subs, total, err)
	}

	search := "20893"

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

	usage, err := f.db.GetUsagePerSubscriber(ctx, "", db.NewDayRange(time.Now().AddDate(0, 0, -1), time.Now()), db.NoUsageLimit)
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

	stored := homeSEPPIdentity(t, "1")
	if err := f.db.PutSEPPIdentity(ctx, &stored); err != nil {
		t.Fatalf("PutSEPPIdentity: %s", err)
	}

	if _, err := f.db.GenerateSEPPIdentity(ctx, 2); err != nil {
		t.Fatalf("GenerateSEPPIdentity: %s", err)
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

	home, err := database.GetSubscriber(ctx, "001010000000001")
	if err != nil {
		t.Fatalf("GetSubscriber: %s", err)
	}

	roaming, stopRoaming := database.Changefeed().Wakeup(db.TopicRoaming)
	defer stopRoaming()

	registrations, stopRegistrations := database.Changefeed().Wakeup(db.TopicUERegistrations)
	defer stopRegistrations()

	partner := &db.RoamingPartner{ID: roamingPartnerID, Name: "partner", PLMNs: plmns("208", "93"), ProfileID: home.ProfileID}

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

	identity := homeSEPPIdentity(t, "1")

	expectOneRaftEntry(t, database, "SEPP identity", func() error {
		return database.PutSEPPIdentity(ctx, &identity)
	})

	if err := database.DeleteRoamingPartner(ctx, roamingPartnerID); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("DeleteRoamingPartner with a roamer: err = %v, want ErrRoamingConflict", err)
	}

	if err := database.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeAMF3GPPAccess, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	expectOneRaftEntry(t, database, "DeleteVisitorIfUnregistered", func() error {
		_, err := database.DeleteVisitorIfUnregistered(ctx, visitorIMSI)
		return err
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
		if _, err := f.db.EnsureSEPPIdentity(ctx, pointers[nodeID]); err != nil {
			t.Fatalf("EnsureSEPPIdentity: %s", err)
		}
	}

	imported := homeSEPPIdentity(t, strconv.Itoa(pointers["node-3"]))
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

	created, err := f.db.EnsureSEPPIdentity(ctx, joined.AMFPointer)
	if err != nil || !created {
		t.Fatalf("EnsureSEPPIdentity for the new member = %v, %v", created, err)
	}
}
