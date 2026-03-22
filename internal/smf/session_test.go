// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

// --- Fakes ---

type fakeStore struct {
	mu          sync.Mutex
	allocatedIP net.IP
	policy      *smf.Policy
	dnnInfo     *smf.DataNetworkInfo
	usageLog    []usageEntry
	flowLog     []smf.FlowReport
	releasedIPs []string
	err         error
}

type usageEntry struct {
	imsi          string
	uplinkBytes   uint64
	downlinkBytes uint64
}

func (f *fakeStore) AllocateIP(_ context.Context, _, _ string) (net.IP, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.allocatedIP, f.err
}

func (f *fakeStore) ReleaseIP(_ context.Context, supi string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.releasedIPs = append(f.releasedIPs, supi)

	return f.err
}

func (f *fakeStore) GetSubscriberPolicy(_ context.Context, _ string) (*smf.Policy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.policy == nil {
		return nil, fmt.Errorf("policy not found")
	}

	return f.policy, f.err
}

func (f *fakeStore) GetDataNetwork(_ context.Context, _ *models.Snssai, _ string) (*smf.DataNetworkInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.dnnInfo == nil {
		return nil, fmt.Errorf("data network not found")
	}

	return f.dnnInfo, f.err
}

func (f *fakeStore) IncrementDailyUsage(_ context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.usageLog = append(f.usageLog, usageEntry{imsi, uplinkBytes, downlinkBytes})

	return f.err
}

func (f *fakeStore) InsertFlowReport(_ context.Context, report *smf.FlowReport) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.flowLog = append(f.flowLog, *report)

	return f.err
}

type fakeUPF struct {
	mu              sync.Mutex
	establishResult *smf.PFCPEstablishmentResponse
	lastEstablish   *smf.PFCPEstablishmentRequest
	modifyCalls     []*smf.PFCPModificationRequest
	deleteCalls     []deletionCall
	err             error
}

type deletionCall struct {
	localSEID  uint64
	remoteSEID uint64
}

func (f *fakeUPF) EstablishSession(_ context.Context, req *smf.PFCPEstablishmentRequest) (*smf.PFCPEstablishmentResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastEstablish = req

	return f.establishResult, f.err
}

func (f *fakeUPF) ModifySession(_ context.Context, req *smf.PFCPModificationRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.modifyCalls = append(f.modifyCalls, req)

	return f.err
}

func (f *fakeUPF) DeleteSession(_ context.Context, localSEID, remoteSEID uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deleteCalls = append(f.deleteCalls, deletionCall{localSEID, remoteSEID})

	return f.err
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
	supi etsi.SUPI
	req  models.N1N2MessageTransferRequest
}

type pageCall struct {
	supi etsi.SUPI
	req  models.N1N2MessageTransferRequest
}

func (f *fakeAMF) TransferN1(_ context.Context, supi etsi.SUPI, n1Msg []byte, pduSessionID uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.n1Calls = append(f.n1Calls, n1Call{supi, pduSessionID, n1Msg})

	return f.err
}

func (f *fakeAMF) TransferN1N2(_ context.Context, supi etsi.SUPI, req models.N1N2MessageTransferRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.n1n2Calls = append(f.n1n2Calls, n1n2Call{supi, req})

	return f.err
}

func (f *fakeAMF) N2TransferOrPage(_ context.Context, supi etsi.SUPI, req models.N1N2MessageTransferRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pageCalls = append(f.pageCalls, pageCall{supi, req})

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

func newTestSMF(store smf.SessionStore, upf smf.UPFClient, amfCb smf.AMFCallback) *smf.SMF {
	return smf.New(store, upf, amfCb)
}

func defaultFakes() (*fakeStore, *fakeUPF, *fakeAMF) {
	store := &fakeStore{
		allocatedIP: net.ParseIP("10.0.0.1").To4(),
		policy: &smf.Policy{
			Ambr: models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
			QosData: models.QosData{
				Var5qi: 9,
				Arp:    &models.Arp{PriorityLevel: 1},
				QFI:    1,
			},
		},
		dnnInfo: &smf.DataNetworkInfo{
			DNS: net.ParseIP("8.8.8.8").To4(),
			MTU: 1500,
		},
	}
	upf := &fakeUPF{
		establishResult: &smf.PFCPEstablishmentResponse{
			RemoteSEID: 100,
			TEID:       5000,
			N3IP:       net.ParseIP("192.168.1.1").To4(),
		},
	}
	amfCb := &fakeAMF{}

	return store, upf, amfCb
}

// --- Session Pool Tests ---

func TestNewSession_AddsToPool(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
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
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	got := s.GetSession("nonexistent-ref")
	if got != nil {
		t.Fatal("expected nil for non-existent session")
	}
}

func TestRemoveSession_RemovesFromPool(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
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
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	supi := testSUPI()
	bgCtx := context.Background()

	s.NewSession(supi, 1, testDNN, testSnssai)
	ref := smf.CanonicalName(supi, 1)

	s.RemoveSession(bgCtx, ref)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPs) != 1 || store.releasedIPs[0] != testIMSI {
		t.Fatalf("expected IP release for %s, got %v", testIMSI, store.releasedIPs)
	}
}

func TestRemoveSession_NonExistent_NoOp(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	bgCtx := context.Background()

	s.RemoveSession(bgCtx, "nonexistent-ref")

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPs) != 0 {
		t.Fatal("should not release IP for non-existent session")
	}
}

func TestSessionCount(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
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
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
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
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
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
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	seid1 := s.AllocateLocalSEID()
	seid2 := s.AllocateLocalSEID()
	seid3 := s.AllocateLocalSEID()

	if seid1 != 1 || seid2 != 2 || seid3 != 3 {
		t.Fatalf("expected SEIDs 1,2,3 but got %d,%d,%d", seid1, seid2, seid3)
	}
}

// --- PDR/FAR/QER/URR Allocation Tests ---

func TestNewPDR_AllocatesIDAndFAR(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

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
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

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
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

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

	if qer.GateStatus.ULGate != smf.GateOpen || qer.GateStatus.DLGate != smf.GateOpen {
		t.Fatal("expected gates to be open")
	}
}

func TestNewURR_DefaultConfig(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	urr, err := s.NewURR()
	if err != nil {
		t.Fatalf("NewURR failed: %v", err)
	}

	if !urr.MeasurementMethods.Volume {
		t.Fatal("expected Volume measurement method")
	}

	if !urr.ReportingTriggers.PeriodicReporting {
		t.Fatal("expected PeriodicReporting trigger")
	}
}

func TestRemovePDR_FreesID(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

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

func TestGetSubscriberPolicy_DelegatesToStore(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	bgCtx := context.Background()
	supi := testSUPI()

	policy, err := s.GetSubscriberPolicy(bgCtx, supi)
	if err != nil {
		t.Fatalf("GetSubscriberPolicy failed: %v", err)
	}

	if policy.Ambr.Uplink != "100 Mbps" {
		t.Fatalf("expected uplink 100 Mbps, got %s", policy.Ambr.Uplink)
	}
}

func TestGetDataNetwork_DelegatesToStore(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	bgCtx := context.Background()

	info, err := s.GetDataNetwork(bgCtx, testSnssai, testDNN)
	if err != nil {
		t.Fatalf("GetDataNetwork failed: %v", err)
	}

	if info.MTU != 1500 {
		t.Fatalf("expected MTU 1500, got %d", info.MTU)
	}
}

// --- Concurrent Access Tests ---

func TestConcurrentSessionCreation(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

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
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
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
