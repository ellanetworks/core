// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

var testRelocationENBFTEID = models.FTEID{TEID: 0x5678, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 9})}

var testTargetENB = s1ap.GlobalENBID{
	PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
	ENBID:        s1ap.ENBID{Kind: s1ap.ENBIDMacro, Value: 0x00abc},
}

func relocationRequest(sessions ...interworking.PDNConnection) interworking.ForwardRelocationRequest {
	if len(sessions) == 0 {
		sessions = []interworking.PDNConnection{{PDUSessionID: 1, EPSBearerIdentity: 5, APN: "internet"}}
	}

	var (
		kasme [32]byte
		nh    [32]byte
	)

	for i := range kasme {
		kasme[i] = byte(i)
		nh[i] = byte(0xa0 + i)
	}

	return interworking.ForwardRelocationRequest{
		ID:   1,
		SUPI: mustSUPI("001010000000001"),
		SecurityContext: interworking.EPSSecurityContext{
			KASME:      kasme,
			EKSI:       nas.KeySetIdentifier{Value: 4, Mapped: true},
			ULNASCount: 7,
			DLNASCount: 11,
			Algorithms: interworking.EPSNASAlgorithms{
				Ciphering: nas.CipheringAES,
				Integrity: nas.IntegrityAES,
			},
			UESecurityCapability: eps.UESecurityCapability{EEA: 0xe0, EIA: 0xe0},
			NH:                   nh,
			NCC:                  2,
		},
		PDNConnections: sessions,
		Target: interworking.ENBIdentity{
			PlmnID: models.PlmnID{Mcc: "001", Mnc: "01"},
			ID:     0x00abc,
			Bits:   20,
			EPSTAC: 1,
		},
		SourceToTarget: []byte{0xde, 0xad, 0xbe, 0xef},
		UEAMBRUplink:   models.MustParseBitRate("1 Gbps"),
		UEAMBRDownlink: models.MustParseBitRate("2 Gbps"),
	}
}

type relocationTarget struct {
	conn *captureConn
	m    *MME
}

func newRelocationTarget(t *testing.T, m *MME) *relocationTarget {
	t.Helper()

	conn := &captureConn{}
	m.RegisterENBByIDForTest(testTargetENB, conn)

	return &relocationTarget{conn: conn, m: m}
}

func (r *relocationTarget) awaitHandoverRequest(t *testing.T) *s1ap.HandoverRequest {
	t.Helper()

	return r.awaitNthHandoverRequest(t, 0)
}

// awaitNthHandoverRequest returns the n-th Handover Request the target eNB
// received, ignoring whatever else the MME sent it in between.
func (r *relocationTarget) awaitNthHandoverRequest(t *testing.T, n int) *s1ap.HandoverRequest {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for {
		if got := r.handoverRequests(t); len(got) > n {
			return got[n]
		}

		if time.Now().After(deadline) {
			t.Fatalf("target eNB received no Handover Request %d", n)
		}

		time.Sleep(time.Millisecond)
	}
}

func (r *relocationTarget) handoverRequests(t *testing.T) []*s1ap.HandoverRequest {
	t.Helper()

	r.conn.mu.Lock()
	pdus := slices.Clone(r.conn.sent)
	r.conn.mu.Unlock()

	var out []*s1ap.HandoverRequest

	for _, pdu := range pdus {
		msg, err := s1ap.Unmarshal(pdu)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		im, ok := msg.(*s1ap.InitiatingMessage)
		if !ok || im.ProcedureCode != s1ap.ProcHandoverResourceAllocation {
			continue
		}

		req, err := s1ap.ParseHandoverRequest(im.Value)
		if err != nil {
			t.Fatalf("parse Handover Request: %v", err)
		}

		out = append(out, req)
	}

	return out
}

func (r *relocationTarget) admit(t *testing.T, req *s1ap.HandoverRequest, refuse ...uint8) {
	t.Helper()

	ue, ok := r.m.LookupUe(req.MMEUES1APID)
	if !ok {
		t.Fatal("the relocated UE context is not reachable by its MME-UE-S1AP-ID")
	}

	refused := make(map[uint8]struct{}, len(refuse))
	for _, ebi := range refuse {
		refused[ebi] = struct{}{}
	}

	admitted := make([]AdmittedERAB, 0, len(req.ERABToBeSetup))

	for _, b := range req.ERABToBeSetup {
		if _, no := refused[uint8(b.ERABID)]; no {
			continue
		}

		admitted = append(admitted, AdmittedERAB{Ebi: uint8(b.ERABID), EnbFTEID: testRelocationENBFTEID})
	}

	if !r.m.MatchAndSetTargetENB(ue, req.MMEUES1APID, 42, r.conn) {
		t.Fatal("the acknowledge did not match the preparation")
	}

	unadmitted, sourceConn, _, _, ok := r.m.MarkHandoverPrepared(ue, req.MMEUES1APID, r.conn, admitted)
	if !ok {
		t.Fatal("MarkHandoverPrepared refused the acknowledge")
	}

	if sourceConn != nil {
		t.Fatal("a handover out of 5GS must report no source eNB")
	}

	r.m.FinishRelocationPreparation(ue, []byte{0x01, 0x02}, unadmitted)
}

func TestForwardRelocationHandsTheTargetENBTheMappedKeyChain(t *testing.T) {
	m := newTestMME(t)
	target := newRelocationTarget(t, m)
	req := relocationRequest()

	type result struct {
		resp interworking.ForwardRelocationResponse
		err  error
	}

	done := make(chan result, 1)

	go func() {
		resp, err := m.ForwardRelocation(context.Background(), req)
		done <- result{resp, err}
	}()

	hoReq := target.awaitHandoverRequest(t)

	if hoReq.HandoverType != s1ap.HandoverTypeFiveGSToEPS {
		t.Errorf("handover type = %d, want fivegs-to-eps", hoReq.HandoverType)
	}

	if hoReq.SecurityContext.NextHopChainingCount != 2 {
		t.Errorf("NCC = %d, want the 2 the peer derived", hoReq.SecurityContext.NextHopChainingCount)
	}

	if [32]byte(hoReq.SecurityContext.NextHopParameter) != req.SecurityContext.NH {
		t.Error("NH is not the one the peer derived")
	}

	// TS 33.501 §8.3.2 step 4
	want := S1apSecurityCapabilities(relocatedNetworkCapability(req.SecurityContext.UESecurityCapability))
	if hoReq.UESecurityCapabilities != want {
		t.Errorf("UE security capabilities = %+v, want %+v", hoReq.UESecurityCapabilities, want)
	}

	if !bytes.Equal([]byte(hoReq.SourceToTarget), req.SourceToTarget) {
		t.Error("the source-to-target container was not relayed verbatim")
	}

	if hoReq.HandoverRestrictionList == nil {
		t.Fatal("no Handover Restriction List: the target cannot determine the serving PLMN")
	}

	if hoReq.HandoverRestrictionList.ServingPLMN != (s1ap.PLMNIdentity{0x00, 0xf1, 0x10}) {
		t.Errorf("serving PLMN = % x", hoReq.HandoverRestrictionList.ServingPLMN)
	}

	if hoReq.UEAMBR.UL != s1ap.BitRate(req.UEAMBRUplink.Bps()) || hoReq.UEAMBR.DL != s1ap.BitRate(req.UEAMBRDownlink.Bps()) {
		t.Errorf("UE-AMBR = %+v", hoReq.UEAMBR)
	}

	target.admit(t, hoReq)

	got := <-done
	if got.err != nil {
		t.Fatalf("ForwardRelocation: %v", got.err)
	}

	if !bytes.Equal(got.resp.TargetToSource, []byte{0x01, 0x02}) {
		t.Errorf("target-to-source container = % x", got.resp.TargetToSource)
	}

	if len(got.resp.AcceptedPDUSessions) != 1 || got.resp.AcceptedPDUSessions[0] != 1 {
		t.Errorf("accepted PDU sessions = %v, want [1]", got.resp.AcceptedPDUSessions)
	}
}

func TestForwardRelocationTakesOverTheAnchorSessions(t *testing.T) {
	sessions := &fakeSessionManager{}
	m := New(nil, fakeBearerStore{}, sessions)
	target := newRelocationTarget(t, m)
	req := relocationRequest(interworking.PDNConnection{PDUSessionID: 3, EPSBearerIdentity: 7, APN: "internet"})

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	hoReq := target.awaitHandoverRequest(t)
	target.admit(t, hoReq)

	if err := <-done; err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	// TS 23.502 §4.11.1.2.1 step 4
	if sessions.lastRequest.RequestType != eps.RequestTypeHandover {
		t.Errorf("request type = %v, want handover", sessions.lastRequest.RequestType)
	}

	if sessions.lastRequest.EPSBearerIdentity != 7 || sessions.lastRequest.PDUSessionID != 3 {
		t.Errorf("session request = ebi %d, pdu session %d; want the identities the source allocated",
			sessions.lastRequest.EPSBearerIdentity, sessions.lastRequest.PDUSessionID)
	}

	ue, ok := m.LookupUe(hoReq.MMEUES1APID)
	if !ok {
		t.Fatal("the relocated UE context is gone")
	}

	p := m.LookupPDN(ue, 7)
	if p == nil {
		t.Fatal("no PDN connection on the EPS bearer identity the source allocated")
	}

	if !p.Transferred {
		t.Error("the PDN connection is not marked as transferred, so the UE would not be offered ePCO")
	}

	if len(hoReq.ERABToBeSetup) != 1 || hoReq.ERABToBeSetup[0].ERABID != 7 {
		t.Errorf("E-RAB list = %+v", hoReq.ERABToBeSetup)
	}
}

func TestForwardRelocationInstallsTheRelocatedSecurityContext(t *testing.T) {
	m := newTestMME(t)
	target := newRelocationTarget(t, m)
	req := relocationRequest()

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	hoReq := target.awaitHandoverRequest(t)
	target.admit(t, hoReq)

	if err := <-done; err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	ue, ok := m.LookupUe(hoReq.MMEUES1APID)
	if !ok {
		t.Fatal("the relocated UE context is gone")
	}

	if !ue.Secured() {
		t.Error("the relocated UE has no usable NAS security context")
	}

	if ue.EMMState() != EMMRegistrationInitiated {
		t.Errorf("EMM state = %v; the UE is not in EPS until it arrives", ue.EMMState())
	}

	if _, ok := m.LookupUeBySupi(req.SUPI); ok {
		t.Error("a preparation that has not completed must not be the MME's context for the subscriber")
	}
}

func TestForwardRelocationRefusesAnUnknownTargetENB(t *testing.T) {
	m := newTestMME(t)

	req := relocationRequest()
	req.Target.ID = 0x00fff

	if _, err := m.ForwardRelocation(context.Background(), req); !errors.Is(err, ErrUnknownTargetENB) {
		t.Fatalf("error = %v, want ErrUnknownTargetENB", err)
	}
}

func TestForwardRelocationReleasesTheSessionsItOpenedOnFailure(t *testing.T) {
	sessions := &fakeSessionManager{}
	m := New(nil, fakeBearerStore{}, sessions)
	m.SetHandoverGuardTimeoutForTest(20 * time.Millisecond)
	newRelocationTarget(t, m)

	_, err := m.ForwardRelocation(context.Background(), relocationRequest())
	if !errors.Is(err, ErrRelocationAbandoned) {
		t.Fatalf("error = %v, want ErrRelocationAbandoned", err)
	}

	if !sessions.released {
		t.Error("the anchor session opened for the handover was not released")
	}

	if _, ok := m.LookupUeByIMSI("001010000000001"); ok {
		t.Error("an abandoned preparation left a UE context behind")
	}
}

func TestForwardRelocationReleasesPDNsTheTargetRefused(t *testing.T) {
	sessions := &fakeSessionManager{}
	m := New(nil, fakeBearerStore{}, sessions)
	target := newRelocationTarget(t, m)

	req := relocationRequest(
		interworking.PDNConnection{PDUSessionID: 1, EPSBearerIdentity: 5, APN: "internet"},
		interworking.PDNConnection{PDUSessionID: 2, EPSBearerIdentity: 6, APN: "ims"},
	)

	type result struct {
		resp interworking.ForwardRelocationResponse
		err  error
	}

	done := make(chan result, 1)

	go func() {
		resp, err := m.ForwardRelocation(context.Background(), req)
		done <- result{resp, err}
	}()

	hoReq := target.awaitHandoverRequest(t)
	target.admit(t, hoReq, 6)

	got := <-done
	if got.err != nil {
		t.Fatalf("ForwardRelocation: %v", got.err)
	}

	if len(got.resp.AcceptedPDUSessions) != 1 || got.resp.AcceptedPDUSessions[0] != 1 {
		t.Errorf("accepted PDU sessions = %v, want only the admitted one", got.resp.AcceptedPDUSessions)
	}

	if !sessions.released {
		t.Error("the refused PDN connection's anchor half was not released")
	}

	ue, ok := m.LookupUe(hoReq.MMEUES1APID)
	if !ok {
		t.Fatal("the relocated UE context is gone")
	}

	if m.LookupPDN(ue, 6) != nil {
		t.Error("the refused PDN connection is still on the UE")
	}

	if m.LookupPDN(ue, 5) == nil {
		t.Error("the admitted PDN connection was released too")
	}
}

func TestForwardRelocationFailsWhenNoSessionCanTransfer(t *testing.T) {
	m := newTestMME(t)
	newRelocationTarget(t, m)

	req := relocationRequest(interworking.PDNConnection{PDUSessionID: 1, EPSBearerIdentity: 5, APN: "not-a-subscribed-apn"})

	if _, err := m.ForwardRelocation(context.Background(), req); !errors.Is(err, ErrNoRelocatablePDN) {
		t.Fatalf("error = %v, want ErrNoRelocatablePDN", err)
	}
}

func TestForwardRelocationRefusesASecondHandoverForTheSameSubscriber(t *testing.T) {
	m := newTestMME(t)
	target := newRelocationTarget(t, m)
	req := relocationRequest()

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	hoReq := target.awaitHandoverRequest(t)

	if _, err := m.ForwardRelocation(context.Background(), req); !errors.Is(err, ErrRelocationInProgress) {
		t.Fatalf("error = %v, want ErrRelocationInProgress", err)
	}

	target.admit(t, hoReq)

	if err := <-done; err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}
}

type fakeFiveGSPeer struct {
	completed   string
	completedID interworking.RelocationID
	err         error
}

func (f *fakeFiveGSPeer) RelocationComplete(_ context.Context, supi etsi.SUPI, id interworking.RelocationID) error {
	f.completed = supi.IMSI()
	f.completedID = id

	return f.err
}

func TestCompleteRelocationPublishesTheContextAndNotifiesThePeer(t *testing.T) {
	m := newTestMME(t)
	peer := &fakeFiveGSPeer{}
	m.FiveGS = peer
	target := newRelocationTarget(t, m)
	req := relocationRequest()

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	hoReq := target.awaitHandoverRequest(t)
	target.admit(t, hoReq)

	if err := <-done; err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	ue, ok := m.LookupUe(hoReq.MMEUES1APID)
	if !ok {
		t.Fatal("the relocated UE context is gone")
	}

	if _, ok := m.MarkHandoverCommitting(ue, target.conn, 42); !ok {
		t.Fatal("the Handover Notify did not match the prepared handover")
	}

	if _, _, _, _, ok := m.FinishHandoverCommit(ue, target.conn, 42); !ok {
		t.Fatal("the handover did not commit")
	}

	m.CompleteRelocation(context.Background(), ue)

	if peer.completed != req.SUPI.IMSI() {
		t.Errorf("the 5GS peer was told %q completed, want %q", peer.completed, req.SUPI)
	}

	held, ok := m.LookupUeBySupi(req.SUPI)
	if !ok || held != ue {
		t.Error("the arrived UE is not the MME's context for the subscriber")
	}

	if ue.EMMState() != EMMRegistered {
		t.Errorf("EMM state = %v, want EMM-REGISTERED", ue.EMMState())
	}

	if err := m.RelocationCancel(context.Background(), req.SUPI, req.ID); !errors.Is(err, ErrNoRelocation) {
		t.Errorf("cancelling an arrived UE = %v, want ErrNoRelocation", err)
	}
}

func TestRelocationCancelAbandonsThePreparation(t *testing.T) {
	sessions := &fakeSessionManager{}
	m := New(nil, fakeBearerStore{}, sessions)
	target := newRelocationTarget(t, m)
	req := relocationRequest()

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	target.awaitHandoverRequest(t)

	if err := m.RelocationCancel(context.Background(), req.SUPI, req.ID); err != nil {
		t.Fatalf("RelocationCancel: %v", err)
	}

	if err := <-done; !errors.Is(err, ErrRelocationAbandoned) {
		t.Fatalf("ForwardRelocation = %v, want ErrRelocationAbandoned", err)
	}

	if !sessions.released {
		t.Error("a cancelled handover left its anchor session behind")
	}

	if _, ok := m.LookupUeBySupi(req.SUPI); ok {
		t.Error("a cancelled handover left a UE context behind")
	}
}

// TS 36.413
func TestPreparedRelocationExpiryReleasesEverything(t *testing.T) {
	sessions := &fakeSessionManager{}
	m := New(nil, fakeBearerStore{}, sessions)
	target := newRelocationTarget(t, m)
	req := relocationRequest()

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	hoReq := target.awaitHandoverRequest(t)
	target.admit(t, hoReq)

	if err := <-done; err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	ue, ok := m.LookupUe(hoReq.MMEUES1APID)
	if !ok {
		t.Fatal("the relocated UE context is gone")
	}

	m.abandonHandover(context.Background(), ue, causeHandoverTS1relocExpiry)

	if !sessions.released {
		t.Error("the expired handover left its anchor session behind")
	}

	if err := m.RelocationCancel(context.Background(), req.SUPI, req.ID); !errors.Is(err, ErrNoRelocation) {
		t.Errorf("the expired handover is still registered: %v", err)
	}
}

// A source that abandons one attempt and immediately starts another for the same
// subscriber must not have the late cancel of the first tear down the second.
func TestRelocationCancelOfAnAbandonedAttemptSparesTheNextOne(t *testing.T) {
	sessions := &fakeSessionManager{}
	m := New(nil, fakeBearerStore{}, sessions)
	target := newRelocationTarget(t, m)

	first := relocationRequest()
	first.ID = 7

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), first)
		done <- err
	}()

	target.awaitHandoverRequest(t)

	if err := m.RelocationCancel(context.Background(), first.SUPI, first.ID); err != nil {
		t.Fatalf("RelocationCancel: %v", err)
	}

	if err := <-done; !errors.Is(err, ErrRelocationAbandoned) {
		t.Fatalf("ForwardRelocation = %v, want ErrRelocationAbandoned", err)
	}

	second := relocationRequest()
	second.ID = 8

	go func() {
		_, err := m.ForwardRelocation(context.Background(), second)
		done <- err
	}()

	hoReq := target.awaitNthHandoverRequest(t, 1)

	if err := m.RelocationCancel(context.Background(), second.SUPI, first.ID); !errors.Is(err, ErrNoRelocation) {
		t.Fatalf("cancelling attempt %d hit the MME's attempt %d: %v", first.ID, second.ID, err)
	}

	target.admit(t, hoReq)

	if err := <-done; err != nil {
		t.Fatalf("the second attempt: %v", err)
	}
}

// TS 24.301 §5.5.3.2.5: a UE moved into a tracking area the MME does not serve
// would have its first TAU rejected, so the relocation is refused up front.
func TestForwardRelocationIntoAnUnservedTrackingArea(t *testing.T) {
	m := newTestMME(t)
	newRelocationTarget(t, m)

	req := relocationRequest()
	req.Target.EPSTAC = 9

	if _, err := m.ForwardRelocation(context.Background(), req); !errors.Is(err, ErrUnknownTargetENB) {
		t.Fatalf("error = %v, want ErrUnknownTargetENB", err)
	}

	if _, ok := m.LookupUeBySupi(req.SUPI); ok {
		t.Error("a refused relocation left a UE context behind")
	}
}

func TestRelocationCancelForAnUnknownSubscriber(t *testing.T) {
	m := newTestMME(t)

	if err := m.RelocationCancel(context.Background(), mustSUPI("001010000000009"), 1); !errors.Is(err, ErrNoRelocation) {
		t.Fatalf("error = %v, want ErrNoRelocation", err)
	}
}

func TestTargetGlobalENBIDWidths(t *testing.T) {
	for _, tc := range []struct {
		bits uint8
		kind s1ap.ENBIDKind
		ok   bool
	}{
		{18, s1ap.ENBIDShortMacro, true},
		{20, s1ap.ENBIDMacro, true},
		{21, s1ap.ENBIDLongMacro, true},
		{28, s1ap.ENBIDHome, true},
		{19, 0, false},
	} {
		got, err := targetGlobalENBID(interworking.ENBIdentity{
			PlmnID: models.PlmnID{Mcc: "001", Mnc: "01"},
			ID:     0x123,
			Bits:   tc.bits,
		})
		if (err == nil) != tc.ok {
			t.Fatalf("%d bits: err = %v, want ok = %t", tc.bits, err, tc.ok)
		}

		if tc.ok && got.ENBID.Kind != tc.kind {
			t.Errorf("%d bits: kind = %d, want %d", tc.bits, got.ENBID.Kind, tc.kind)
		}
	}
}

// TS 36.413 §8.4.2.3, TS 38.413 §8.4.1.3
func TestForwardRelocationReportsATargetRejection(t *testing.T) {
	sessions := &fakeSessionManager{}
	m := New(nil, fakeBearerStore{}, sessions)
	target := newRelocationTarget(t, m)
	req := relocationRequest()

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	hoReq := target.awaitHandoverRequest(t)

	ue, ok := m.LookupUe(hoReq.MMEUES1APID)
	if !ok {
		t.Fatal("the relocated UE context is not reachable")
	}

	refusal := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkHOFailureInTarget}
	m.FailHandoverToSource(context.Background(), ue, refusal)

	err := <-done
	if !errors.Is(err, interworking.ErrTargetRefused) {
		t.Fatalf("error = %v, want ErrTargetRefused", err)
	}

	if errors.Is(err, ErrRelocationAbandoned) {
		t.Error("a refusal was reported as an abandonment")
	}

	if !sessions.released {
		t.Error("the refused handover left its anchor session behind")
	}

	if _, ok := m.LookupUeBySupi(req.SUPI); ok {
		t.Error("the refused handover left a UE context behind")
	}
}

func TestRelocatedConnectionIsFullyEstablished(t *testing.T) {
	peer := &fakeFiveGSPeer{}
	m := newTestMME(t)
	m.FiveGS = peer
	target := newRelocationTarget(t, m)
	req := relocationRequest()

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	hoReq := target.awaitHandoverRequest(t)
	target.admit(t, hoReq)

	if err := <-done; err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	ue, ok := m.LookupUe(hoReq.MMEUES1APID)
	if !ok {
		t.Fatal("the relocated UE context is gone")
	}

	if _, ok := m.MarkHandoverCommitting(ue, target.conn, 42); !ok {
		t.Fatal("the Handover Notify did not match the prepared handover")
	}

	if _, _, _, _, ok := m.FinishHandoverCommit(ue, target.conn, 42); !ok {
		t.Fatal("the handover did not commit")
	}

	conn := ue.Conn()
	if conn == nil {
		t.Fatal("the UE holds no connection after the handover")
	}

	if conn.ICS != ICSCompleted {
		t.Errorf("connection ICS state = %v, want the bearers the handover established to count as set up", conn.ICS)
	}
}

// TS 36.413 §8.4.5.1
func TestRelocationCancelAfterTheUEArrivesIsRefused(t *testing.T) {
	sessions := &fakeSessionManager{}
	m := New(nil, fakeBearerStore{}, sessions)
	m.FiveGS = &fakeFiveGSPeer{}
	target := newRelocationTarget(t, m)
	req := relocationRequest()

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	hoReq := target.awaitHandoverRequest(t)
	target.admit(t, hoReq)

	if err := <-done; err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	ue, ok := m.LookupUe(hoReq.MMEUES1APID)
	if !ok {
		t.Fatal("the relocated UE context is gone")
	}

	if _, ok := m.MarkHandoverCommitting(ue, target.conn, 42); !ok {
		t.Fatal("the Handover Notify did not match the prepared handover")
	}

	if err := m.RelocationCancel(context.Background(), req.SUPI, req.ID); !errors.Is(err, ErrRelocationTooLate) {
		t.Fatalf("error = %v, want ErrRelocationTooLate", err)
	}

	if sessions.released {
		t.Error("a late cancel released the anchor sessions of a UE that had already arrived")
	}

	if m.LookupPDN(ue, 5) == nil {
		t.Error("a late cancel dropped the PDN connection of a UE that had already arrived")
	}
}

func TestForwardRelocationTakesTheSliceFromTheSource(t *testing.T) {
	sessions := &fakeSessionManager{}
	m := New(nil, fakeBearerStore{}, sessions)
	target := newRelocationTarget(t, m)

	onAnotherSlice := models.Snssai{Sst: 3, Sd: "0000ff"}

	req := relocationRequest(interworking.PDNConnection{
		PDUSessionID:      1,
		EPSBearerIdentity: 5,
		APN:               "internet",
		Snssai:            onAnotherSlice,
	})

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	hoReq := target.awaitHandoverRequest(t)
	target.admit(t, hoReq)

	if err := <-done; err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	got := sessions.lastRequest.Snssai
	if got == nil || *got != onAnotherSlice {
		t.Fatalf("session created on slice %+v, want the %+v the source named", got, onAnotherSlice)
	}
}
