// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/eps"
)

// idleRegisteredUE returns a registered UE parked in ECM-IDLE — the state just
// before a Service Request or paging.
func idleRegisteredUE(t *testing.T, m *MME) *UeContext {
	t.Helper()

	ue, _ := securedUE(t, m)
	ue.SetUESecurityCapability(eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(), nil, MintAuthProofForAttachRequest())
	testPDN(ue).SgwFTEID = testSGWFTEID

	if _, err := m.ReallocateGUTI(t.Context(), ue, models.PlmnID{Mcc: "001", Mnc: "01"}, 1, 1); err != nil {
		t.Fatal(err)
	}

	// A registered UE holds a registration area, assigned in ATTACH/TAU ACCEPT.
	served, err := m.ServedTAIs(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	ue.AllocateRegistrationArea(served)

	m.FreeUeConn(ue)

	return ue
}

// Fixtures the fake session manager returns.
var (
	testUEIP         = netip.AddrFrom4([4]byte{10, 45, 0, 2})
	testUEIPv6Prefix = netip.MustParseAddr("2001:db8:1::")
	testUEIPv6IID    = [8]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
	testSGWFTEID     = models.FTEID{TEID: 0x1234, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 2})}
)

// fakeSessionManager stands in for the SMF+PGW-C anchor. CreateEPSSession honors
// the requested PDN type so tests can drive IPv4/IPv6/IPv4v6.
type fakeSessionManager struct {
	lastRequest     models.EPSBearerRequest
	modifiedENB     models.FTEID // records the eNB F-TEID from the last ModifyEPSSession
	released        bool
	deactivated     bool
	ambrUpdated     bool
	ambrUplink      string // records the last UpdateEPSSessionAMBR uplink value
	ambrDownlink    string
	ambrErr         error // when set, UpdateEPSSessionAMBR fails with it
	framedChanged   bool  // FramedRoutesChanged returns this
	framedErr       error // when set, FramedRoutesChanged fails with it
	staticIPChanged bool  // StaticIPChanged returns this
	staticIPErr     error // when set, StaticIPChanged fails with it

	clearSuppressionCalls int // counts ClearEPSPagingSuppression calls
}

func (f *fakeSessionManager) CreateEPSSession(_ context.Context, req models.EPSBearerRequest) (models.EPSBearer, error) {
	f.lastRequest = req

	pdnType := req.RequestedPDNType
	if pdnType == 0 {
		pdnType = 1 // IPv4
	}

	bearer := models.EPSBearer{PDNType: pdnType, SGW: testSGWFTEID}

	if pdnType == 1 || pdnType == 3 { // IPv4 / IPv4v6
		bearer.IPv4 = testUEIP
	}

	if pdnType == 2 || pdnType == 3 { // IPv6 / IPv4v6
		bearer.IPv6Prefix = testUEIPv6Prefix
		bearer.IPv6IID = testUEIPv6IID
	}

	return bearer, nil
}

func (f *fakeSessionManager) ModifyEPSSession(_ context.Context, _ string, _ uint8, enb models.FTEID) error {
	f.modifiedENB = enb

	return nil
}

// hookSessionManager runs onModify on the first ModifyEPSSession, so a test can
// simulate a concurrent release (freeing ue.active) during the unlocked user-plane
// switch of a Path Switch or Handover Notify.
func (f *fakeSessionManager) UpdateEPSSessionAMBR(_ context.Context, _ string, _ uint8, ambrUplink, ambrDownlink string) error {
	if f.ambrErr != nil {
		return f.ambrErr
	}

	f.ambrUpdated = true
	f.ambrUplink = ambrUplink
	f.ambrDownlink = ambrDownlink

	return nil
}

func (f *fakeSessionManager) DeactivateEPSSession(_ context.Context, _ string, _ uint8) error {
	f.deactivated = true

	return nil
}

func (f *fakeSessionManager) HandleEPSPagingFailure(_ context.Context, _ string, _ uint8) error {
	return nil
}

func (f *fakeSessionManager) ClearEPSPagingSuppression(_ context.Context, _ string, _ uint8) error {
	f.clearSuppressionCalls++
	return nil
}

func (f *fakeSessionManager) ReleaseEPSSession(_ context.Context, _ string) error {
	f.released = true

	return nil
}

func (f *fakeSessionManager) FramedRoutesChanged(_ context.Context, _ string, _ uint8) (bool, error) {
	return f.framedChanged, f.framedErr
}

func (f *fakeSessionManager) StaticIPChanged(_ context.Context, _ string, _ uint8) (bool, error) {
	return f.staticIPChanged, f.staticIPErr
}

// fakeBearerStore resolves a fixed default-bearer QoS (QCI 9, APN "internet",
// 1 Gbps UE-AMBR) for any subscriber.
type fakeBearerStore struct{}

func (fakeBearerStore) GetSubscriber(_ context.Context, imsi string) (*db.Subscriber, error) {
	return &db.Subscriber{Imsi: imsi, ProfileID: "test-profile"}, nil
}

func (fakeBearerStore) GetProfileByID(_ context.Context, id string) (*db.Profile, error) {
	return &db.Profile{ID: id, UeAmbrDownlink: "1 Gbps", UeAmbrUplink: "1 Gbps", Allow4G: true, Allow5G: true}, nil
}

func (fakeBearerStore) GetDefaultPolicyByProfile(_ context.Context, _ string) (*db.Policy, error) {
	return &db.Policy{Var5qi: 9, Arp: 15, DataNetworkID: "test-dn", IsDefault: true, SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "200 Mbps"}, nil
}

func (fakeBearerStore) ListPoliciesByProfile(_ context.Context, _ string) ([]db.Policy, error) {
	return []db.Policy{
		{Var5qi: 9, Arp: 15, DataNetworkID: "test-dn", IsDefault: true, SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "200 Mbps"},
		{Var5qi: 9, Arp: 15, DataNetworkID: "test-dn-ims"},
	}, nil
}

func (fakeBearerStore) GetDataNetworkByID(_ context.Context, id string) (*db.DataNetwork, error) {
	if id == "test-dn-ims" {
		return &db.DataNetwork{Name: "ims", IPv4Pool: "10.46.0.0/16"}, nil
	}

	return &db.DataNetwork{Name: "internet"}, nil
}

func (fakeBearerStore) GetOperator(_ context.Context) (*db.Operator, error) {
	return &db.Operator{
		Mcc:           "001",
		Mnc:           "01",
		SupportedTACs: `["1"]`,
		Ciphering:     `["AES"]`,
		Integrity:     `["AES"]`,
	}, nil
}

func (fakeBearerStore) NodeID() int { return 1 }

// testSubscriber is the TS 35.208 test-set-1 key material used across the MME
// tests (matching the fake credential store below), so a test acting as the UE
// can recompute RES/AUTS from the same K/OPc.
var testSubscriber = struct {
	IMSI string
	K    [16]byte
	OPc  [16]byte
	SQN  [6]byte
}{
	IMSI: "001010000000001",
	K:    [16]byte{0x46, 0x5b, 0x5c, 0xe8, 0xb1, 0x99, 0xb4, 0x9f, 0xaa, 0x5f, 0x0a, 0x2e, 0xe2, 0x38, 0xa6, 0xbc},
	OPc:  [16]byte{0xcd, 0x63, 0xcb, 0x71, 0x95, 0x4a, 0x9f, 0x4e, 0x48, 0xa5, 0x99, 0x4e, 0x37, 0xa0, 0x2b, 0xaf},
	SQN:  [6]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
}

// fakeCredStore is an in-memory udm.SubscriberStore seeded with testSubscriber.
type fakeCredStore struct {
	mu   sync.Mutex
	subs map[string]*udm.Subscriber
}

func newFakeCredStore() *fakeCredStore {
	return &fakeCredStore{subs: map[string]*udm.Subscriber{
		testSubscriber.IMSI: {
			PermanentKey:   "465b5ce8b199b49faa5f0a2ee238a6bc",
			Opc:            "cd63cb71954a9f4e48a5994e37a02baf",
			SequenceNumber: "000000000001",
		},
	}}
}

func (f *fakeCredStore) GetSubscriber(_ context.Context, imsi string) (*udm.Subscriber, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, ok := f.subs[imsi]
	if !ok {
		return nil, fmt.Errorf("subscriber %s not found", imsi)
	}

	cp := *s

	return &cp, nil
}

func (f *fakeCredStore) UpdateSequenceNumber(_ context.Context, imsi, sqn string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if s, ok := f.subs[imsi]; ok {
		s.SequenceNumber = sqn
	}

	return nil
}

func noopKeyResolver(string, int) (string, error) { return "", nil }

// newTestMME builds an MME backed by a credential authority over a fake store
// seeded with testSubscriber.
func newTestMME(t *testing.T) *MME {
	t.Helper()

	return New(udm.New(newFakeCredStore(), noopKeyResolver), fakeBearerStore{}, &fakeSessionManager{})
}

// testPDN returns the UE's default PDN connection (on the default EPS bearer
// identity), creating it if absent, so tests can set and read per-bearer fields.
func testPDN(ue *UeContext) *PdnConnection {
	ue.DefaultEBI = DefaultERABID

	return ue.EnsurePDN(DefaultERABID)
}

// mustSUPI builds a SUPI from a bare IMSI for test struct literals.
func mustSUPI(imsi string) etsi.SUPI {
	s, _ := etsi.NewSUPIFromIMSI(imsi)
	return s
}
