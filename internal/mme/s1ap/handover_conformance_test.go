// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

// Conformance tests for the 4G S1 handover procedures

const (
	ieIDHandoverTargetID  = 4
	ieIDHandoverCause     = 2
	ieIDHandoverEUTRANCGI = 100
	ieIDHandoverTAI       = 67
	ieIDHandoverENBUEID   = 8
)

func secondPDN(ue *mme.UeContext) {
	p := ue.EnsurePDN(6)
	p.Apn = "ims"
	p.Qci, p.Arp = 5, 7
	p.SgwFTEID = models.FTEID{TEID: 0x2222, Addr: netip.AddrFrom4([4]byte{10, 0, 0, 2})}
}

const ackTargetENBUEID s1ap.ENBUES1APID = 55

func ackWith(targetMME s1ap.MMEUES1APID, admitted, failed []uint8) *s1ap.HandoverRequestAcknowledge {
	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID:    s1ap.Ptr(targetMME),
		ENBUES1APID:    s1ap.Ptr(ackTargetENBUEID),
		TargetToSource: s1ap.TransparentContainer{0xaa, 0xbb},
	}

	for _, ebi := range admitted {
		ack.ERABAdmitted = append(ack.ERABAdmitted, s1ap.ERABAdmittedItem{
			ERABID:                s1ap.ERABID(ebi),
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               s1ap.GTPTEID(0x90 + uint32(ebi)),
		})
	}

	for _, ebi := range failed {
		ack.ERABFailedToSetup = append(ack.ERABFailedToSetup, s1ap.ERABItem{
			ERABID: s1ap.ERABID(ebi),
			Cause:  s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkRadioResourcesNotAvailable},
		})
	}

	return ack
}

func handoverCommandOn(t *testing.T, source *captureConn) *s1ap.HandoverCommand {
	t.Helper()

	so, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || so.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("expected a HANDOVER COMMAND to the source, got %T", lastPDU(t, source))
	}

	cmd, err := s1ap.ParseHandoverCommand(so.Value)
	if err != nil {
		t.Fatalf("parse HANDOVER COMMAND: %v", err)
	}

	return cmd
}

func handoverRequestOn(t *testing.T, target *captureConn) *s1ap.HandoverRequest {
	t.Helper()

	im, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcHandoverResourceAllocation {
		t.Fatalf("expected a HANDOVER REQUEST to the target, got %T", lastPDU(t, target))
	}

	req, err := s1ap.ParseHandoverRequest(im.Value)
	if err != nil {
		t.Fatalf("parse HANDOVER REQUEST: %v", err)
	}

	return req
}

func preparationFailureOn(t *testing.T, source *captureConn) *s1ap.HandoverPreparationFailure {
	t.Helper()

	uo, ok := lastPDU(t, source).(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("expected a HANDOVER PREPARATION FAILURE to the source, got %T", lastPDU(t, source))
	}

	fail, err := s1ap.ParseHandoverPreparationFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse HANDOVER PREPARATION FAILURE: %v", err)
	}

	return fail
}

func TestS1HandoverCommandCarriesMandatoryIEs(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	sourceMME, sourceENB := ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID

	driveToPrepared(t, m, ue, source, target)

	cmd := handoverCommandOn(t, source)

	if cmd.MMEUES1APID != sourceMME {
		t.Errorf("HANDOVER COMMAND MME-UE-S1AP-ID = %d, want the source's %d", cmd.MMEUES1APID, sourceMME)
	}

	if cmd.ENBUES1APID != sourceENB {
		t.Errorf("HANDOVER COMMAND eNB-UE-S1AP-ID = %d, want the source's %d", cmd.ENBUES1APID, sourceENB)
	}

	if cmd.HandoverType != s1ap.HandoverTypeIntraLTE {
		t.Errorf("HANDOVER COMMAND Handover Type = %d, want intralte", cmd.HandoverType)
	}

	if !bytes.Equal(cmd.TargetToSource, []byte{0xaa}) {
		t.Errorf("Target to Source Transparent Container = %x, want it relayed unchanged", cmd.TargetToSource)
	}
}

func TestS1HandoverCommandReportsEveryUnadmittedERAB(t *testing.T) {
	for _, tt := range []struct {
		name   string
		failed []uint8
	}{
		{"target reported it failed", []uint8{6}},
		{"target omitted it from both lists", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMME(t)
			ue, source, target := handoverUE(t, m)
			secondPDN(ue)

			handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
				initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

			req := handoverRequestOn(t, target)
			if len(req.ERABToBeSetup) != 2 {
				t.Fatalf("HANDOVER REQUEST asked for %d E-RABs, want both PDN connections", len(req.ERABToBeSetup))
			}

			ack := ackWith(req.MMEUES1APID, []uint8{mme.DefaultERABID}, tt.failed)
			handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target),
				successfulValue(t, mustMarshal(t, ack.Marshal)))

			cmd := handoverCommandOn(t, source)

			var reported bool

			for _, it := range cmd.ERABToRelease {
				if it.ERABID == 6 {
					reported = true
				}
			}

			if !reported {
				t.Errorf("E-RAB 6 was not admitted but is absent from the HANDOVER COMMAND E-RABs to Release List: %+v",
					cmd.ERABToRelease)
			}

			for _, it := range cmd.ERABToRelease {
				if it.ERABID == s1ap.ERABID(mme.DefaultERABID) {
					t.Errorf("the admitted E-RAB %d is also in the to-release list", it.ERABID)
				}
			}

			want := causeHOFailureInTarget
			if tt.failed != nil {
				want = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkRadioResourcesNotAvailable}
			}

			assertReleaseCause(t, cmd, 6, want)
		})
	}
}

func assertReleaseCause(t *testing.T, cmd *s1ap.HandoverCommand, ebi uint8, want s1ap.Cause) {
	t.Helper()

	for _, it := range cmd.ERABToRelease {
		if uint8(it.ERABID) != ebi {
			continue
		}

		if it.Cause != want {
			t.Errorf("release cause for E-RAB %d = %+v, want %+v", ebi, it.Cause, want)
		}

		return
	}

	t.Fatalf("E-RAB %d is not in the to-release list", ebi)
}

func TestS1HandoverCommandReportsABearerTheCoreCouldNotOffer(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	unusable := ue.EnsurePDN(6)
	unusable.Apn = "ims"
	unusable.Qci, unusable.Arp = 5, 7
	unusable.SgwFTEID = models.FTEID{TEID: 0x2222}

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req := handoverRequestOn(t, target)
	if len(req.ERABToBeSetup) != 1 {
		t.Fatalf("HANDOVER REQUEST asked for %d E-RABs, want only the encodable one", len(req.ERABToBeSetup))
	}

	ack := ackWith(req.MMEUES1APID, []uint8{mme.DefaultERABID}, nil)
	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target),
		successfulValue(t, mustMarshal(t, ack.Marshal)))

	cmd := handoverCommandOn(t, source)
	assertReleaseCause(t, cmd, 6, s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUnspecified})
}

func TestS1HandoverPreparationFailureCarriesMandatoryIEs(t *testing.T) {
	m := newTestMME(t)
	ue, source, _ := handoverUE(t, m)

	sourceMME, sourceENB := ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID

	req := sampleHandoverRequired(ue)
	req.TargetID.TargeteNBID.GlobalENBID.ENBID = s1ap.ENBID{Kind: s1ap.ENBIDMacro, Value: 0x99}

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, req.Marshal)))

	fail := preparationFailureOn(t, source)

	if fail.MMEUES1APID == nil || *fail.MMEUES1APID != sourceMME {
		t.Errorf("preparation failure MME-UE-S1AP-ID = %v, want %d", fail.MMEUES1APID, sourceMME)
	}

	if fail.ENBUES1APID == nil || *fail.ENBUES1APID != sourceENB {
		t.Errorf("preparation failure eNB-UE-S1AP-ID = %v, want %d", fail.ENBUES1APID, sourceENB)
	}

	if fail.Cause == nil {
		t.Error("preparation failure carries no Cause (mandatory)")
	}
}

func TestS1HandoverRequiredWithoutTargetIDIsRejected(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	body := dropIEs(t, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)), ieIDHandoverTargetID)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), body)

	if target.count() != 0 {
		t.Fatalf("a HANDOVER REQUIRED missing a mandatory reject-criticality IE was acted on: %d messages to the target",
			target.count())
	}

	if preparationFailureOn(t, source).Cause == nil {
		t.Error("the rejection carries no Cause")
	}
}

func TestS1HandoverRequiredWithoutCauseStillPrepares(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	body := dropIEs(t, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)), ieIDHandoverCause)

	stripped, err := s1ap.ParseHandoverRequired(body)
	if err != nil {
		t.Fatalf("a message missing only an ignore-criticality IE must still parse: %v", err)
	}

	if stripped.Cause != nil {
		t.Fatal("dropIEs did not strip the Cause")
	}

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), body)

	if target.count() != 1 {
		t.Fatalf("the preparation did not reach the target: %d messages", target.count())
	}

	if handoverRequestOn(t, target).Cause == nil {
		t.Error("HANDOVER REQUEST carries no Cause (mandatory) after an absent one was relayed")
	}
}

func TestS1HandoverRequestNCCWrapsToZero(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	ue.SetNCCForTest(7)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if got := handoverRequestOn(t, target).SecurityContext.NextHopChainingCount; got != 0 {
		t.Errorf("HANDOVER REQUEST NCC after 7 = %d, want 0", got)
	}

	if got := ue.NCCForTest(); got != 0 {
		t.Errorf("stored NCC after 7 = %d, want 0", got)
	}
}

func TestS1HandoverRequestCarriesMandatoryIEs(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req := handoverRequestOn(t, target)

	if req.HandoverType != s1ap.HandoverTypeIntraLTE {
		t.Errorf("Handover Type = %d, want intralte", req.HandoverType)
	}

	if req.Cause == nil {
		t.Error("HANDOVER REQUEST carries no Cause (mandatory)")
	}

	if req.UEAMBR.DL == 0 || req.UEAMBR.UL == 0 {
		t.Errorf("UE Aggregate Maximum Bit Rate = %+v, want the UE's stored AMBR", req.UEAMBR)
	}

	if len(req.ERABToBeSetup) == 0 {
		t.Fatal("E-RABs To Be Setup List is empty; its Range is 1")
	}

	setup := req.ERABToBeSetup[0]
	if setup.GTPTEID == 0 {
		t.Error("the E-RAB to set up carries no S-GW uplink TEID")
	}

	if len(setup.TransportLayerAddress) == 0 {
		t.Error("the E-RAB to set up carries no S-GW transport layer address")
	}

	if setup.QoS.QCI == 0 {
		t.Error("the E-RAB to set up carries no E-RAB Level QoS Parameters")
	}

	if len(req.SourceToTarget) == 0 {
		t.Error("HANDOVER REQUEST carries no Source to Target Transparent Container (mandatory)")
	}

	if req.UESecurityCapabilities == (s1ap.UESecurityCapabilities{}) {
		t.Error("HANDOVER REQUEST carries no UE Security Capabilities (mandatory)")
	}
}

func TestS1HandoverFailureWithoutENBUES1APIDFailsPreparation(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	targetMME := handoverRequestOn(t, target).MMEUES1APID
	cause := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkRadioResourcesNotAvailable}

	fail := &s1ap.HandoverFailure{MMEUES1APID: s1ap.Ptr(targetMME), Cause: s1ap.Ptr(cause)}
	handleHandoverFailure(m, context.Background(), mme.NewRadioForTest(target),
		unsuccessfulValue(t, mustMarshal(t, fail.Marshal)))

	got := preparationFailureOn(t, source)
	if got.Cause == nil || *got.Cause != cause {
		t.Errorf("preparation failure cause = %+v, want the target's %+v", got.Cause, cause)
	}

	if ue.HasHandoverForTest() {
		t.Error("the failed handover was left in flight")
	}

	if ue.Conn().Conn() != source {
		t.Error("the UE moved off the source eNB after a failed preparation")
	}
}

func TestS1AcknowledgeAdmittingNoUsableBearerRejectsTheHandover(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req := handoverRequestOn(t, target)

	ack := ackWith(req.MMEUES1APID, []uint8{mme.DefaultERABID}, nil)
	ack.ERABAdmitted[0].TransportLayerAddress = s1ap.TransportLayerAddress{10, 4, 0}

	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target),
		successfulValue(t, mustMarshal(t, ack.Marshal)))

	rel := parseUEContextReleaseCommand(t, lastSent(t, target))
	if rel.UES1APIDs.MMEUES1APID != req.MMEUES1APID {
		t.Errorf("target release names MME-UE-S1AP-ID %d, want the target reservation's %d",
			rel.UES1APIDs.MMEUES1APID, req.MMEUES1APID)
	}

	if preparationFailureOn(t, source).Cause == nil {
		t.Error("the source was not told why the handover was rejected")
	}

	if ue.HasHandoverForTest() {
		t.Error("the rejected handover was left in flight")
	}

	if ue.Conn().Conn() != source {
		t.Error("the UE moved off the source eNB after a rejected handover")
	}
}

func TestS1AcknowledgeWithoutENBUES1APIDIsNotActedOn(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req := handoverRequestOn(t, target)
	sourceBefore := source.count()

	ack := ackWith(req.MMEUES1APID, []uint8{mme.DefaultERABID}, nil)
	body := dropIEs(t, successfulValue(t, mustMarshal(t, ack.Marshal)), ieIDHandoverENBUEID)

	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target), body)

	if source.count() != sourceBefore {
		t.Errorf("an acknowledge with no eNB UE S1AP ID produced %d messages to the source",
			source.count()-sourceBefore)
	}

	if pdn := m.LookupPDN(ue, mme.DefaultERABID); pdn == nil || pdn.EnbFTEID.TEID != 0 {
		t.Errorf("an acknowledge with no eNB UE S1AP ID switched the downlink: %+v", pdn)
	}
}

func TestS1HandoverNotifySwitchesDownlinkAndReleasesTheSource(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	sourceMME, sourceENB := ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	if pdn := m.LookupPDN(ue, mme.DefaultERABID); pdn == nil || pdn.EnbFTEID.TEID == 0x99 {
		t.Fatal("the downlink was switched to the target before HANDOVER NOTIFY")
	}

	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target),
		initiatingValue(t, mustMarshal(t, handoverNotify(targetMME, targetENB).Marshal)))

	pdn := m.LookupPDN(ue, mme.DefaultERABID)
	if pdn == nil || pdn.EnbFTEID.TEID != 0x99 {
		t.Fatalf("the downlink was not switched to the target: %+v", pdn)
	}

	rel := parseUEContextReleaseCommand(t, lastSent(t, source))
	if rel.UES1APIDs.MMEUES1APID != sourceMME || rel.UES1APIDs.ENBUES1APID != sourceENB {
		t.Errorf("source release names (%d, %d), want the source's (%d, %d)",
			rel.UES1APIDs.MMEUES1APID, rel.UES1APIDs.ENBUES1APID, sourceMME, sourceENB)
	}

	if rel.Cause == nil || *rel.Cause != mme.CauseHandoverSuccess {
		t.Errorf("source release cause = %+v, want successful-handover", rel.Cause)
	}
}

func TestS1HandoverNotifyReleasesTheRejectedPDNConnection(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)
	secondPDN(ue)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req := handoverRequestOn(t, target)

	ack := ackWith(req.MMEUES1APID, []uint8{mme.DefaultERABID}, []uint8{6})
	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target),
		successfulValue(t, mustMarshal(t, ack.Marshal)))

	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target),
		initiatingValue(t, mustMarshal(t, handoverNotify(req.MMEUES1APID, ackTargetENBUEID).Marshal)))

	if m.LookupPDN(ue, 6) != nil {
		t.Error("the PDN connection the target rejected was not released")
	}

	if m.LookupPDN(ue, mme.DefaultERABID) == nil {
		t.Error("the admitted PDN connection was released too")
	}

	fsm, ok := m.Session.(*fakeSessionManager)
	if !ok {
		t.Fatalf("session manager is %T", m.Session)
	}

	if !fsm.released {
		t.Error("the rejected PDN's core-network resources were not freed")
	}
}

func TestS1HandoverNotifyToleratesAbsentIgnoreCriticalityIEs(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	body := dropIEs(t, initiatingValue(t, mustMarshal(t, handoverNotify(targetMME, targetENB).Marshal)),
		ieIDHandoverEUTRANCGI, ieIDHandoverTAI)

	stripped, err := s1ap.ParseHandoverNotify(body)
	if err != nil {
		t.Fatalf("a message missing only ignore-criticality IEs must still parse: %v", err)
	}

	if stripped.EUTRANCGI != nil || stripped.TAI != nil {
		t.Fatal("dropIEs did not strip the ignore-criticality IEs")
	}

	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), body)

	if ue.HasHandoverForTest() {
		t.Error("the handover did not complete without E-UTRAN CGI and TAI")
	}

	if ue.Conn().Conn() != target {
		t.Error("the UE was not moved onto the target association")
	}
}

func TestS1HandoverCancelReleasesPreparationAndAcknowledges(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	sourceMME, sourceENB := ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	cause := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkPartialHandover}

	cancel := &s1ap.HandoverCancel{MMEUES1APID: sourceMME, ENBUES1APID: sourceENB, Cause: s1ap.Ptr(cause)}
	handleHandoverCancel(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, cancel.Marshal)))

	rel := parseUEContextReleaseCommand(t, lastSent(t, target))
	if rel.UES1APIDs.MMEUES1APID != targetMME || rel.UES1APIDs.ENBUES1APID != targetENB {
		t.Errorf("target release names (%d, %d), want the prepared target's (%d, %d)",
			rel.UES1APIDs.MMEUES1APID, rel.UES1APIDs.ENBUES1APID, targetMME, targetENB)
	}

	if rel.Cause == nil || *rel.Cause != cause {
		t.Errorf("target release cause = %+v, want the source's cancel cause %+v", rel.Cause, cause)
	}

	so, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || so.ProcedureCode != s1ap.ProcHandoverCancel {
		t.Fatalf("expected a HANDOVER CANCEL ACKNOWLEDGE, got %T", lastPDU(t, source))
	}

	ack, err := s1ap.ParseHandoverCancelAcknowledge(so.Value)
	if err != nil {
		t.Fatalf("parse HANDOVER CANCEL ACKNOWLEDGE: %v", err)
	}

	if ack.MMEUES1APID == nil || *ack.MMEUES1APID != sourceMME {
		t.Errorf("acknowledge MME-UE-S1AP-ID = %v, want %d", ack.MMEUES1APID, sourceMME)
	}

	if ack.ENBUES1APID == nil || *ack.ENBUES1APID != sourceENB {
		t.Errorf("acknowledge eNB-UE-S1AP-ID = %v, want %d", ack.ENBUES1APID, sourceENB)
	}

	if ue.HasHandoverForTest() {
		t.Error("the cancelled handover was left in flight")
	}

	if ue.Conn().Conn() != source {
		t.Error("the UE did not stay on the source eNB")
	}
}

func TestS1HandoverCancelWithoutCauseStillCancels(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	driveToPrepared(t, m, ue, source, target)

	targetBefore := target.count()

	body := dropIEs(t, initiatingValue(t, mustMarshal(t,
		(&s1ap.HandoverCancel{
			MMEUES1APID: ue.Conn().MMEUES1APID,
			ENBUES1APID: ue.Conn().ENBUES1APID,
			Cause:       s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkPartialHandover}),
		}).Marshal)), ieIDHandoverCause)

	handleHandoverCancel(m, context.Background(), mme.NewRadioForTest(source), body)

	if target.count() != targetBefore+1 {
		t.Errorf("a HANDOVER CANCEL without Cause did not release the target: %d new messages",
			target.count()-targetBefore)
	}

	so, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || so.ProcedureCode != s1ap.ProcHandoverCancel {
		t.Fatalf("a HANDOVER CANCEL without Cause was not acknowledged; last message %T", lastPDU(t, source))
	}
}

func TestS1MMEStatusTransferAddressesTheTargetWithTheSameContainer(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	container := s1ap.StatusTransferContainer{0xde, 0xad, 0xbe, 0xef}

	st := &s1ap.ENBStatusTransfer{
		MMEUES1APID: ue.Conn().MMEUES1APID,
		ENBUES1APID: ue.Conn().ENBUES1APID,
		Container:   container,
	}

	sourceBefore := source.count()

	handleENBStatusTransfer(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, st.Marshal)))

	im, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcMMEStatusTransfer {
		t.Fatalf("expected an MME STATUS TRANSFER to the target, got %T", lastPDU(t, target))
	}

	relayed, err := s1ap.ParseMMEStatusTransfer(im.Value)
	if err != nil {
		t.Fatalf("parse MME STATUS TRANSFER: %v", err)
	}

	if relayed.MMEUES1APID != targetMME || relayed.ENBUES1APID != targetENB {
		t.Errorf("MME STATUS TRANSFER names (%d, %d), want the target's (%d, %d)",
			relayed.MMEUES1APID, relayed.ENBUES1APID, targetMME, targetENB)
	}

	if !bytes.Equal(relayed.Container, container) {
		t.Errorf("container = %x, want it relayed unchanged (%x)", relayed.Container, container)
	}

	if source.count() != sourceBefore {
		t.Error("the status container was echoed back to the source")
	}
}

func TestS1HandoverCompletesWithoutAStatusTransfer(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target),
		initiatingValue(t, mustMarshal(t, handoverNotify(targetMME, targetENB).Marshal)))

	if ue.HasHandoverForTest() {
		t.Error("the handover did not complete without an eNB Status Transfer")
	}
}

func TestS1StatusTransferWithNoHandoverIsNotRelayed(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	st := &s1ap.ENBStatusTransfer{
		MMEUES1APID: ue.Conn().MMEUES1APID,
		ENBUES1APID: ue.Conn().ENBUES1APID,
		Container:   s1ap.StatusTransferContainer{0x01},
	}

	handleENBStatusTransfer(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, st.Marshal)))

	if target.count() != 0 {
		t.Errorf("a status transfer with no handover in progress was relayed: %d messages", target.count())
	}
}

func lastSent(t *testing.T, cc *captureConn) []byte {
	t.Helper()

	cc.mu.Lock()
	defer cc.mu.Unlock()

	if len(cc.sent) == 0 {
		t.Fatal("no S1AP message captured")
	}

	return append([]byte(nil), cc.sent[len(cc.sent)-1]...)
}

func TestS1HandoverCommandRelaysTheTargetsReleaseCause(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)
	secondPDN(ue)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req := handoverRequestOn(t, target)

	targetCause := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkRadioResourcesNotAvailable}

	ack := ackWith(req.MMEUES1APID, []uint8{mme.DefaultERABID}, nil)
	ack.ERABFailedToSetup = []s1ap.ERABItem{{ERABID: 6, Cause: targetCause}}

	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target),
		successfulValue(t, mustMarshal(t, ack.Marshal)))

	cmd := handoverCommandOn(t, source)
	if len(cmd.ERABToRelease) != 1 {
		t.Fatalf("E-RABs to Release List = %+v, want the refused E-RAB", cmd.ERABToRelease)
	}

	if cmd.ERABToRelease[0].Cause != targetCause {
		t.Errorf("release cause = %+v, want the target's %+v", cmd.ERABToRelease[0].Cause, targetCause)
	}
}
