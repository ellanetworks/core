// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

// --- Fakes ---

type fakeStore struct {
	mu              sync.Mutex
	teardownSeq     *teardownRecorder
	allocatedIP     netip.Addr
	allocatedIPv6   netip.Addr
	releasedIP      netip.Addr
	releasedIPv6    netip.Addr
	usageLog        []usageEntry
	flowLog         []models.FlowReportRequest
	releasedIPs     []string
	releasedIPv6s   []string
	err             error
	allocateIPErr   error
	allocateIPv6Err error
	framedRoutes    []netip.Prefix
	framedRoutesErr error
	staticIPv4      netip.Addr
	staticIPv6      netip.Addr
	staticIPErr     error
	opLog           []string
	allocSessionLog []uint8
}

func (f *fakeStore) ResolveDNN(_ context.Context, _ string) (smf.DNNStore, error) {
	return f, nil
}

// ops returns the IPv4 allocate/release calls in the order they arrived.
func (f *fakeStore) ops() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.opLog)
}

// allocSessionIDs returns the session key id of each IPv4 allocation in order.
func (f *fakeStore) allocSessionIDs() []uint8 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.allocSessionLog)
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

func (f *fakeStore) AllocateIP(_ context.Context, _ string, sessionKeyID uint8) (netip.Addr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.opLog = append(f.opLog, "alloc")
	f.allocSessionLog = append(f.allocSessionLog, sessionKeyID)

	if f.allocateIPErr != nil {
		return f.allocatedIP, f.allocateIPErr
	}

	return f.allocatedIP, f.err
}

func (f *fakeStore) ReleaseIP(_ context.Context, imsi string, _ uint8) (netip.Addr, error) {
	f.teardownSeq.record("release-ip")

	f.mu.Lock()
	defer f.mu.Unlock()

	f.opLog = append(f.opLog, "release")
	f.releasedIPs = append(f.releasedIPs, imsi)

	return f.releasedIP, f.err
}

func (f *fakeStore) AllocateIPv6(_ context.Context, _ string, _ uint8) (netip.Addr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.allocateIPv6Err != nil {
		return f.allocatedIPv6, f.allocateIPv6Err
	}

	return f.allocatedIPv6, f.err
}

func (f *fakeStore) ReleaseIPv6(_ context.Context, imsi string, _ uint8) (netip.Addr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.releasedIPv6s = append(f.releasedIPv6s, imsi)

	return f.releasedIPv6, f.err
}

func (f *fakeStore) ListFramedRoutes(_ context.Context, _ string) ([]netip.Prefix, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.framedRoutes, f.framedRoutesErr
}

func (f *fakeStore) GetStaticIP(_ context.Context, _ string, ipv6 bool) (netip.Addr, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.staticIPErr != nil {
		return netip.Addr{}, false, f.staticIPErr
	}

	addr := f.staticIPv4
	if ipv6 {
		addr = f.staticIPv6
	}

	return addr, addr.IsValid(), nil
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
	mu               sync.Mutex
	teardownSeq      *teardownRecorder
	establishResult  *models.EstablishResponse
	lastEstablish    *models.EstablishRequest
	modifyCalls      []*models.ModifyRequest
	deleteCalls      []deletionCall
	suppressDDNCalls []uint64
	clearDDNCalls    []uint64
	lastIPv6Reg      *models.IPv6SessionRegistration
	err              error
}

type deletionCall struct {
	seid uint64
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

func (f *fakeUPF) DeleteSession(_ context.Context, seid uint64) error {
	f.teardownSeq.record("delete-session")

	f.mu.Lock()
	defer f.mu.Unlock()

	f.deleteCalls = append(f.deleteCalls, deletionCall{seid})

	return f.err
}

func (f *fakeUPF) SuppressDownlinkDataNotification(_ context.Context, seid uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.suppressDDNCalls = append(f.suppressDDNCalls, seid)
}

func (f *fakeUPF) ClearDownlinkDataNotification(_ context.Context, seid uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.clearDDNCalls = append(f.clearDDNCalls, seid)
}

func (f *fakeUPF) FlushUsage(_ context.Context, _ uint64) {}

func (f *fakeUPF) UpdateFilters(_ context.Context, _ string, _ models.Direction, _ []models.FilterRule) error {
	return nil
}

func (f *fakeUPF) RegisterIPv6Session(_ context.Context, reg *models.IPv6SessionRegistration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastIPv6Reg = reg

	return nil
}

func (f *fakeUPF) UnregisterIPv6Session(_ context.Context, _ uint32) error {
	return nil
}

type fakeAMF struct {
	mu           sync.Mutex
	n1Calls      []n1Call
	n1n2Calls    []n1n2Call
	modifyCalls  []n1n2Call
	releaseCalls []releaseCall
	pageCalls    []pageCall
	droppedCalls []droppedCall
	err          error
}

type droppedCall struct {
	supi         etsi.SUPI
	pduSessionID uint8
	ref          string
	n2Transfer   []byte
}

func (f *fakeAMF) SessionDropped(_ context.Context, supi etsi.SUPI, pduSessionID uint8, ref string, n2Transfer []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.droppedCalls = append(f.droppedCalls, droppedCall{supi, pduSessionID, ref, n2Transfer})
}

func (f *fakeAMF) dropped() []droppedCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]droppedCall(nil), f.droppedCalls...)
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

type releaseCall struct {
	supi         etsi.SUPI
	pduSessionID uint8
	n1Msg        []byte
	n2Transfer   []byte
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

func (f *fakeAMF) ModifyN1N2(_ context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Msg []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.modifyCalls = append(f.modifyCalls, n1n2Call{supi, pduSessionID, nil, n1Msg, n2Msg})

	return f.err
}

func (f *fakeAMF) ReleaseSession(_ context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Transfer []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.releaseCalls = append(f.releaseCalls, releaseCall{supi, pduSessionID, n1Msg, n2Transfer})

	return f.err
}

func (f *fakeAMF) N2TransferOrPage(_ context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n2Msg []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pageCalls = append(f.pageCalls, pageCall{supi, pduSessionID, snssai, n2Msg})

	return f.err
}

// fakeMME records 4G paging calls, standing in for the MME's smf.MMECallback.
type fakeMME struct {
	mu           sync.Mutex
	pagedIMSI    []string
	droppedCalls []mmeTransferredCall
	err          error
}

type mmeTransferredCall struct {
	imsi string
	ebi  uint8
	ref  string
}

func (f *fakeMME) SessionDropped(_ context.Context, imsi string, ebi uint8, ref string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.droppedCalls = append(f.droppedCalls, mmeTransferredCall{imsi, ebi, ref})
}

func (f *fakeMME) dropped() []mmeTransferredCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]mmeTransferredCall(nil), f.droppedCalls...)
}

func (f *fakeMME) Page(_ context.Context, imsi string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pagedIMSI = append(f.pagedIMSI, imsi)

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

// Holds the session's Mutex the way production callers do.
func removeSession(s *smf.SMF, ctx context.Context, sc *smf.SMContext) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	s.RemoveSession(ctx, sc.Ref)
}

func defaultFakes() (*fakePCF, *fakeStore, *fakeUPF, *fakeAMF) {
	pcf := &fakePCF{
		policy: &smf.Policy{
			Ambr: models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")},
			QosData: models.QosData{
				Var5qi: 9,
				Arp:    &models.Arp{PriorityLevel: 1},
				QFI:    1,
			},
			DNS:      net.ParseIP("8.8.8.8").To4(),
			MTU:      1500,
			IPv4Pool: "10.0.0.0/24",
			IPv6Pool: "",
		},
	}
	store := &fakeStore{
		allocatedIP: netip.MustParseAddr("10.0.0.1"),
		releasedIP:  netip.MustParseAddr("10.0.0.1"),
	}
	upf := &fakeUPF{
		establishResult: &models.EstablishResponse{
			N3TEID: 5000,
			N3IPv4: netip.MustParseAddr("192.168.1.1"),
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

	smCtx, _ := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)
	if smCtx == nil {
		t.Fatal("expected non-nil SMContext")
	}

	if smCtx.PDUSessionID != 1 {
		t.Fatalf("expected PDUSessionID 1, got %d", smCtx.PDUSessionID)
	}

	if smCtx.Dnn != testDNN {
		t.Fatalf("expected DNN %s, got %s", testDNN, smCtx.Dnn)
	}

	ref := smCtx.Ref

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

	smCtx, _ := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)

	removeSession(s, bgCtx, smCtx)

	got := s.GetSession(smCtx.Ref)
	if got != nil {
		t.Fatal("session should have been removed")
	}
}

func TestRemoveSession_ReleasesIP(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()
	bgCtx := context.Background()

	smCtx, _ := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)
	smCtx.PDUIPV4Address = net.ParseIP("10.0.0.1").To4()

	removeSession(s, bgCtx, smCtx)

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

	_, _ = s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)

	if s.SessionCount() != 1 {
		t.Fatal("expected 1 session")
	}

	_, _ = s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 2}, testDNN, testSnssai)

	if s.SessionCount() != 2 {
		t.Fatal("expected 2 sessions")
	}
}

func TestSessionsByDNN(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()

	_, _ = s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, "internet", testSnssai)
	_, _ = s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 2}, "ims", testSnssai)
	_, _ = s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 3}, "internet", testSnssai)

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

	smCtx, _ := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)

	seid := s.AllocateSEID()
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

func TestAllocateSEID_Increments(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	seid1 := s.AllocateSEID()
	seid2 := s.AllocateSEID()
	seid3 := s.AllocateSEID()

	if seid1 != 1 || seid2 != 2 || seid3 != 3 {
		t.Fatalf("expected SEIDs 1,2,3 but got %d,%d,%d", seid1, seid2, seid3)
	}
}

// --- PDR/FAR/QER/URR Construction Tests ---

func TestNewPDR_BuildsPDRWithFAR(t *testing.T) {
	pdr := smf.NewPDR(1, 1)

	if pdr.PDRID != 1 {
		t.Fatalf("expected PDR ID 1, got %d", pdr.PDRID)
	}

	if pdr.FAR == nil || pdr.FAR.FARID != 1 {
		t.Fatal("expected FAR with ID 1")
	}

	if !pdr.FAR.ApplyAction.Drop {
		t.Fatal("expected default FAR action to be Drop")
	}
}

func TestNewQER_SetsPolicy(t *testing.T) {
	policy := &smf.Policy{
		Ambr: models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")},
		QosData: models.QosData{
			Var5qi: 9,
			Arp:    &models.Arp{PriorityLevel: 1},
			QFI:    5,
		},
	}

	qer := smf.NewQER(policy, 1)

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

	if !policy.Ambr.Uplink.Equal(models.MustParseBitRate("100 Mbps")) {
		t.Fatalf("expected uplink 100 Mbps, got %s", policy.Ambr.Uplink)
	}
}

// --- Concurrent Access Tests ---

func TestConcurrentSessionCreation(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)

		go func(n int) {
			defer wg.Done()

			supi, err := etsi.NewSUPIFromIMSI(fmt.Sprintf("0010100000%05d", n))
			if err != nil {
				t.Error(err)
				return
			}

			if _, err := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai); err != nil {
				t.Errorf("NewSession for %s: %v", supi, err)
			}
		}(i)
	}

	wg.Wait()

	if s.SessionCount() != 100 {
		t.Fatalf("expected 100 sessions, got %d", s.SessionCount())
	}
}

func TestNewSession_ClaimsAnIdentityOnce(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()

	ctx1, err := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, "ims", testSnssai); err == nil {
		t.Fatal("a second session claimed a PDU session identity a live session already holds")
	}

	for _, id := range []uint8{0, 16, 64} {
		if _, err := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: id}, testDNN, testSnssai); err == nil {
			t.Errorf("NewSession accepted PDU session identity %d, which no UE may allocate", id)
		}
	}

	s.RemoveSession(t.Context(), ctx1.Ref)

	ctx2, err := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, "ims", testSnssai)
	if err != nil {
		t.Fatal(err)
	}

	if ctx1.Ref == ctx2.Ref {
		t.Fatalf("the replacing session must get a distinct ref, got %q twice", ctx1.Ref)
	}

	if got := s.GetSession(ctx2.Ref); got != ctx2 {
		t.Fatal("the replacing session must be retrievable by its own ref")
	} else if got.Dnn != "ims" {
		t.Fatalf("expected DNN ims, got %s", got.Dnn)
	}

	if s.SessionCount() != 1 {
		t.Fatalf("expected the released session to be gone, got %d sessions", s.SessionCount())
	}
}

func TestNewSession_NamespacesAreDisjoint(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()

	fiveG, err := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 5}, testDNN, testSnssai)
	if err != nil {
		t.Fatal(err)
	}

	fourG, err := s.NewSession(supi, smf.Access4G, smf.SessionIdentity{EBI: 5}, testDNN, nil)
	if err != nil {
		t.Fatalf("EPS bearer identity 5 was refused though PDU session identity 5 is a different session: %v", err)
	}

	if fiveG.Ref == fourG.Ref {
		t.Fatal("the two sessions share a ref")
	}

	if s.SessionCount() != 2 {
		t.Fatalf("expected 2 sessions, got %d", s.SessionCount())
	}
}

// Route announce/withdraw is driven by the BGP reconciler reading the
// replicated ip_leases table; see internal/bgp/reconciler.go and its tests.
