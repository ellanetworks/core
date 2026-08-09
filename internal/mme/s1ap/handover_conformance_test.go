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

// Conformance tests for the 4G S1 handover procedures: Handover Preparation
// (TS 36.413 §8.4.1), Handover Resource Allocation (§8.4.2), Handover
// Notification (§8.4.3), Handover Cancellation (§8.4.5) and the eNB/MME Status
// Transfer pair (§8.4.6/§8.4.7), together with the stage-2 call flow in TS 23.401
// §5.5.1.2 and the AS key handling in TS 33.401 §7.2.8.4.3.
//
// The requirements are extracted from the E-UTRAN specifications alone; nothing
// here is carried over from the NGAP suite by analogy. Where a clause exists on
// only one side the difference is called out in the test comment.

// Protocol IE identifiers these tests strip from an encoded body to exercise
// §10.3.5 absent-IE handling (TS 36.413 §9.2.x).
const (
	ieIDHandoverTargetID  = 4 // Target ID
	ieIDHandoverCause     = 2 // Cause
	ieIDHandoverEUTRANCGI = 100
	ieIDHandoverTAI       = 67
	ieIDHandoverENBUEID   = 8 // eNB UE S1AP ID
)

// secondPDN adds a second PDN connection on EBI 6 so partial-admission
// requirements have something to reject.
func secondPDN(ue *mme.UeContext) {
	p := ue.EnsurePDN(6)
	p.Apn = "ims"
	p.Qci, p.Arp = 5, 7
	p.SgwFTEID = models.FTEID{TEID: 0x2222, Addr: netip.AddrFrom4([4]byte{10, 0, 0, 2})}
}

// ackTargetENBUEID is the eNB-UE-S1AP-ID the target eNB allocates and reports in
// its HANDOVER REQUEST ACKNOWLEDGE.
const ackTargetENBUEID s1ap.ENBUES1APID = 55

// ackWith builds a HANDOVER REQUEST ACKNOWLEDGE admitting the given EBIs and
// reporting the ones in failed. EBIs named in neither list are simply omitted,
// which is what §8.4.1.2's "any E-RABs that could not be admitted" has to cover.
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
			Cause:  s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
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

// ---------------------------------------------------------------------------
// Batch S1P — Handover Preparation
// ---------------------------------------------------------------------------

// S1P-2/S1P-3/S1P-5. TS 36.413 §8.4.1.2: "When the preparation, including the
// reservation of resources at the target side is ready, the MME responds with the
// HANDOVER COMMAND message to the source eNB", and "If the Target to Source
// Transparent Container IE has been received by the MME from the handover target
// then the transparent container shall be included in the HANDOVER COMMAND
// message." §9.1.5.2 makes MME UE S1AP ID, eNB UE S1AP ID, Handover Type and that
// container mandatory — S1AP has no mandatory admitted-bearer list, unlike the
// NGAP PDU Session Resource Handover List.
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

// S1P-4. TS 36.413 §8.4.1.2: "If there are any E-RABs that could not be admitted
// in the target, they shall be indicated in the E-RABs to Release List IE."
// §8.4.2.2 makes the target's two lists exhaustive ("The E-RABs that have not been
// admitted in the target cell, if any, shall be included in the E-RABs Failed to
// Setup List IE"), but the MME's own obligation covers every requested E-RAB that
// did not come back admitted, including one the target left out of both lists.
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

			// An E-RAB cannot both be handed over and be released by the same
			// command; TS 36.413 §9.1.5.5 keeps the target's own two lists
			// disjoint for the same reason.
			for _, it := range cmd.ERABToRelease {
				if it.ERABID == s1ap.ERABID(mme.DefaultERABID) {
					t.Errorf("the admitted E-RAB %d is also in the to-release list", it.ERABID)
				}
			}
		})
	}
}

// S1P-6. TS 36.413 §8.4.1.3: "If the EPC or the target system is not able to
// accept any of the bearers or a failure occurs during the Handover Preparation,
// the MME sends the HANDOVER PREPARATION FAILURE message with an appropriate cause
// value to the source eNB." §9.1.5.3 makes MME UE S1AP ID, eNB UE S1AP ID and
// Cause mandatory in it.
func TestS1HandoverPreparationFailureCarriesMandatoryIEs(t *testing.T) {
	m := newTestMME(t)
	ue, source, _ := handoverUE(t, m)

	sourceMME, sourceENB := ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID

	// An unknown target eNB fails the preparation before any target is involved.
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

// S1P-7. TS 36.413 §9.1.5.1 makes Target ID mandatory with reject criticality in
// the HANDOVER REQUIRED. Per §10.3.5 the message is then not acted on, and
// §10.3.4.2 answers with the procedure's own unsuccessful outcome rather than an
// Error Indication.
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

// S1P-8. TS 36.413 §9.1.5.1 makes Cause mandatory with ignore criticality. Per
// §10.3.5 an absent ignore-criticality IE does not stop delivery, and the Cause
// the MME relays onward is mandatory in the HANDOVER REQUEST (§9.1.5.4), so an
// absent one has to become a real cause value.
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

// S1P-9. TS 33.401 §7.2.8.4.3: "Upon reception of the HANDOVER REQUIRED message
// the source MME shall increase its locally kept NCC value by one and compute a
// fresh NH from its stored data ... store that fresh pair" and send it to the
// target eNB in the S1 HANDOVER REQUEST. NCC is a three-bit counter (§7.2.8.1.1),
// so the increment wraps from 7 to 0.
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

// ---------------------------------------------------------------------------
// Batch S1R — Handover Resource Allocation
// ---------------------------------------------------------------------------

// S1R-2/S1R-3. TS 36.413 §9.1.5.4: the HANDOVER REQUEST carries MME UE S1AP ID,
// Handover Type, Cause, UE Aggregate Maximum Bit Rate, an E-RABs To Be Setup List
// of Range 1, Source to Target Transparent Container, UE Security Capabilities and
// Security Context, all mandatory. GUMMEI is optional here and there is no
// Allowed NSSAI at all — the two IEs NGAP §9.2.3.4 makes mandatory.
// TS 23.401 §5.5.1.2.2 step 5: "For each EPS Bearer, the Bearers to Setup includes
// Serving GW address and uplink TEID for user plane, and EPS Bearer QoS."
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

// S1R-4. TS 36.413 §9.1.5.6: the HANDOVER FAILURE carries MME UE S1AP ID and
// Cause and has **no eNB UE S1AP ID** — the target admitted nothing, so it holds no
// context. The MME must resolve the preparation on the MME-UE-S1AP-ID alone and
// then fail it toward the source (§8.4.1.3).
func TestS1HandoverFailureWithoutENBUES1APIDFailsPreparation(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	targetMME := handoverRequestOn(t, target).MMEUES1APID
	cause := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 27} // no-radio-resources-available-in-target-cell

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

// S1R-5. TS 23.401 §5.5.1.2.3: "the Target MME rejects the handover request and
// clears all resource in Target eNodeB and Target MME if the Target eNodeB accepts
// the handover request but none of the default EPS bearers gets resources
// allocated", and step 9 sends a Handover Preparation Failure to the source eNodeB.
//
// TS 36.413 §9.1.5.5 gives the E-RABs Admitted List Range 1, so the acknowledge is
// never literally empty; the case is an acknowledge from which no usable S1-U
// endpoint can be taken. Every E-RAB in Ella Core is a PDN connection's default
// bearer, so that leaves no default bearer allocated.
func TestS1AcknowledgeAdmittingNoUsableBearerRejectsTheHandover(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req := handoverRequestOn(t, target)

	// A three-octet transport layer address is neither IPv4 nor IPv6, so the MME
	// can derive no downlink endpoint for the E-RAB the target claims to admit.
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

// S1R-9. TS 36.413 §9.1.5.5 makes both UE S1AP IDs mandatory with **ignore**
// criticality in the HANDOVER REQUEST ACKNOWLEDGE. Per §10.3.5 an absent
// ignore-criticality IE is ignored and the procedure continues where it can — but
// the eNB UE S1AP ID is what the MME binds to the target association, so the
// procedure cannot continue. It must not be acted on; §10.6 lets the MME say so
// with an Error Indication.
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

// ---------------------------------------------------------------------------
// Batch S1E — execution, notification and cancellation
// ---------------------------------------------------------------------------

// S1E-3/S1E-5. TS 23.401 §5.5.1.2.2 step 15: the MME sends the "eNodeB address and
// TEID allocated at the target eNodeB for downlink traffic on S1-U for the accepted
// EPS bearers" only after HANDOVER NOTIFY (step 13); step 19 then releases the
// source eNB's UE context.
func TestS1HandoverNotifySwitchesDownlinkAndReleasesTheSource(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	sourceMME, sourceENB := ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	// Before the notify the downlink still points at the source.
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

// S1E-2. TS 23.401 §5.5.1.2.2 step 15: "If the default bearer of a PDN connection
// has not been accepted by the target eNodeB and there are other PDN connections
// active, the MME shall handle it in the same way as if all bearers of a PDN
// connection have not been accepted. The MME releases these PDN connections by
// triggering the MME requested PDN disconnection procedure specified in clause
// 5.10.3."
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

// S1E-6. TS 36.413 §9.1.5.7 makes E-UTRAN CGI and TAI mandatory with **ignore**
// criticality in the HANDOVER NOTIFY. Per §10.3.5 their absence does not stop the
// procedure, so the handover still completes — the UE has already arrived.
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

// S1E-7/S1E-8/S1E-10. TS 36.413 §8.4.5.2: "Upon reception of a HANDOVER CANCEL
// message, the EPC shall terminate the ongoing Handover Preparation procedure,
// release any resources associated with the handover preparation and send a
// HANDOVER CANCEL ACKNOWLEDGE message to the source eNB." §9.1.5.12 makes both UE
// S1AP IDs mandatory in the acknowledge; TS 23.401 §5.5.2.5.2 step 3 adds that the
// source EPC node "resumes operation on the resources in the source side".
//
// The EPC's obligations here are spelled out in S1AP; NGAP §8.4.5.2 says only that
// the source node sends the message, and locates the same behaviour in
// TS 23.502 §4.9.1.4.
func TestS1HandoverCancelReleasesPreparationAndAcknowledges(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	sourceMME, sourceENB := ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	cause := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}

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

// S1E-9. TS 36.413 §9.1.5.11 makes Cause mandatory with ignore criticality in the
// HANDOVER CANCEL, so per §10.3.5 an absent one does not stop the cancellation.
func TestS1HandoverCancelWithoutCauseStillCancels(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	driveToPrepared(t, m, ue, source, target)

	targetBefore := target.count()

	body := dropIEs(t, initiatingValue(t, mustMarshal(t,
		(&s1ap.HandoverCancel{
			MMEUES1APID: ue.Conn().MMEUES1APID,
			ENBUES1APID: ue.Conn().ENBUES1APID,
			Cause:       s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}),
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

// ---------------------------------------------------------------------------
// Batch S1S — eNB / MME status transfer
// ---------------------------------------------------------------------------

// S1S-1/S1S-2/S1S-3. TS 23.401 §5.5.1.2.2 step 10: "The source eNodeB sends the
// eNodeB Status Transfer message to the target eNodeB **via the MME(s)** ... the
// source MME ... sends the information to the target eNodeB via the MME Status
// Transfer message." TS 36.413 §9.1.5.14 makes MME UE S1AP ID, eNB UE S1AP ID and
// the eNB Status Transfer Transparent Container mandatory in that message, and the
// UE S1AP IDs must address the target association.
func TestS1MMEStatusTransferAddressesTheTargetWithTheSameContainer(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	// The container is opaque to the MME (TS 36.413 §9.2.1.31).
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

// S1S-4. TS 23.401 §5.5.1.2.2 step 10: "The source eNodeB may omit sending this
// message if none of the E-RABs of the UE shall be treated with PDCP status
// preservation", so its absence must not gate completion.
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

// S1S-5. With no handover in progress there is no target association to relay to,
// so the MME has nothing to send (TS 36.413 §8.4.7 exists only between a prepared
// source and target pair).
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

// lastSent returns the raw bytes of the most recent message captured on a conn.
func lastSent(t *testing.T, cc *captureConn) []byte {
	t.Helper()

	cc.mu.Lock()
	defer cc.mu.Unlock()

	if len(cc.sent) == 0 {
		t.Fatal("no S1AP message captured")
	}

	return append([]byte(nil), cc.sent[len(cc.sent)-1]...)
}

// TS 36.413 §9.1.5.2: the E-RABs to Release List is an E-RAB List, so each entry
// carries a Cause. The target's own cause is the most informative one to relay
// when it gave one, which is what NGAP §8.4.1.2 does with the Handover Resource
// Allocation Unsuccessful Transfer.
func TestS1HandoverCommandRelaysTheTargetsReleaseCause(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)
	secondPDN(ue)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req := handoverRequestOn(t, target)

	// radio-resources-not-available (TS 36.413 §9.2.1.3).
	targetCause := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 24}

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
