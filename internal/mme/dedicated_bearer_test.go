// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

const voiceSGWTEID = 0x2010

func voiceBearerRequest() models.DedicatedBearerRequest {
	rate := models.BitRateFromBps(41000)

	return models.DedicatedBearerRequest{
		SessionRef: "ref-internet",
		LinkedEBI:  DefaultERABID,
		QCI:        1,
		ARP:        models.Arp{PriorityLevel: 2, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable},
		MBR:        models.Ambr{Uplink: rate, Downlink: rate},
		GBR:        models.Ambr{Uplink: rate, Downlink: rate},
		Filters: []models.SDFFilter{
			{ID: 1, Direction: models.FilterDownlink, Precedence: 32, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50000, RemotePort: 49000},
			{ID: 2, Direction: models.FilterUplink, Precedence: 33, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50000, RemotePort: 49000},
		},
		SGW: models.FTEID{TEID: voiceSGWTEID, Addr: netip.MustParseAddr("10.3.0.1")},
	}
}

func initiating(t *testing.T, pdu []byte, code s1ap.ProcedureCode) []byte {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal S1AP: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != code {
		t.Fatalf("got %T, want procedure %d", msg, code)
	}

	return im.Value
}

func downlinkPlain(t *testing.T, ue *UeContext, wire []byte) []byte {
	t.Helper()

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	return plain
}

func activateVoiceBearer(t *testing.T, m *MME, ue *UeContext, cc *captureConn) *s1ap.ERABSetupRequest {
	t.Helper()

	if err := m.ActivateDedicatedBearer(context.Background(), ue.imsiOrEmpty(), voiceBearerRequest()); err != nil {
		t.Fatalf("ActivateDedicatedBearer: %v", err)
	}

	sent := cc.snapshot()

	req, err := s1ap.ParseERABSetupRequest(initiating(t, sent[len(sent)-1], s1ap.ProcERABSetup))
	if err != nil {
		t.Fatalf("parse E-RAB Setup Request: %v", err)
	}

	return req
}

func TestDedicatedBearerActivation(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	req := activateVoiceBearer(t, m, ue, cc)

	if len(req.ERABToBeSetup) != 1 {
		t.Fatalf("E-RABs %+v, want one", req.ERABToBeSetup)
	}

	item := req.ERABToBeSetup[0]
	if item.ERABID != 6 || item.QoS.QCI != 1 || item.QoS.GBR == nil || item.QoS.GBR.GuaranteedBitrateUL != 41000 || uint32(item.GTPTEID) != voiceSGWTEID {
		t.Fatalf("E-RAB %+v, want EBI 6 at QCI 1 with a 41 kbps GBR on TEID %#x", item, voiceSGWTEID)
	}

	if item.QoS.ARP.PriorityLevel != 2 || item.QoS.ARP.PreemptionCapability != s1ap.PreemptionMayTrigger || item.QoS.ARP.PreemptionVulnerability != s1ap.PreemptionNotPreemptable {
		t.Fatalf("ARP %+v, want priority 2, may pre-empt, not pre-emptable", item.QoS.ARP)
	}

	act, err := eps.ParseActivateDedicatedEPSBearerContextRequest(downlinkPlain(t, ue, []byte(item.NASPDU)))
	if err != nil {
		t.Fatalf("parse Activate Dedicated EPS Bearer Context Request: %v", err)
	}

	if act.EPSBearerIdentity != 6 || uint8(act.LinkedEPSBearerIdentity) != DefaultERABID || act.EPSQoS.QCI != 1 {
		t.Fatalf("activation %+v, want EBI 6 linked to %d at QCI 1", act, DefaultERABID)
	}

	if rates, ok := act.EPSQoS.GBRBitRates(); !ok || rates.GuaranteedDownlinkKbps != 41 {
		t.Fatalf("EPS QoS bit rates %+v, want a 41 kbps GBR", rates)
	}

	if act.TFT.Operation != eps.TFTCreate || len(act.TFT.Filters) != 2 || act.TFT.Filters[1].Direction != eps.TFTUplink {
		t.Fatalf("TFT %+v, want a new TFT with a downlink and an uplink filter", act.TFT)
	}

	if !m.DedicatedBearerAccepted(context.Background(), ue, 6) {
		t.Fatal("the accept was not matched to the bearer")
	}

	if activated, _ := fake.dedicatedOutcomes(); len(activated) != 0 {
		t.Fatalf("activation reported before the eNB answered: %+v", activated)
	}

	enb := models.FTEID{TEID: 0x77, Addr: netip.MustParseAddr("10.3.0.3")}
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6, EnbFTEID: enb}}})

	activated, dropped := fake.dedicatedOutcomes()
	want := dedicatedOutcome{ref: "ref-internet", teid: voiceSGWTEID, ebi: 6, enb: enb}

	if len(activated) != 1 || activated[0] != want || len(dropped) != 0 {
		t.Fatalf("SMF told activated %+v dropped %+v, want %+v", activated, dropped, want)
	}

	if ebi := m.AddPDN(ue); ebi == nil || ebi.Ebi != 7 {
		t.Fatalf("next PDN connection got %+v, want EBI 7 past the dedicated bearer", ebi)
	}
}

func TestDedicatedBearerRejectedByTheUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	m.FailDedicatedBearer(context.Background(), ue, 6, "rejected")

	sent := cc.snapshot()

	cmd, err := s1ap.ParseERABReleaseCommand(initiating(t, sent[len(sent)-1], s1ap.ProcERABRelease))
	if err != nil || len(cmd.ERABToBeReleased) != 1 || cmd.ERABToBeReleased[0].ERABID != 6 {
		t.Fatalf("release %+v (%v), want E-RAB 6 released", cmd, err)
	}

	if _, dropped := fake.dedicatedOutcomes(); len(dropped) != 1 || dropped[0].teid != voiceSGWTEID {
		t.Fatalf("SMF told dropped %+v, want the voice bearer", dropped)
	}

	if _, b := m.LookupDedicated(ue, 6); b != nil {
		t.Fatal("the failed bearer is still held")
	}
}

func TestDedicatedBearerDeactivation(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	if err := m.DeactivateDedicatedBearer(context.Background(), ue.imsiOrEmpty(), 6, voiceSGWTEID); err != nil {
		t.Fatalf("DeactivateDedicatedBearer: %v", err)
	}

	sent := cc.snapshot()

	cmd, err := s1ap.ParseERABReleaseCommand(initiating(t, sent[len(sent)-1], s1ap.ProcERABRelease))
	if err != nil {
		t.Fatalf("parse E-RAB Release Command: %v", err)
	}

	deact, err := eps.ParseDeactivateEPSBearerContextRequest(downlinkPlain(t, ue, []byte(cmd.NASPDU)))
	if err != nil || deact.EPSBearerIdentity != 6 || deact.Cause != eps.ESMCauseRegularDeactivation {
		t.Fatalf("deactivation %+v (%v), want EBI 6 with cause #36", deact, err)
	}

	if _, dropped := fake.dedicatedOutcomes(); len(dropped) != 0 {
		t.Fatal("release reported before the UE accepted")
	}

	if !m.ReleaseDedicated(context.Background(), ue, 6) {
		t.Fatal("the accept was not matched to the bearer")
	}

	if _, dropped := fake.dedicatedOutcomes(); len(dropped) != 1 {
		t.Fatalf("SMF told dropped %+v, want the voice bearer", dropped)
	}
}

func TestDedicatedBearerOfAnIdleUE(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)
	testPDN(ue).SessionRef = "ref-internet"

	if err := m.ActivateDedicatedBearer(context.Background(), ue.imsiOrEmpty(), voiceBearerRequest()); !errors.Is(err, ErrUENotReachable) {
		t.Fatalf("ActivateDedicatedBearer error = %v, want ErrUENotReachable", err)
	}

	if !m.pagingActive(ue) {
		t.Fatal("the idle UE was not paged for the dedicated bearer")
	}
}

func TestIdleUEDedicatedBearerIsReleasedLocally(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	ue.Conn().releaseCause.Store(&s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUserInactivity})
	m.FreeUeConn(context.Background(), ue)

	if _, b := m.LookupDedicated(ue, 6); b == nil {
		t.Fatal("an active dedicated bearer did not survive a user-inactivity release")
	}

	if err := m.DeactivateDedicatedBearer(context.Background(), ue.imsiOrEmpty(), 6, voiceSGWTEID); err != nil {
		t.Fatalf("DeactivateDedicatedBearer: %v", err)
	}

	if _, dropped := fake.dedicatedOutcomes(); len(dropped) != 1 {
		t.Fatalf("SMF told dropped %+v, want the local release", dropped)
	}

	if !ue.LocalBearerDeactivationPending() {
		t.Fatal("the local release is not flagged for the next TAU")
	}
}

func TestConnectionLossFailsAnActivation(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.FreeUeConn(context.Background(), ue)

	deadline := time.Now().Add(5 * time.Second)

	for {
		if _, dropped := fake.dedicatedOutcomes(); len(dropped) == 1 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("the SMF was not told the activation failed")
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestPDNReleaseTakesItsDedicatedBearers(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	second := m.AddPDN(ue)
	second.SessionRef = "ref-other"

	m.sendERABRelease(context.Background(), ue, ue.Conn(), testPDN(ue), nil)

	sent := cc.snapshot()

	cmd, err := s1ap.ParseERABReleaseCommand(initiating(t, sent[len(sent)-1], s1ap.ProcERABRelease))
	if err != nil {
		t.Fatalf("parse E-RAB Release Command: %v", err)
	}

	if len(cmd.ERABToBeReleased) != 2 || cmd.ERABToBeReleased[1].ERABID != 6 {
		t.Fatalf("released E-RABs %+v, want the default and the dedicated bearer", cmd.ERABToBeReleased)
	}

	_, b := m.LookupDedicated(ue, 6)
	m.ReleasePDN(context.Background(), ue, testPDN(ue))

	if b.guard.Active() {
		t.Fatal("the dedicated bearer's guard outlived its PDN connection")
	}

	if _, held := m.LookupDedicated(ue, 6); held != nil {
		t.Fatal("the dedicated bearer outlived its PDN connection")
	}
}

func TestLateERABSetupResponseIsReleased(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	activateVoiceBearer(t, m, ue, cc)
	m.FailDedicatedBearer(context.Background(), ue, 6, "rejected")

	before := len(cc.snapshot())

	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}, ReleaseUnknown: true})

	sent := cc.snapshot()
	if len(sent) != before+1 {
		t.Fatalf("sent %d messages for the late E-RAB, want one release", len(sent)-before)
	}

	cmd, err := s1ap.ParseERABReleaseCommand(initiating(t, sent[len(sent)-1], s1ap.ProcERABRelease))
	if err != nil || cmd.ERABToBeReleased[0].ERABID != 6 {
		t.Fatalf("release %+v (%v), want E-RAB 6", cmd, err)
	}
}

func TestAcceptedActivationStillWaitsForTheRadio(t *testing.T) {
	m := newTestMME(t)
	m.esmGuardCfg.ExpireTime = 20 * time.Millisecond
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)

	deadline := time.Now().Add(5 * time.Second)

	for {
		if _, dropped := fake.dedicatedOutcomes(); len(dropped) == 1 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("an activation with no E-RAB Setup Response never failed")
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestActiveDedicatedBearerFailsLocally(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	before := len(cc.snapshot())

	m.FailDedicatedBearer(context.Background(), ue, 6, "inactive at the UE")

	if n := len(cc.snapshot()) - before; n != 0 {
		t.Fatalf("sent %d messages for an active bearer the UE dropped, want a local deactivation", n)
	}

	if _, dropped := fake.dedicatedOutcomes(); len(dropped) != 1 || !ue.LocalBearerDeactivationPending() {
		t.Fatalf("dropped %+v, local flag %v, want a reported local deactivation", dropped, ue.LocalBearerDeactivationPending())
	}
}

func TestDeactivationAfterTheUEAcceptedSignalsTheUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)

	m.DeactivateDedicated(context.Background(), ue, 6)

	sent := cc.snapshot()

	cmd, err := s1ap.ParseERABReleaseCommand(initiating(t, sent[len(sent)-1], s1ap.ProcERABRelease))
	if err != nil || len(cmd.NASPDU) == 0 {
		t.Fatalf("release %+v (%v), want one carrying the NAS deactivation", cmd, err)
	}

	if !m.ReleaseDedicated(context.Background(), ue, 6) {
		t.Fatal("the deactivation was not pending the UE's accept")
	}
}

func TestESMStatusInvalidBearerReleasesTheDedicatedBearer(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	if !m.DedicatedESMStatus(context.Background(), ue, 6, eps.ESMCauseInvalidEPSBearerIdentity) {
		t.Fatal("the ESM STATUS was not matched to the dedicated bearer")
	}

	if _, b := m.LookupDedicated(ue, 6); b != nil {
		t.Fatal("the bearer survived ESM STATUS #43")
	}

	if _, dropped := fake.dedicatedOutcomes(); len(dropped) != 1 {
		t.Fatalf("SMF told dropped %+v, want the bearer", dropped)
	}
}

func TestRefusedRadioEndpointReleasesTheBearer(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	fake.outcomeMu.Lock()
	fake.moveErr = errors.New("UPF refused the rules")
	fake.outcomeMu.Unlock()

	before := len(cc.snapshot())

	result := m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6, EnbFTEID: models.FTEID{TEID: 0x78, Addr: netip.MustParseAddr("10.3.0.4")}}}})
	if len(result.Applied) != 0 || len(result.Failed) != 1 || result.Failed[0] != 6 {
		t.Fatalf("result %+v, want E-RAB 6 failed", result)
	}

	if sent := cc.snapshot(); len(sent) != before {
		t.Fatalf("sent %d messages before the procedure concluded, want none", len(sent)-before)
	}

	m.DeactivatePendingDedicated(context.Background(), ue)

	sent := cc.snapshot()

	cmd, err := s1ap.ParseERABReleaseCommand(initiating(t, sent[len(sent)-1], s1ap.ProcERABRelease))
	if err != nil {
		t.Fatalf("parse E-RAB Release Command: %v", err)
	}

	deact, err := eps.ParseDeactivateEPSBearerContextRequest(downlinkPlain(t, ue, []byte(cmd.NASPDU)))
	if err != nil || deact.EPSBearerIdentity != 6 {
		t.Fatalf("deactivation %+v (%v), want EBI 6", deact, err)
	}
}

func TestBearerRefusedOnPathSwitchIsDeactivatedOverNAS(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	fake.outcomeMu.Lock()
	fake.moveErr = errors.New("UPF refused the rules")
	fake.outcomeMu.Unlock()

	result := m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{
		Present:       []RANBearer{{Ebi: 5}, {Ebi: 6, EnbFTEID: models.FTEID{TEID: 0x78, Addr: netip.MustParseAddr("10.3.0.4")}}},
		Authoritative: true,
		ReleaseFailed: true,
	})
	if len(result.Failed) != 1 || result.Failed[0] != 6 {
		t.Fatalf("result %+v, want E-RAB 6 in the To Be Released list", result)
	}

	m.DeactivatePendingDedicated(context.Background(), ue)

	sent := cc.snapshot()

	deact, err := eps.ParseDeactivateEPSBearerContextRequest(downlinkPlain(t, ue, decodeDownlinkNAS(t, sent[len(sent)-1])))
	if err != nil || deact.EPSBearerIdentity != 6 {
		t.Fatalf("deactivation %+v (%v), want EBI 6 in Downlink NAS Transport", deact, err)
	}
}

func TestS1ReleaseKeepsGBRBearersOnlyForPreservingCauses(t *testing.T) {
	tests := []struct {
		name  string
		cause *s1ap.Cause
		kept  bool
	}{
		{"user inactivity", &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUserInactivity}, true},
		{"inter-RAT redirection", &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkInterRATRedirection}, true},
		{"CS fallback triggered", &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkCSFallbackTriggered}, true},
		{"radio connection lost", &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkRadioConnectionWithUELost}, false},
		{"UE not available for PS service", &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUENotAvailableForPSService}, false},
		{"MME normal release", &s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASNormalRelease}, false},
		{"local release, no cause", nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			ue, cc := connectedBearerUE(t, m)
			fake := m.Session.(*fakeSessionManager)

			activateVoiceBearer(t, m, ue, cc)
			m.DedicatedBearerAccepted(context.Background(), ue, 6)
			m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

			ue.Conn().releaseCause.Store(tc.cause)
			m.FreeUeConn(context.Background(), ue)

			_, b := m.LookupDedicated(ue, 6)

			var dropped []dedicatedOutcome

			for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
				if _, dropped = fake.dedicatedOutcomes(); len(dropped) > 0 || tc.kept {
					break
				}
			}

			if tc.kept {
				if b == nil || len(dropped) != 0 {
					t.Fatalf("GBR bearer kept = %v, SMF told dropped %+v; want it preserved", b != nil, dropped)
				}

				return
			}

			if b != nil || len(dropped) != 1 || !ue.LocalBearerDeactivationPending() {
				t.Fatalf("GBR bearer kept = %v, SMF told dropped %+v, TAU sync pending = %v; want it deactivated locally",
					b != nil, dropped, ue.LocalBearerDeactivationPending())
			}
		})
	}
}

func TestENBReleasedDedicatedBearerIsReleasedWithoutSignalling(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	sent := len(cc.snapshot())

	if !m.DedicatedReleasedByRAN(context.Background(), ue, 6) {
		t.Fatal("the released E-RAB was not matched to the dedicated bearer")
	}

	if _, b := m.LookupDedicated(ue, 6); b != nil {
		t.Fatal("the dedicated bearer outlived its E-RAB")
	}

	if n := len(cc.snapshot()); n != sent {
		t.Fatalf("%d S1AP messages sent for an eNB-initiated release, want none (TS 23.401 §5.4.4.2 step 7)", n-sent)
	}

	if _, dropped := fake.dedicatedOutcomes(); len(dropped) != 1 {
		t.Fatalf("SMF told dropped %+v, want the voice bearer", dropped)
	}

	if m.DedicatedReleasedByRAN(context.Background(), ue, 6) {
		t.Fatal("a second indication for the same E-RAB matched a bearer")
	}
}

func TestHandoverOffersTheVoiceBearerWithItsGBR(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	testPDN(ue).SgwFTEID = models.FTEID{TEID: 0x10, Addr: netip.MustParseAddr("10.3.0.1")}

	bearers, candidates, ok := HandoverBearers(ue, true)
	if !ok || len(bearers) != 2 || len(candidates) != 2 {
		t.Fatalf("handover E-RABs %+v, candidates %+v; want the default and the voice bearer", bearers, candidates)
	}

	voice := bearers[1]
	if voice.ERABID != 6 || voice.QoS.QCI != 1 || voice.QoS.GBR == nil || voice.QoS.GBR.GuaranteedBitrateDL == 0 || uint32(voice.GTPTEID) != voiceSGWTEID {
		t.Fatalf("voice E-RAB %+v, want EBI 6 at QCI 1 with GBR QoS Information (TS 36.413 §8.4.2.4)", voice)
	}
}

func TestDedicatedBearerCarriesTheMappedQoSFlow(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	mapped := []nas.PCOContainer{
		{ID: nas.PCOContainerQoSRules, Content: []byte{2, 0, 3, 0x21, 1, 0x02}},
		{ID: nas.PCOContainerQoSFlowDescriptions, Content: []byte{2, 0x20, 0x41, 1, 1, 1}},
	}

	req := voiceBearerRequest()
	req.MappedFiveGSQoS = mapped

	if err := m.ActivateDedicatedBearer(context.Background(), ue.imsiOrEmpty(), req); err != nil {
		t.Fatalf("ActivateDedicatedBearer: %v", err)
	}

	sent := cc.snapshot()

	setup, err := s1ap.ParseERABSetupRequest(initiating(t, sent[len(sent)-1], s1ap.ProcERABSetup))
	if err != nil {
		t.Fatalf("parse E-RAB Setup Request: %v", err)
	}

	act, err := eps.ParseActivateDedicatedEPSBearerContextRequest(downlinkPlain(t, ue, []byte(setup.ERABToBeSetup[0].NASPDU)))
	if err != nil {
		t.Fatalf("parse Activate Dedicated EPS Bearer Context Request: %v", err)
	}

	pco := act.ProtocolConfigurationOptions
	if pco == nil {
		pco = act.ExtendedProtocolConfigurationOptions
	}

	if pco == nil || !slices.Equal(pco.ContainerIDs(), []uint16{nas.PCOContainerQoSRules, nas.PCOContainerQoSFlowDescriptions}) {
		t.Fatalf("activation PCO %+v, want the QoS rules and QoS flow descriptions (TS 24.501 §6.1.4.1)", pco)
	}
}
