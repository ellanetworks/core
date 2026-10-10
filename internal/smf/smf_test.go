// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/udm"
)

// --- Fakes ---

type fakeStore struct {
	mu              sync.Mutex
	apnLookups      int
	pcf             *fakePCF
	teardownSeq     *teardownRecorder
	allocatedIP     netip.Addr
	allocatedIPv6   netip.Addr
	releasedIP      netip.Addr
	releasedIPv6    netip.Addr
	usageLog        []usageEntry
	batchCalls      int
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
	f.mu.Lock()
	defer f.mu.Unlock()

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

type reportedRule struct {
	smf.RuleReport
	cause smf.EnforcementFailure
}

type fakePCF struct {
	mu           sync.Mutex
	policy       *smf.Policy
	err          error
	lastSnssai   *models.Snssai
	associations map[string]smf.PolicyContext
	terminated   []string
	failures     []string
	failedRules  []string
	reports      []reportedRule
	updates      int
	revision     uint64
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

func (f *fakeStore) FramedRoutes(_ context.Context, _, _ string) ([]netip.Prefix, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.framedRoutes, f.framedRoutesErr
}

func (f *fakeStore) StaticIP(_ context.Context, _, _ string, ipv6 bool) (netip.Addr, bool, error) {
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

func (f *fakePCF) decisionLocked(snssai models.Snssai) (*smf.PolicyDecision, error) {
	f.lastSnssai = &snssai

	if f.err != nil {
		return nil, f.err
	}

	if f.policy == nil {
		return nil, fmt.Errorf("policy not found")
	}

	f.revision++

	d := &smf.PolicyDecision{Revision: f.revision, PolicyID: f.policy.PolicyID, Var5qi: f.policy.QosData.Var5qi, SessionAMBR: f.policy.Ambr}
	if f.policy.QosData.Arp != nil {
		d.Arp = f.policy.QosData.Arp.PriorityLevel
	}

	return d, nil
}

func (f *fakePCF) CreateAssociation(_ context.Context, ref string, c smf.PolicyContext) (*smf.PolicyDecision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	d, err := f.decisionLocked(c.Snssai)
	if err != nil {
		return nil, err
	}

	if f.associations == nil {
		f.associations = make(map[string]smf.PolicyContext)
	}

	f.associations[ref] = c

	return d, nil
}

func (f *fakePCF) UpdateAssociation(_ context.Context, ref string, subscribed smf.SubscribedQoS) (*smf.PolicyDecision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	c, ok := f.associations[ref]
	if !ok {
		return nil, fmt.Errorf("no policy association %q", ref)
	}

	c.Subscribed = subscribed
	f.associations[ref] = c
	f.updates++

	return f.decisionLocked(c.Snssai)
}

func (f *fakePCF) ReportEnforcementFailure(ref string, reports []smf.RuleReport, cause smf.EnforcementFailure) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failures = append(f.failures, ref)

	for _, r := range reports {
		f.failedRules = append(f.failedRules, r.Rule.ID)
		f.reports = append(f.reports, reportedRule{RuleReport: r, cause: cause})
	}
}

func (f *fakePCF) subscribed() (smf.SubscribedQoS, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return smf.SubscribedQoS{}, f.err
	}

	if f.policy == nil {
		return smf.SubscribedQoS{}, smf.ErrNoPolicyMatch
	}

	q := smf.SubscribedQoS{Var5qi: f.policy.QosData.Var5qi, SessionAMBR: f.policy.Ambr}
	if f.policy.QosData.Arp != nil {
		q.Arp = f.policy.QosData.Arp.PriorityLevel
	}

	return q, nil
}

func (f *fakePCF) dataNetwork() smf.DataNetworkConfig {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.policy == nil {
		return smf.DataNetworkConfig{}
	}

	return smf.DataNetworkConfig{DNS: f.policy.DNS, MTU: f.policy.MTU, IPv4Pool: f.policy.IPv4Pool, IPv6Pool: f.policy.IPv6Pool, PCSCF: f.policy.PCSCF}
}

func (f *fakeStore) SessionManagement(_ context.Context, _ string) (*udm.SessionManagementSubscription, error) {
	f.mu.Lock()
	f.apnLookups++
	pcf := f.pcf
	f.mu.Unlock()

	c := udm.DNNConfiguration{Snssai: *testSnssai, DNN: testDNN, Default: true}

	if pcf != nil {
		q, err := pcf.subscribed()

		switch {
		case errors.Is(err, smf.ErrNoPolicyMatch):
			return &udm.SessionManagementSubscription{}, nil
		case errors.Is(err, smf.ErrDNNNotInSlice):
			c.DNN = "other-" + testDNN
		case err != nil:
			return nil, err
		}

		c.Var5qi, c.Arp, c.SessionAMBR = q.Var5qi, q.Arp, q.SessionAMBR
	}

	other := c
	other.Snssai, other.Default = otherTestSnssai, false

	return &udm.SessionManagementSubscription{DNNs: []udm.DNNConfiguration{c, other}}, nil
}

func (f *fakeStore) Config(context.Context) (smf.DataNetworkConfig, error) {
	f.mu.Lock()
	pcf := f.pcf
	f.mu.Unlock()

	if pcf == nil {
		return smf.DataNetworkConfig{}, nil
	}

	return pcf.dataNetwork(), nil
}

func (f *fakePCF) TerminateAssociation(ref string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.associations[ref]; ok {
		delete(f.associations, ref)
		f.terminated = append(f.terminated, ref)
	}
}

func (f *fakeStore) IncrementDailyUsageBatch(_ context.Context, usages []models.SubscriberUsage) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.batchCalls++

	for _, u := range usages {
		f.usageLog = append(f.usageLog, usageEntry{u.IMSI, u.UplinkVolume, u.DownlinkVolume})
	}

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
	modifyErrBySEID  map[uint64]error
	modifyCalls      []*models.ModifyRequest
	deleteCalls      []deletionCall
	suppressDDNCalls []uint64
	clearDDNCalls    []uint64
	lastIPv6Reg      *models.IPv6SessionRegistration
	forwardingTEID   uint32
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

func (f *fakeUPF) ModifySession(_ context.Context, req *models.ModifyRequest) (*models.ModifyResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.modifyCalls = append(f.modifyCalls, req)

	if err, ok := f.modifyErrBySEID[req.SEID]; ok {
		return nil, err
	}

	if f.err != nil {
		return nil, f.err
	}

	teids := make(map[uint8]uint32)

	for _, pdr := range req.UpdatePDRs {
		if pdr.PDI.LocalFTEID == nil || pdr.PDI.LocalFTEID.ChooseID == 0 {
			continue
		}

		id := pdr.PDI.LocalFTEID.ChooseID
		teids[id] = 0x1000 + uint32(id)

		if id == 4 && f.forwardingTEID != 0 {
			teids[id] = f.forwardingTEID
		}
	}

	return &models.ModifyResponse{ChosenTEIDs: teids}, nil
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
	pageARPs     []*models.Arp

	accessReleases   []uint8
	accessReleaseErr error
	droppedCalls     []droppedCall
	err              error

	flowEBIs    []uint8
	releasedEBI []uint8
	noFreeEBI   bool
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

func (f *fakeAMF) AssignEPSBearerIdentity(_ etsi.SUPI, _ uint8, _ string) (uint8, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.noFreeEBI {
		return 0, errors.New("no free EPS bearer identity")
	}

	ebi := uint8(6 + len(f.flowEBIs))
	f.flowEBIs = append(f.flowEBIs, ebi)

	return ebi, nil
}

func (f *fakeAMF) ReleaseEPSBearerIdentities(_ etsi.SUPI, _ uint8, _ string, ebis []uint8) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.releasedEBI = append(f.releasedEBI, ebis...)
}

func (f *fakeAMF) released() []uint8 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.releasedEBI)
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

func (f *fakeAMF) TransferN1N2(_ context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n1Msg, n2Msg []byte) (models.N1N2MessageTransferCause, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.n1n2Calls = append(f.n1n2Calls, n1n2Call{supi, pduSessionID, snssai, n1Msg, n2Msg})

	return models.N1N2TransferInitiated, f.err
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

func (f *fakeAMF) ReleaseAccessResources(_ context.Context, _ etsi.SUPI, pduSessionID uint8, _ []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.accessReleases = append(f.accessReleases, pduSessionID)

	return f.accessReleaseErr
}

func (f *fakeAMF) releasedAccess() []uint8 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]uint8(nil), f.accessReleases...)
}

func (f *fakeAMF) N2TransferOrPage(_ context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n2Msg []byte, arp *models.Arp) (models.N1N2MessageTransferCause, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pageCalls = append(f.pageCalls, pageCall{supi, pduSessionID, snssai, n2Msg})
	f.pageARPs = append(f.pageARPs, arp)

	return models.N1N2AttemptingToReachUE, f.err
}

// fakeMME records 4G paging calls, standing in for the MME's smf.MMECallback.
type fakeMME struct {
	mu           sync.Mutex
	pagedIMSI    []string
	notifyCauses []models.DownlinkDataNotificationCause
	droppedCalls []mmeTransferredCall
	modified     []models.EPSBearerModification
	reactivated  []uint8
	activations  []models.DedicatedBearerRequest
	deactivated  []uint8
	dedicatedMod []models.DedicatedBearerModification
	pagedEBIs    []uint8
	activateErr  error
	modifyErr    error
	dedicatedErr error
	err          error
}

func (f *fakeMME) ModifyDedicatedBearer(_ context.Context, _ string, _ uint8, mod models.DedicatedBearerModification) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.dedicatedErr != nil {
		return f.dedicatedErr
	}

	f.dedicatedMod = append(f.dedicatedMod, mod)

	return nil
}

func (f *fakeMME) notifiedEBIs() []uint8 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.pagedEBIs)
}

func (f *fakeMME) dedicatedModifications() []models.DedicatedBearerModification {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.dedicatedMod)
}

func (f *fakeMME) ActivateDedicatedBearer(_ context.Context, _ string, req models.DedicatedBearerRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.activateErr != nil {
		return f.activateErr
	}

	f.activations = append(f.activations, req)

	return nil
}

func (f *fakeMME) DeactivateDedicatedBearer(_ context.Context, _ string, ebi uint8, _ uint32) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deactivated = append(f.deactivated, ebi)

	return nil
}

func (f *fakeMME) dedicatedActivations() []models.DedicatedBearerRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.activations)
}

func (f *fakeMME) dedicatedDeactivations() []uint8 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.deactivated)
}

func (f *fakeMME) ModifyEPSBearer(_ context.Context, _ string, _ uint8, mod models.EPSBearerModification) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.modifyErr != nil {
		return f.modifyErr
	}

	f.modified = append(f.modified, mod)

	return nil
}

func (f *fakeMME) ReactivateEPSBearer(_ context.Context, _ string, ebi uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.reactivated = append(f.reactivated, ebi)

	return nil
}

func (f *fakeMME) modifications() []models.EPSBearerModification {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.modified)
}

func (f *fakeMME) reactivations() []uint8 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.reactivated)
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

func (f *fakeMME) NotifyDownlinkData(_ context.Context, imsi string, ebi uint8, cause models.DownlinkDataNotificationCause) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pagedIMSI = append(f.pagedIMSI, imsi)
	f.pagedEBIs = append(f.pagedEBIs, ebi)
	f.notifyCauses = append(f.notifyCauses, cause)

	return f.err
}

// --- Test helpers ---

const (
	testIMSI = "001010000000001"
	testDNN  = "internet"
)

var (
	testSnssai      = &models.Snssai{Sst: 1, Sd: "010203"}
	otherTestSnssai = models.Snssai{Sst: 1, Sd: "000001"}
)

func testSUPI() etsi.SUPI {
	supi, err := etsi.NewSUPIFromPrefixed("imsi-" + testIMSI)
	if err != nil {
		panic(fmt.Sprintf("bad test SUPI: %v", err))
	}

	return supi
}

func newTestSMF(pcf smf.PCF, store smf.SessionStore, upf smf.UPFClient, amfCb smf.AMFCallback, opts ...smf.Option) *smf.SMF {
	if fs, ok := store.(*fakeStore); ok {
		if fp, ok := pcf.(*fakePCF); ok {
			fs.mu.Lock()
			fs.pcf = fp
			fs.mu.Unlock()
		}
	}

	if subs, ok := store.(smf.SubscriptionData); ok {
		opts = append(opts, smf.WithSubscriptions(subs))
	}

	return smf.New(pcf, store, upf, amfCb, opts...)
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
	s.AssignPFCPSession(smCtx, seid)

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
