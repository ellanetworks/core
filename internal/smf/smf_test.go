// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf_test

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

// --- Fakes ---

type fakeStore struct {
	mu            sync.Mutex
	allocatedIP   netip.Addr
	releasedIP    netip.Addr
	usageLog      []usageEntry
	flowLog       []models.FlowReportRequest
	releasedIPs   []string
	err           error
	allocateIPErr error
}

type fakePCF struct {
	mu     sync.Mutex
	policy *smf.Policy
	err    error
}

type usageEntry struct {
	imsi          string
	uplinkBytes   uint64
	downlinkBytes uint64
}

func (f *fakeStore) AllocateIP(_ context.Context, _ string, _ string, _ uint8) (netip.Addr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.allocateIPErr != nil {
		return f.allocatedIP, f.allocateIPErr
	}

	return f.allocatedIP, f.err
}

func (f *fakeStore) ReleaseIP(_ context.Context, imsi string, _ string, _ uint8) (netip.Addr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.releasedIPs = append(f.releasedIPs, imsi)

	return f.releasedIP, f.err
}

func (f *fakePCF) GetSessionPolicy(_ context.Context, _ string, _ *models.Snssai, _ string) (*smf.Policy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return nil, f.err
	}

	if f.policy == nil {
		return nil, fmt.Errorf("policy not found")
	}

	return f.policy, nil
}

func (f *fakeStore) IncrementDailyUsage(_ context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.usageLog = append(f.usageLog, usageEntry{imsi, uplinkBytes, downlinkBytes})

	return f.err
}

func (f *fakeStore) InsertFlowReports(_ context.Context, reports []*models.FlowReportRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, r := range reports {
		f.flowLog = append(f.flowLog, *r)
	}

	return f.err
}

type fakeUPF struct {
	mu              sync.Mutex
	establishResult *models.EstablishResponse
	lastEstablish   *models.EstablishRequest
	modifyCalls     []*models.ModifyRequest
	deleteCalls     []deletionCall
	err             error
}

type deletionCall struct {
	remoteSEID uint64
}

func (f *fakeUPF) EstablishSession(_ context.Context, req *models.EstablishRequest) (*models.EstablishResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastEstablish = req

	return f.establishResult, f.err
}

func (f *fakeUPF) ModifySession(_ context.Context, req *models.ModifyRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.modifyCalls = append(f.modifyCalls, req)

	return f.err
}

func (f *fakeUPF) DeleteSession(_ context.Context, remoteSEID uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deleteCalls = append(f.deleteCalls, deletionCall{remoteSEID})

	return f.err
}

func (f *fakeUPF) UpdateFilters(_ context.Context, _ int64, _ models.Direction, _ []models.FilterRule) error {
	return nil
}

type fakeAMF struct {
	mu        sync.Mutex
	n1Calls   []n1Call
	n1n2Calls []n1n2Call
	pageCalls []pageCall
	err       error
}

type n1Call struct {
	supi         etsi.SUPI
	pduSessionID uint8
	n1Msg        []byte
}

type n1n2Call struct {
	supi         etsi.SUPI
	pduSessionID uint8
	snssai       *models.Snssai
	n1Msg        []byte
	n2Msg        []byte
}

type pageCall struct {
	supi         etsi.SUPI
	pduSessionID uint8
	snssai       *models.Snssai
	n2Msg        []byte
}

func (f *fakeAMF) TransferN1(_ context.Context, supi etsi.SUPI, n1Msg []byte, pduSessionID uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.n1Calls = append(f.n1Calls, n1Call{supi, pduSessionID, n1Msg})

	return f.err
}

func (f *fakeAMF) TransferN1N2(_ context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n1Msg, n2Msg []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.n1n2Calls = append(f.n1n2Calls, n1n2Call{supi, pduSessionID, snssai, n1Msg, n2Msg})

	return f.err
}

func (f *fakeAMF) N2TransferOrPage(_ context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n2Msg []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pageCalls = append(f.pageCalls, pageCall{supi, pduSessionID, snssai, n2Msg})

	return f.err
}

// --- Test helpers ---

const (
	testIMSI = "001010000000001"
	testDNN  = "internet"
)

var testSnssai = &models.Snssai{Sst: 1, Sd: "010203"}

func testSUPI() etsi.SUPI {
	supi, err := etsi.NewSUPIFromPrefixed("imsi-" + testIMSI)
	if err != nil {
		panic(fmt.Sprintf("bad test SUPI: %v", err))
	}

	return supi
}

func newTestSMF(pcf smf.PCF, store smf.SessionStore, upf smf.UPFClient, amfCb smf.AMFCallback) *smf.SMF {
	return smf.New(pcf, store, upf, amfCb)
}

func defaultFakes() (*fakePCF, *fakeStore, *fakeUPF, *fakeAMF) {
	pcf := &fakePCF{
		policy: &smf.Policy{
			Ambr: models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
			QosData: models.QosData{
				Var5qi: 9,
				Arp:    &models.Arp{PriorityLevel: 1},
				QFI:    1,
			},
			DNS: net.ParseIP("8.8.8.8").To4(),
			MTU: 1500,
		},
	}
	store := &fakeStore{
		allocatedIP: netip.MustParseAddr("10.0.0.1"),
		releasedIP:  netip.MustParseAddr("10.0.0.1"),
	}
	upf := &fakeUPF{
		establishResult: &models.EstablishResponse{
			RemoteSEID: 100,
			CreatedPDRs: []models.CreatedPDR{
				{PDRID: 1, TEID: 5000, N3IP: netip.MustParseAddr("192.168.1.1")},
			},
		},
	}
	amfCb := &fakeAMF{}

	return pcf, store, upf, amfCb
}

// --- Session Pool Tests ---

func TestNewSession_AddsToPool(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()

	smCtx := s.NewSession(supi, 1, testDNN, testSnssai)
	if smCtx == nil {
		t.Fatal("expected non-nil SMContext")
	}

	if smCtx.PDUSessionID != 1 {
		t.Fatalf("expected PDUSessionID 1, got %d", smCtx.PDUSessionID)
	}

	if smCtx.Dnn != testDNN {
		t.Fatalf("expected DNN %s, got %s", testDNN, smCtx.Dnn)
	}

	ref := smf.CanonicalName(supi, 1)

	got := s.GetSession(ref)
	if got != smCtx {
		t.Fatal("GetSession should return the same context")
	}
}

func TestGetSession_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	got := s.GetSession("nonexistent-ref")
	if got != nil {
		t.Fatal("expected nil for non-existent session")
	}
}

func TestRemoveSession_RemovesFromPool(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()
	bgCtx := context.Background()

	s.NewSession(supi, 1, testDNN, testSnssai)
	ref := smf.CanonicalName(supi, 1)

	s.RemoveSession(bgCtx, ref)

	got := s.GetSession(ref)
	if got != nil {
		t.Fatal("session should have been removed")
	}
}

func TestRemoveSession_ReleasesIP(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()
	bgCtx := context.Background()

	smCtx := s.NewSession(supi, 1, testDNN, testSnssai)
	smCtx.PDUAddress = net.ParseIP("10.0.0.1").To4()
	ref := smf.CanonicalName(supi, 1)

	s.RemoveSession(bgCtx, ref)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPs) != 1 || store.releasedIPs[0] != testIMSI {
		t.Fatalf("expected IP release for %s, got %v", testIMSI, store.releasedIPs)
	}
}

func TestRemoveSession_NonExistent_NoOp(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	bgCtx := context.Background()

	s.RemoveSession(bgCtx, "nonexistent-ref")

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPs) != 0 {
		t.Fatal("should not release IP for non-existent session")
	}
}

func TestSessionCount(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()

	if s.SessionCount() != 0 {
		t.Fatal("expected 0 sessions initially")
	}

	s.NewSession(supi, 1, testDNN, testSnssai)

	if s.SessionCount() != 1 {
		t.Fatal("expected 1 session")
	}

	s.NewSession(supi, 2, testDNN, testSnssai)

	if s.SessionCount() != 2 {
		t.Fatal("expected 2 sessions")
	}
}

func TestSessionsByDNN(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()

	s.NewSession(supi, 1, "internet", testSnssai)
	s.NewSession(supi, 2, "ims", testSnssai)
	s.NewSession(supi, 3, "internet", testSnssai)

	internet := s.SessionsByDNN("internet")
	if len(internet) != 2 {
		t.Fatalf("expected 2 internet sessions, got %d", len(internet))
	}

	ims := s.SessionsByDNN("ims")
	if len(ims) != 1 {
		t.Fatalf("expected 1 ims session, got %d", len(ims))
	}

	none := s.SessionsByDNN("nonexistent")
	if len(none) != 0 {
		t.Fatalf("expected 0 sessions for nonexistent DNN, got %d", len(none))
	}
}

func TestGetSessionBySEID(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()

	smCtx := s.NewSession(supi, 1, testDNN, testSnssai)

	seid := s.AllocateLocalSEID()
	smCtx.SetPFCPSession(seid)

	got := s.GetSessionBySEID(seid)
	if got != smCtx {
		t.Fatal("expected to find session by SEID")
	}

	got = s.GetSessionBySEID(999)
	if got != nil {
		t.Fatal("expected nil for non-existent SEID")
	}
}

func TestAllocateLocalSEID_Increments(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	seid1 := s.AllocateLocalSEID()
	seid2 := s.AllocateLocalSEID()
	seid3 := s.AllocateLocalSEID()

	if seid1 != 1 || seid2 != 2 || seid3 != 3 {
		t.Fatalf("expected SEIDs 1,2,3 but got %d,%d,%d", seid1, seid2, seid3)
	}
}

// --- PDR/FAR/QER/URR Allocation Tests ---

func TestNewPDR_AllocatesIDAndFAR(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	pdr, err := s.NewPDR()
	if err != nil {
		t.Fatalf("NewPDR failed: %v", err)
	}

	if pdr.PDRID == 0 {
		t.Fatal("expected non-zero PDR ID")
	}

	if pdr.FAR == nil {
		t.Fatal("expected non-nil FAR")
	}

	if !pdr.FAR.ApplyAction.Drop {
		t.Fatal("expected default FAR action to be Drop")
	}
}

func TestNewPDR_UniqueIDs(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	pdr1, err := s.NewPDR()
	if err != nil {
		t.Fatalf("NewPDR 1 failed: %v", err)
	}

	pdr2, err := s.NewPDR()
	if err != nil {
		t.Fatalf("NewPDR 2 failed: %v", err)
	}

	if pdr1.PDRID == pdr2.PDRID {
		t.Fatal("PDR IDs should be unique")
	}
}

func TestNewQER_SetsPolicy(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	policy := &smf.Policy{
		Ambr: models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
		QosData: models.QosData{
			Var5qi: 9,
			Arp:    &models.Arp{PriorityLevel: 1},
			QFI:    5,
		},
	}

	qer, err := s.NewQER(policy)
	if err != nil {
		t.Fatalf("NewQER failed: %v", err)
	}

	if qer.QFI != 5 {
		t.Fatalf("expected QFI 5, got %d", qer.QFI)
	}

	if qer.MBR.ULMBR != 100000 {
		t.Fatalf("expected ULMBR 100000, got %d", qer.MBR.ULMBR)
	}

	if qer.MBR.DLMBR != 200000 {
		t.Fatalf("expected DLMBR 200000, got %d", qer.MBR.DLMBR)
	}

	if qer.GateStatus.ULGate != models.GateOpen || qer.GateStatus.DLGate != models.GateOpen {
		t.Fatal("expected gates to be open")
	}
}

func TestNewURR_DefaultConfig(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	urr, err := s.NewURR()
	if err != nil {
		t.Fatalf("NewURR failed: %v", err)
	}

	if urr.URRID == 0 {
		t.Fatal("expected non-zero URR ID")
	}
}

func TestRemovePDR_FreesID(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	pdr1, _ := s.NewPDR()
	s.RemovePDR(pdr1)
	s.RemoveFAR(pdr1.FAR)

	// After freeing, allocation should still succeed (no error / no exhaustion).
	pdr2, err := s.NewPDR()
	if err != nil {
		t.Fatalf("allocation after free failed: %v", err)
	}

	if pdr2.PDRID == 0 {
		t.Fatal("expected non-zero PDR ID")
	}
}

// --- Store Delegation Tests ---

func TestGetSessionPolicy_DelegatesToStore(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	bgCtx := context.Background()
	supi := testSUPI()

	policy, err := s.GetSessionPolicy(bgCtx, supi, testSnssai, testDNN)
	if err != nil {
		t.Fatalf("GetSessionPolicy failed: %v", err)
	}

	if policy.Ambr.Uplink != "100 Mbps" {
		t.Fatalf("expected uplink 100 Mbps, got %s", policy.Ambr.Uplink)
	}
}

// --- Concurrent Access Tests ---

func TestConcurrentSessionCreation(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func(id uint8) {
			defer wg.Done()

			supi := testSUPI()
			s.NewSession(supi, id, testDNN, testSnssai)
		}(uint8(i))
	}

	wg.Wait()

	if s.SessionCount() != 100 {
		t.Fatalf("expected 100 sessions, got %d", s.SessionCount())
	}
}

func TestNewSession_ReplacesExisting(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()

	s.NewSession(supi, 1, testDNN, testSnssai)
	ctx2 := s.NewSession(supi, 1, "ims", testSnssai)

	ref := smf.CanonicalName(supi, 1)

	got := s.GetSession(ref)
	if got != ctx2 {
		t.Fatal("expected the second session to replace the first")
	}

	if got.Dnn != "ims" {
		t.Fatalf("expected DNN ims, got %s", got.Dnn)
	}

	if s.SessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", s.SessionCount())
	}
}

// --- fakeBGP ---

type fakeBGP struct {
	mu          sync.Mutex
	announced   []string
	withdrawn   []string
	owners      []string
	running     bool
	advertising bool
	announceErr error
	withdrawErr error
}

func (f *fakeBGP) Announce(ip netip.Addr, owner string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.announced = append(f.announced, ip.String())
	f.owners = append(f.owners, owner)

	return f.announceErr
}

func (f *fakeBGP) Withdraw(ip netip.Addr) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.withdrawn = append(f.withdrawn, ip.String())

	return f.withdrawErr
}

func (f *fakeBGP) IsRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.running
}

func (f *fakeBGP) IsAdvertising() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.running && f.advertising
}

// --- BGP Integration Tests ---

func TestRemoveSession_WithdrawsBGPRoute(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	bgpFake := &fakeBGP{running: true, advertising: true}
	s := smf.New(pcf, store, upf, amfCb, smf.WithBGP(bgpFake))
	supi := testSUPI()
	bgCtx := context.Background()

	smCtx := s.NewSession(supi, 1, testDNN, testSnssai)
	smCtx.PDUAddress = net.ParseIP("10.0.0.1")
	ref := smf.CanonicalName(supi, 1)

	s.RemoveSession(bgCtx, ref)

	bgpFake.mu.Lock()
	defer bgpFake.mu.Unlock()

	if len(bgpFake.withdrawn) != 1 {
		t.Fatalf("expected 1 BGP withdraw, got %d", len(bgpFake.withdrawn))
	}

	if bgpFake.withdrawn[0] != "10.0.0.1" {
		t.Fatalf("expected withdraw for 10.0.0.1, got %s", bgpFake.withdrawn[0])
	}
}

func TestRemoveSession_NoBGP_NoWithdraw(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	// No BGP configured
	s := smf.New(pcf, store, upf, amfCb)
	supi := testSUPI()
	bgCtx := context.Background()

	smCtx := s.NewSession(supi, 1, testDNN, testSnssai)
	smCtx.PDUAddress = net.ParseIP("10.0.0.1")
	ref := smf.CanonicalName(supi, 1)

	// Should not panic
	s.RemoveSession(bgCtx, ref)
}

func TestRemoveSession_BGPNotRunning_NoWithdraw(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	bgpFake := &fakeBGP{running: false}
	s := smf.New(pcf, store, upf, amfCb, smf.WithBGP(bgpFake))
	supi := testSUPI()
	bgCtx := context.Background()

	smCtx := s.NewSession(supi, 1, testDNN, testSnssai)
	smCtx.PDUAddress = net.ParseIP("10.0.0.1")
	ref := smf.CanonicalName(supi, 1)

	s.RemoveSession(bgCtx, ref)

	bgpFake.mu.Lock()
	defer bgpFake.mu.Unlock()

	if len(bgpFake.withdrawn) != 0 {
		t.Fatalf("expected 0 BGP withdraws when not running, got %d", len(bgpFake.withdrawn))
	}
}

func TestRemoveSession_NilPDUAddress_NoWithdraw(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	bgpFake := &fakeBGP{running: true, advertising: true}
	s := smf.New(pcf, store, upf, amfCb, smf.WithBGP(bgpFake))
	supi := testSUPI()
	bgCtx := context.Background()

	// Session without PDUAddress (e.g., allocation failed before setting it)
	s.NewSession(supi, 1, testDNN, testSnssai)
	ref := smf.CanonicalName(supi, 1)

	s.RemoveSession(bgCtx, ref)

	bgpFake.mu.Lock()
	defer bgpFake.mu.Unlock()

	if len(bgpFake.withdrawn) != 0 {
		t.Fatalf("expected 0 BGP withdraws for nil PDUAddress, got %d", len(bgpFake.withdrawn))
	}
}
