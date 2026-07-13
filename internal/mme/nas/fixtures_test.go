// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/s1ap"
)

var (
	testUEIP      = netip.AddrFrom4([4]byte{10, 45, 0, 2})
	testUEIPv6IID = [8]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
	testSGWFTEID  = models.FTEID{TEID: 0x1234, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 2})}
)

// connectedBearerUE returns a secured UE with a recorded default-bearer QoS, so a
// reconcile against an unchanged policy is a no-op.
func connectedBearerUE(t *testing.T, m *mme.MME) (*mme.UeContext, *captureConn) {
	t.Helper()

	ue, cc := securedUE(t, m)
	p := testPDN(ue)
	p.Apn = "internet"

	if qos, err := mme.ResolveQoSByAPN(context.Background(), m, ue.IMSI(), p.Apn); err == nil {
		p.SessAmbrDLBps = mme.BitRateToBps(qos.SessAmbrDLStr)
		p.SessAmbrULBps = mme.BitRateToBps(qos.SessAmbrULStr)
		p.Qci = qos.QCI
		p.Arp = qos.ARP
	}

	return ue, cc
}

// initialUEMessagePDU builds an S1AP Initial UE Message carrying nas.
func initialUEMessagePDU(t *testing.T, enbID s1ap.ENBUES1APID, nas []byte) []byte {
	t.Helper()

	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}

	b, err := (&s1ap.InitialUEMessage{
		ENBUES1APID:           enbID,
		NASPDU:                s1ap.NASPDU(nas),
		TAI:                   s1ap.TAI{PLMNIdentity: plmn, TAC: 1},
		EUTRANCGI:             s1ap.EUTRANCGI{PLMNIdentity: plmn, CellID: 1},
		RRCEstablishmentCause: s1ap.RRCCauseEmergency,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// initiatingValue unwraps an S1AP InitiatingMessage to its procedure value.
func initiatingValue(t *testing.T, b []byte) []byte {
	t.Helper()

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok {
		t.Fatalf("expected InitiatingMessage, got %T", pdu)
	}

	return im.Value
}

// fakeSessionManager stands in for the SMF+PGW-C anchor.
type fakeSessionManager struct {
	lastRequest models.EPSBearerRequest
	modifiedENB models.FTEID
	released    bool
	deactivated bool
}

func (f *fakeSessionManager) CreateEPSSession(_ context.Context, req models.EPSBearerRequest) (models.EPSBearer, error) {
	f.lastRequest = req

	pdnType := req.RequestedPDNType
	if pdnType == 0 {
		pdnType = 1
	}

	bearer := models.EPSBearer{PDNType: pdnType, SGW: testSGWFTEID}
	if pdnType == 1 || pdnType == 3 {
		bearer.IPv4 = testUEIP
	}

	return bearer, nil
}

func (f *fakeSessionManager) ModifyEPSSession(_ context.Context, _ string, _ uint8, enb models.FTEID) error {
	f.modifiedENB = enb

	return nil
}

func (f *fakeSessionManager) UpdateEPSSessionAMBR(_ context.Context, _ string, _ uint8, _, _ string) error {
	return nil
}

func (f *fakeSessionManager) DeactivateEPSSession(_ context.Context, _ string, _ uint8) error {
	f.deactivated = true

	return nil
}

func (f *fakeSessionManager) ReleaseEPSSession(_ context.Context, _ string) error {
	f.released = true

	return nil
}

func (f *fakeSessionManager) FramedRoutesChanged(_ context.Context, _ string, _ uint8) (bool, error) {
	return false, nil
}

// fakeBearerStore resolves a fixed default-bearer QoS for any subscriber.
type erroringSessionManager struct{ fakeSessionManager }

func (erroringSessionManager) CreateEPSSession(context.Context, models.EPSBearerRequest) (models.EPSBearer, error) {
	return models.EPSBearer{}, fmt.Errorf("no IP pool available for DNN")
}

type barredBearerStore struct{ fakeBearerStore }

func (barredBearerStore) GetProfileByID(_ context.Context, id string) (*db.Profile, error) {
	return &db.Profile{ID: id, UeAmbrDownlink: "1 Gbps", UeAmbrUplink: "1 Gbps", Allow4G: false, Allow5G: true}, nil
}

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
	return &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["1"]`, Ciphering: `["AES"]`, Integrity: `["AES"]`}, nil
}

func (fakeBearerStore) NodeID() int { return 1 }

// testSubscriber is the TS 35.208 test-set-1 key material used across the tests.
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
// seeded with testSubscriber, with the NAS layer wired in.
func newTestMME(t *testing.T) *mme.MME {
	t.Helper()

	m := mme.New(udm.New(newFakeCredStore(), noopKeyResolver), fakeBearerStore{}, &fakeSessionManager{})
	m.NAS = &nasHandler{m: m}

	return m
}

// nasHandler implements mme.NASHandler over the in-package handlers, so the
// kernel's S1AP layer dispatches NAS in tests.
type nasHandler struct{ m *mme.MME }

func (h *nasHandler) HandleNAS(ctx context.Context, conn *mme.UeConn, pdu []byte) {
	HandleNAS(ctx, h.m, conn, pdu)
}

func (h *nasHandler) HandleServiceRequest(ctx context.Context, conn mme.S1APWriter, msg *s1ap.InitialUEMessage) {
	HandleServiceRequest(ctx, h.m, conn, msg)
}

// servedAttachTAI is a TAI in the test operator's served area (PLMN 001/01, TAC 1),
// as an INITIAL UE MESSAGE reports for an admitted attach.
var servedAttachTAI = s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}

// newAttachUe registers a test UE with a served serving TAI (as HandleInitialUEMessage
// sets in production) so the attach clears the serving-area gate (TS 24.301 §5.5.1.2.5).
func newAttachUe(m *mme.MME, conn mme.S1APWriter, enbUEID s1ap.ENBUES1APID) *mme.UeContext {
	ue := m.NewUe(conn, enbUEID)
	if ue != nil {
		ue.Conn().ServingTAI = servedAttachTAI
	}

	return ue
}

// securedUE returns a registered UE with a valid EPS NAS security context.
func securedUE(t *testing.T, m *mme.MME) (*mme.UeContext, *captureConn) {
	t.Helper()

	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	kasme := make([]byte, 32)
	for i := range kasme {
		kasme[i] = byte(i + 1)
	}

	if err := ue.SetSecurityContextForTest(kasme, 2, 2); err != nil {
		t.Fatal(err)
	}

	ue.Conn().MarkSecureExchangeEstablished()
	ue.ForceStateForTest(mme.EMMRegistered)
	m.RegisterUEForTest(ue, testSubscriber.IMSI)

	return ue, cc
}

// testPDN returns the UE's default PDN connection, creating it if absent.
func testPDN(ue *mme.UeContext) *mme.PdnConnection {
	ue.DefaultEBI = mme.DefaultERABID

	return ue.EnsurePDN(mme.DefaultERABID)
}

func parseUEContextReleaseCommand(t *testing.T, pdu []byte) *s1ap.UEContextReleaseCommand {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command, got %T", msg)
	}

	cmd, err := s1ap.ParseUEContextReleaseCommand(im.Value)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	return cmd
}

// establishResumeForTest binds a UE returning from ECM-IDLE to a fresh verified S1
// connection, the resume primitives HandleServiceRequest uses.
func establishResumeForTest(m *mme.MME, ue *mme.UeContext, conn mme.S1APWriter, enbUEID s1ap.ENBUES1APID) {
	c := m.NewUeConn(conn, enbUEID)
	c.ServingTAI = servedAttachTAI // set from the resume's INITIAL UE MESSAGE in production
	m.AttachUeConn(ue, c)
	c.MarkSecureExchangeEstablished()
}
