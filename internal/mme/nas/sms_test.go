// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

type fakeSMSHandler struct {
	mu        sync.Mutex
	allowed   bool
	err       error
	pending   bool
	uplinks   [][]byte
	reachable int
}

func (h *fakeSMSHandler) Allowed(context.Context, string) (bool, error) { return h.allowed, h.err }

func (h *fakeSMSHandler) Uplink(_ context.Context, _ string, payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.uplinks = append(h.uplinks, payload)
}

func (h *fakeSMSHandler) UEReachable(context.Context, string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.reachable++
}

func (h *fakeSMSHandler) AllowedEach(_ context.Context, imsis []string) (map[string]bool, error) {
	allowed := make(map[string]bool, len(imsis))
	for _, imsi := range imsis {
		allowed[imsi] = h.allowed
	}

	return allowed, h.err
}

func (h *fakeSMSHandler) DeliveryFailed(string) {}

func (h *fakeSMSHandler) TransactionPending(string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.pending
}

func (h *fakeSMSHandler) setPending(p bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.pending = p
}

func smsAttachAccept(t *testing.T, m *mme.MME, ue *mme.UeContext) *eps.AttachAccept {
	t.Helper()

	testPDN(ue).PdnType = eps.PDNTypeIPv4
	testPDN(ue).UeIP = testUEIP

	wire, err := buildProtectedAttachAccept(context.Background(), m, ue, models.EPSBearer{QoS: models.EPSBearerQoS{QCI: 9}, MTU: 1400})
	if err != nil {
		t.Fatal(err)
	}

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatal(err)
	}

	accept, err := eps.ParseAttachAccept(plain)
	if err != nil {
		t.Fatal(err)
	}

	return accept
}

func smsOnlyLAI(t *testing.T, m *mme.MME) nas.LAI {
	t.Helper()

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	return *mme.SMSOnlyLAI(plmn)
}

func expectSMSOnly(t *testing.T, m *mme.MME, lai *nas.LAI, result *eps.AdditionalUpdateResult, cause *eps.EMMCause) {
	t.Helper()

	if lai == nil || *lai != smsOnlyLAI(t, m) {
		t.Fatalf("LAI = %v, want %v", lai, smsOnlyLAI(t, m))
	}

	if result == nil || *result != eps.AdditionalUpdateResultSMSOnly {
		t.Fatalf("additional update result = %v, want SMS only", result)
	}

	if cause != nil {
		t.Fatalf("EMM cause = %s, want none", *cause)
	}
}

// TS 24.301 §5.5.1.3.4.2
func TestCombinedAttachIsAcceptedForSMSOnlyWhenSMSIsAllowed(t *testing.T) {
	m := newTestMME(t)
	m.SMS = &fakeSMSHandler{allowed: true}
	ue, _ := securedUE(t, m)
	ue.CombinedAttach = true

	accept := smsAttachAccept(t, m, ue)

	if accept.EPSAttachResult != eps.AttachResultCombined {
		t.Fatalf("EPS attach result = %s, want combined", accept.EPSAttachResult)
	}

	expectSMSOnly(t, m, accept.LAI, accept.AdditionalUpdateResult, accept.Cause)

	if accept.MSIdentity != nil {
		t.Fatalf("MS identity = %v, want none", accept.MSIdentity)
	}

	if !ue.SMSOnly() {
		t.Fatal("the UE context does not record the SMS-only grant")
	}
}

// TS 24.301 §5.5.1.3.4.3
func TestCombinedAttachWithoutSMSIsAcceptedForEPSServicesOnly(t *testing.T) {
	for name, handler := range map[string]mme.SMSHandler{
		"SMS not allowed": &fakeSMSHandler{allowed: false},
		"no SMSF":         nil,
	} {
		t.Run(name, func(t *testing.T) {
			m := newTestMME(t)
			m.SMS = handler
			ue, _ := securedUE(t, m)
			ue.CombinedAttach = true

			accept := smsAttachAccept(t, m, ue)

			if accept.EPSAttachResult != eps.AttachResultEPS || accept.LAI != nil || accept.AdditionalUpdateResult != nil {
				t.Fatalf("attach result = %s, LAI = %v, additional update result = %v, want EPS only", accept.EPSAttachResult, accept.LAI, accept.AdditionalUpdateResult)
			}

			if accept.Cause == nil || *accept.Cause != eps.EMMCauseCSDomainNotAvailable {
				t.Fatalf("EMM cause = %v, want #18", accept.Cause)
			}

			if ue.SMSOnly() {
				t.Fatal("the UE context records an SMS-only grant")
			}
		})
	}
}

func TestEPSAttachIsNotGrantedSMS(t *testing.T) {
	m := newTestMME(t)
	m.SMS = &fakeSMSHandler{allowed: true}
	ue, _ := securedUE(t, m)

	accept := smsAttachAccept(t, m, ue)

	if accept.EPSAttachResult != eps.AttachResultEPS || accept.AdditionalUpdateResult != nil || accept.Cause != nil || ue.SMSOnly() {
		t.Fatalf("attach result = %s, additional update result = %v, cause = %v, SMS only = %t", accept.EPSAttachResult, accept.AdditionalUpdateResult, accept.Cause, ue.SMSOnly())
	}
}

// TS 24.301 §5.5.3.3.4.2, §5.5.3.2.4
func TestTrackingAreaUpdatesMaintainTheSMSOnlyGrant(t *testing.T) {
	m := newTestMME(t)
	m.SMS = &fakeSMSHandler{allowed: true}
	ue, cc := securedUE(t, m)

	handleTAU(t, m, ue, tauRequest(eps.EPSUpdateTypeCombinedTALAIMSI))

	combined := parseTAUAccept(t, ue, cc.sent[0])
	if combined.EPSUpdateResult != eps.EPSUpdateResultCombined {
		t.Fatalf("EPS update result = %d, want combined TA/LA updated", combined.EPSUpdateResult)
	}

	expectSMSOnly(t, m, combined.LAI, combined.AdditionalUpdateResult, combined.Cause)

	handleTrackingAreaUpdateComplete(context.Background(), m, ue, ue.Conn())
	handleTAU(t, m, ue, tauRequest(eps.EPSUpdateTypePeriodic))

	periodic := parseTAUAccept(t, ue, cc.sent[len(cc.sent)-1])
	if periodic.EPSUpdateResult != eps.EPSUpdateResultTA || periodic.LAI != nil || periodic.AdditionalUpdateResult != nil {
		t.Fatalf("periodic update result = %d, LAI = %v, additional update result = %v", periodic.EPSUpdateResult, periodic.LAI, periodic.AdditionalUpdateResult)
	}

	if !ue.SMSOnly() {
		t.Fatal("a periodic update dropped the SMS-only grant")
	}

	handleTrackingAreaUpdateComplete(context.Background(), m, ue, ue.Conn())
	handleTAU(t, m, ue, tauRequest(eps.EPSUpdateTypeTA))

	if ue.SMSOnly() {
		t.Fatal("an EPS-only update kept the SMS-only grant")
	}
}

func TestTrackingAreaUpdateCompleteMakesTheUEReachableForSMS(t *testing.T) {
	m := newTestMME(t)
	handler := &fakeSMSHandler{allowed: true}
	m.SMS = handler
	ue, _ := securedUE(t, m)

	handleTAU(t, m, ue, tauRequest(eps.EPSUpdateTypeCombinedTALAIMSI))
	handleTrackingAreaUpdateComplete(context.Background(), m, ue, ue.Conn())

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if handler.reachable != 1 {
		t.Fatalf("UE reachable reports = %d, want 1", handler.reachable)
	}
}

// TS 24.301 §5.6.3.2
func TestUplinkNASTransportIsHandedToTheSMSF(t *testing.T) {
	m := newTestMME(t)
	handler := &fakeSMSHandler{allowed: true}
	m.SMS = handler
	ue, cc := securedUE(t, m)

	cpData := []byte{0x09, 0x01, 0x02}

	plain, err := (&eps.UplinkNASTransport{NASMessageContainer: cpData}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleEmmMessage(context.Background(), m, ue, ue.Conn(), plain, true)

	m.DecideSMS(context.Background(), ue, true)

	if d := HandleEmmMessage(context.Background(), m, ue, ue.Conn(), plain, true); d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v, want handled", d)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.uplinks) != 1 || !bytes.Equal(handler.uplinks[0], cpData) {
		t.Fatalf("uplinks = %x, want only the one sent after the SMS-only grant", handler.uplinks)
	}

	if cc.count() != 0 {
		t.Fatalf("sent %d downlinks, want none", cc.count())
	}
}

// TS 24.301 §5.5.2.2.3
func TestIMSIDetachRevokesSMSOnly(t *testing.T) {
	m := newTestMME(t)
	m.SMS = &fakeSMSHandler{allowed: true}
	ue, cc := securedUE(t, m)
	testPDN(ue)
	m.DecideSMS(context.Background(), ue, true)

	d := handleDetachRequest(context.Background(), m, ue, ue.Conn(), &eps.DetachRequestUE{TypeOfDetach: eps.DetachTypeIMSI}, true)
	if d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v", d)
	}

	if ue.SMSOnly() {
		t.Fatal("the SMS-only grant survived an IMSI detach")
	}

	if ue.EMMState() != mme.EMMRegistered || ue.Conn() == nil || len(m.SnapshotPDNs(ue)) != 1 {
		t.Fatalf("EMM state = %s, connected = %t, PDNs = %d: an IMSI detach must keep the EPS registration", ue.EMMState(), ue.Conn() != nil, len(m.SnapshotPDNs(ue)))
	}

	if cc.count() != 1 {
		t.Fatalf("sent %d downlinks, want only the DETACH ACCEPT", cc.count())
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatal(err)
	}

	if mt, _ := eps.PeekMessageType(plain); mt != eps.MsgDetachAccept {
		t.Fatalf("downlink %s, want DETACH ACCEPT", mt)
	}
}

func TestIMSIDetachAtSwitchOffIsNotAnswered(t *testing.T) {
	m := newTestMME(t)
	m.SMS = &fakeSMSHandler{allowed: true}
	ue, cc := securedUE(t, m)
	m.DecideSMS(context.Background(), ue, true)

	handleDetachRequest(context.Background(), m, ue, ue.Conn(), &eps.DetachRequestUE{TypeOfDetach: eps.DetachTypeIMSI, SwitchOff: true}, true)

	if ue.SMSOnly() || ue.EMMState() != mme.EMMRegistered || cc.count() != 0 {
		t.Fatalf("SMS only = %t, EMM state = %s, downlinks = %d", ue.SMSOnly(), ue.EMMState(), cc.count())
	}
}

func downlinkPlain(t *testing.T, ue *mme.UeContext, sent []byte) []byte {
	t.Helper()

	dl := decodeDownlinkNAS(t, sent)

	plain, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatal(err)
	}

	return plain
}

// TS 24.301 §5.5.1.3.4.3
func TestCombinedAttachWhoseSMSGrantCannotBeDecidedIsRetriable(t *testing.T) {
	m := newTestMME(t)
	m.SMS = &fakeSMSHandler{err: errors.New("leader changed")}
	ue, _ := securedUE(t, m)
	ue.CombinedAttach = true

	accept := smsAttachAccept(t, m, ue)

	if accept.EPSAttachResult != eps.AttachResultEPS || accept.Cause == nil || *accept.Cause != eps.EMMCauseNetworkFailure {
		t.Fatalf("attach result = %s, cause = %v, want EPS only with #17", accept.EPSAttachResult, accept.Cause)
	}
}

// TS 24.301 §5.5.3.2.4
func TestTAUAnsweringAnSMSPageKeepsTheConnectionForTheSMS(t *testing.T) {
	m := newTestMME(t)
	handler := &fakeSMSHandler{allowed: true}
	m.SMS = handler
	ue, cc := securedUE(t, m)
	m.DecideSMS(context.Background(), ue, true)

	ue.AnswerSignallingPageForTest()
	handleTAU(t, m, ue, tauRequest(eps.EPSUpdateTypePeriodic))

	ue.Conn().TauReleaseOnComplete = true

	handler.setPending(true)

	handleTrackingAreaUpdateComplete(context.Background(), m, ue, ue.Conn())

	if ue.PagingState() != mme.PagingIdle {
		t.Fatalf("paging state = %s, want Idle once the TAU answered the page", ue.PagingState())
	}

	sent := cc.count()

	for _, pdu := range cc.sent {
		if msg, _ := s1ap.Unmarshal(pdu); msg != nil {
			if im, ok := msg.(*s1ap.InitiatingMessage); ok && im.ProcedureCode == s1ap.ProcUEContextRelease {
				t.Fatal("the connection was released with an SMS transaction open")
			}
		}
	}

	handler.setPending(false)
	m.SMSSignallingSettled(context.Background(), ue.IMSI())

	if cc.count() != sent+1 {
		t.Fatalf("sent %d messages after the transaction ended, want the deferred release", cc.count()-sent)
	}

	parseUEContextReleaseCommand(t, cc.sent[sent])
}

func TestWithdrawingSMSDetachesTheUEFromSMSOnly(t *testing.T) {
	m := newTestMME(t)
	handler := &fakeSMSHandler{allowed: true}
	m.SMS = handler
	ue, cc := securedUE(t, m)
	testPDN(ue)
	m.DecideSMS(context.Background(), ue, true)

	handler.mu.Lock()
	handler.err = errors.New("leader changed")
	handler.allowed = false
	handler.mu.Unlock()

	m.ReevaluateSMS(context.Background())

	if !ue.SMSOnly() || cc.count() != 0 {
		t.Fatal("a grant that could not be re-evaluated was withdrawn")
	}

	handler.mu.Lock()
	handler.err = nil
	handler.mu.Unlock()

	m.ReevaluateSMS(context.Background())

	if ue.SMSOnly() || cc.count() != 1 {
		t.Fatalf("SMS only = %t, downlinks = %d, want the grant withdrawn and one DETACH REQUEST", ue.SMSOnly(), cc.count())
	}

	req, err := eps.ParseDetachRequestNetwork(downlinkPlain(t, ue, cc.sent[0]))
	if err != nil || req.TypeOfDetach != eps.DetachTypeNetworkIMSI {
		t.Fatalf("downlink = %+v (%v), want an IMSI detach", req, err)
	}

	handleDetachAccept(context.Background(), m, ue, ue.Conn())

	if ue.EMMState() != mme.EMMRegistered || ue.Conn() == nil || len(m.SnapshotPDNs(ue)) != 1 || cc.count() != 1 {
		t.Fatalf("EMM state = %s, connected = %t, PDNs = %d, downlinks = %d: the IMSI detach must keep the EPS registration", ue.EMMState(), ue.Conn() != nil, len(m.SnapshotPDNs(ue)), cc.count())
	}
}

func TestAnIdleUEIsDetachedFromSMSWhenItReconnects(t *testing.T) {
	m := newTestMME(t)
	handler := &fakeSMSHandler{allowed: true}
	m.SMS = handler
	ue, cc := securedUE(t, m)
	m.DecideSMS(context.Background(), ue, true)
	m.FreeUeConn(context.Background(), ue)

	handler.mu.Lock()
	handler.allowed = false
	handler.mu.Unlock()

	m.ReevaluateSMS(context.Background())

	if ue.SMSOnly() {
		t.Fatal("the grant was not withdrawn")
	}

	c := m.NewUeConn(cc, 9)
	m.AttachUeConn(context.Background(), ue, c)
	c.MarkSecureExchangeEstablished()

	before := cc.count()

	m.SMSReachable(context.Background(), ue)

	if cc.count() != before+1 {
		t.Fatalf("sent %d messages on reconnection, want the IMSI detach", cc.count()-before)
	}
}
