// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/s1ap/aper"
)

func sampleTargetID() TargetID {
	return TargetID{TargeteNBID: TargeteNBID{
		GlobalENBID: GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{Kind: ENBIDMacro, Value: 0x00101}},
		SelectedTAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7},
	}}
}

// TestHandoverRequiredRoundTrip covers HANDOVER REQUIRED through the generic
// Unmarshal envelope, checking every modeled IE survives including the opaque
// source-to-target container.
func TestHandoverRequiredRoundTrip(t *testing.T) {
	in := &HandoverRequired{
		MMEUES1APID:    0x020000bf,
		ENBUES1APID:    2,
		HandoverType:   HandoverTypeIntraLTE,
		Cause:          Cause{Group: CauseGroupRadioNetwork, Value: 16}, // handover-desirable-for-radio-reasons
		TargetID:       sampleTargetID(),
		SourceToTarget: TransparentContainer{0x01, 0x02, 0x03, 0x04},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcHandoverPreparation {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverRequired(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID || out.HandoverType != in.HandoverType {
		t.Fatalf("ids/type: mme=%#x enb=%d type=%d", out.MMEUES1APID, out.ENBUES1APID, out.HandoverType)
	}

	if out.Cause != in.Cause {
		t.Fatalf("cause = %+v, want %+v", out.Cause, in.Cause)
	}

	if out.TargetID != in.TargetID {
		t.Fatalf("targetID = %+v, want %+v", out.TargetID, in.TargetID)
	}

	if !bytes.Equal(out.SourceToTarget, in.SourceToTarget) {
		t.Fatalf("source-to-target container = %x, want %x", out.SourceToTarget, in.SourceToTarget)
	}
}

// TestHandoverTypeRootValuesRoundTrip checks every root HandoverType decodes
// faithfully; restricting handover to intralte is an MME-layer policy, not a
// codec concern (TS 36.413).
func TestHandoverTypeRootValuesRoundTrip(t *testing.T) {
	for ht := HandoverTypeIntraLTE; ht <= HandoverTypeGERANtoLTE; ht++ {
		in := &HandoverRequired{
			MMEUES1APID:    1,
			ENBUES1APID:    2,
			HandoverType:   ht,
			Cause:          Cause{Group: CauseGroupRadioNetwork, Value: 16},
			TargetID:       sampleTargetID(),
			SourceToTarget: TransparentContainer{0x01},
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatalf("HandoverType %d: %v", ht, err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatalf("HandoverType %d: %v", ht, err)
		}

		out, err := ParseHandoverRequired(pdu.(*InitiatingMessage).Value)
		if err != nil {
			t.Fatalf("HandoverType %d: %v", ht, err)
		}

		if out.HandoverType != ht {
			t.Fatalf("HandoverType = %d, want %d", out.HandoverType, ht)
		}
	}
}

// TestTargetIDNonENBAlternativeRejected checks the parser rejects a TargetID
// CHOICE arm other than targeteNB-ID, which is out of scope (TS 36.413).
func TestTargetIDNonENBAlternativeRejected(t *testing.T) {
	var w aper.Writer

	// Encode TargetID with root choice index 1 (targetRNC-ID), unmodeled.
	if err := w.WriteChoiceIndex(1, targetIDRootCount, true, false); err != nil {
		t.Fatal(err)
	}

	if _, err := decodeTargetID(aper.NewReader(w.Bytes())); err == nil {
		t.Fatal("expected decodeTargetID to reject a non-targeteNB-ID alternative")
	}
}

// TestHandoverRequestRoundTrip covers HANDOVER REQUEST with its E-RABs To Be
// Setup list, AMBR, security context, and opaque container.
func TestHandoverRequestRoundTrip(t *testing.T) {
	var nh SecurityKey
	for i := range nh {
		nh[i] = byte(i + 1)
	}

	in := &HandoverRequest{
		MMEUES1APID:  0x020000bf,
		HandoverType: HandoverTypeIntraLTE,
		Cause:        Cause{Group: CauseGroupRadioNetwork, Value: 16},
		UEAMBR:       UEAggregateMaximumBitRate{DL: 1_000_000, UL: 500_000},
		ERABToBeSetup: []ERABToBeSetupItemHOReq{{
			ERABID:                5,
			TransportLayerAddress: TransportLayerAddress{10, 1, 2, 3},
			GTPTEID:               0x01020304,
			QoS:                   ERABLevelQoSParameters{QCI: 9, ARP: AllocationAndRetentionPriority{PriorityLevel: 15}},
		}},
		SourceToTarget:         TransparentContainer{0xaa, 0xbb},
		UESecurityCapabilities: UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000},
		SecurityContext:        SecurityContext{NextHopChainingCount: 3, NextHopParameter: nh},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcHandoverResourceAllocation {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverRequest(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.SecurityContext != in.SecurityContext {
		t.Fatalf("mme=%#x secctx=%+v", out.MMEUES1APID, out.SecurityContext)
	}

	if out.UEAMBR != in.UEAMBR || out.UESecurityCapabilities != in.UESecurityCapabilities {
		t.Fatalf("ambr=%+v caps=%+v", out.UEAMBR, out.UESecurityCapabilities)
	}

	if len(out.ERABToBeSetup) != 1 {
		t.Fatalf("E-RAB list len = %d, want 1", len(out.ERABToBeSetup))
	}

	got, want := out.ERABToBeSetup[0], in.ERABToBeSetup[0]
	if got.ERABID != want.ERABID || got.GTPTEID != want.GTPTEID || got.QoS.QCI != want.QoS.QCI ||
		!bytes.Equal(got.TransportLayerAddress, want.TransportLayerAddress) {
		t.Fatalf("E-RAB item = %+v, want %+v", got, want)
	}
}

// TestHandoverRequestAcknowledgeRoundTrip covers HANDOVER REQUEST ACKNOWLEDGE
// with an admitted E-RAB carrying the target DL endpoint, a failed E-RAB, and the
// opaque target-to-source container.
func TestHandoverRequestAcknowledgeRoundTrip(t *testing.T) {
	in := &HandoverRequestAcknowledge{
		MMEUES1APID: 0x020000bf,
		ENBUES1APID: 9,
		ERABAdmitted: []ERABAdmittedItem{{
			ERABID:                5,
			TransportLayerAddress: TransportLayerAddress{10, 9, 9, 9},
			GTPTEID:               0x0a0b0c0d,
		}},
		ERABFailedToSetup: []ERABItem{{ERABID: 6, Cause: Cause{Group: CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource:    TransparentContainer{0x11, 0x22, 0x33},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcHandoverResourceAllocation {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverRequestAcknowledge(so.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID {
		t.Fatalf("ids: mme=%#x enb=%d", out.MMEUES1APID, out.ENBUES1APID)
	}

	if len(out.ERABAdmitted) != 1 || out.ERABAdmitted[0].GTPTEID != in.ERABAdmitted[0].GTPTEID ||
		!bytes.Equal(out.ERABAdmitted[0].TransportLayerAddress, in.ERABAdmitted[0].TransportLayerAddress) {
		t.Fatalf("admitted = %+v", out.ERABAdmitted)
	}

	if len(out.ERABFailedToSetup) != 1 || out.ERABFailedToSetup[0].ERABID != 6 {
		t.Fatalf("failed = %+v", out.ERABFailedToSetup)
	}

	if !bytes.Equal(out.TargetToSource, in.TargetToSource) {
		t.Fatalf("target-to-source = %x, want %x", out.TargetToSource, in.TargetToSource)
	}
}

// TestHandoverRequestAcknowledgeForwardingTunnels covers the optional DL/UL
// data-forwarding F-TEIDs in an admitted E-RAB surviving a round-trip even though
// the MME ignores them.
func TestHandoverRequestAcknowledgeForwardingTunnels(t *testing.T) {
	dlTEID := GTPTEID(0x44556677)

	in := &HandoverRequestAcknowledge{
		MMEUES1APID: 1,
		ENBUES1APID: 9,
		ERABAdmitted: []ERABAdmittedItem{{
			ERABID:                5,
			TransportLayerAddress: TransportLayerAddress{10, 9, 9, 9},
			GTPTEID:               0x0a0b0c0d,
			DLTransportLayerAddr:  TransportLayerAddress{10, 8, 8, 8},
			DLGTPTEID:             &dlTEID,
		}},
		TargetToSource: TransparentContainer{0x01},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseHandoverRequestAcknowledge(pdu.(*SuccessfulOutcome).Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	it := out.ERABAdmitted[0]
	if it.DLGTPTEID == nil || *it.DLGTPTEID != dlTEID || !bytes.Equal(it.DLTransportLayerAddr, in.ERABAdmitted[0].DLTransportLayerAddr) {
		t.Fatalf("DL forwarding tunnel = %+v", it)
	}

	if it.ULGTPTEID != nil {
		t.Fatalf("unexpected UL forwarding tunnel = %+v", it.ULGTPTEID)
	}
}

// TestHandoverCommandRoundTrip covers HANDOVER COMMAND with a bearer-to-release
// list and the opaque target-to-source container.
func TestHandoverCommandRoundTrip(t *testing.T) {
	in := &HandoverCommand{
		MMEUES1APID:    7,
		ENBUES1APID:    2,
		HandoverType:   HandoverTypeIntraLTE,
		ERABToRelease:  []ERABItem{{ERABID: 6, Cause: Cause{Group: CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource: TransparentContainer{0x11, 0x22},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcHandoverPreparation {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverCommand(so.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID || out.HandoverType != in.HandoverType {
		t.Fatalf("ids/type: %+v", out)
	}

	if len(out.ERABToRelease) != 1 || out.ERABToRelease[0].ERABID != 6 {
		t.Fatalf("to-release = %+v", out.ERABToRelease)
	}

	if !bytes.Equal(out.TargetToSource, in.TargetToSource) {
		t.Fatalf("target-to-source = %x, want %x", out.TargetToSource, in.TargetToSource)
	}
}

// TestHandoverPreparationFailureRoundTrip covers HANDOVER PREPARATION FAILURE.
func TestHandoverPreparationFailureRoundTrip(t *testing.T) {
	in := &HandoverPreparationFailure{
		MMEUES1APID: 7,
		ENBUES1APID: 2,
		Cause:       Cause{Group: CauseGroupRadioNetwork, Value: 0},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcHandoverPreparation {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverPreparationFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID || out.Cause != in.Cause {
		t.Fatalf("failure = %+v, want %+v", out, in)
	}
}

// TestHandoverFailureRoundTrip covers HANDOVER FAILURE.
func TestHandoverFailureRoundTrip(t *testing.T) {
	in := &HandoverFailure{
		MMEUES1APID: 7,
		Cause:       Cause{Group: CauseGroupRadioNetwork, Value: 0},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcHandoverResourceAllocation {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.Cause != in.Cause {
		t.Fatalf("failure = %+v, want %+v", out, in)
	}
}

// TestHandoverNotifyRoundTrip covers HANDOVER NOTIFY with the UE's new location.
func TestHandoverNotifyRoundTrip(t *testing.T) {
	in := &HandoverNotify{
		MMEUES1APID: 7,
		ENBUES1APID: 9,
		EUTRANCGI:   EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x123c601},
		TAI:         TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcHandoverNotification {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverNotify(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID || out.EUTRANCGI != in.EUTRANCGI || out.TAI != in.TAI {
		t.Fatalf("notify = %+v, want %+v", out, in)
	}
}

// TestStatusTransferRelayRoundTrip covers ENB STATUS TRANSFER decoding and MME
// STATUS TRANSFER re-encoding of the same opaque container, the MME's relay path.
func TestStatusTransferRelayRoundTrip(t *testing.T) {
	container := StatusTransferContainer{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02}

	enb := &ENBStatusTransfer{MMEUES1APID: 7, ENBUES1APID: 2, Container: container}

	b, err := enb.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcENBStatusTransfer {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	parsed, err := ParseENBStatusTransfer(im.Value)
	if err != nil {
		t.Fatalf("parse enb: %v", err)
	}

	if !bytes.Equal(parsed.Container, container) {
		t.Fatalf("relayed container = %x, want %x", parsed.Container, container)
	}

	// Relay into an MME STATUS TRANSFER addressed to the target eNB, then decode
	// the container back out unchanged.
	mme := &MMEStatusTransfer{MMEUES1APID: parsed.MMEUES1APID, ENBUES1APID: 9, Container: parsed.Container}

	mb, err := mme.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	mpdu, err := Unmarshal(mb)
	if err != nil {
		t.Fatal(err)
	}

	mim, ok := mpdu.(*InitiatingMessage)
	if !ok || mim.ProcedureCode != ProcMMEStatusTransfer {
		t.Fatalf("got %T procedureCode %d", mpdu, mpdu.procedureCode())
	}

	mout, err := ParseMMEStatusTransfer(mim.Value)
	if err != nil {
		t.Fatalf("parse mme: %v", err)
	}

	if mout.ENBUES1APID != 9 || !bytes.Equal(mout.Container, container) {
		t.Fatalf("mme status transfer = %+v", mout)
	}
}

// TestHandoverCancelRoundTrip covers HANDOVER CANCEL and its acknowledge.
func TestHandoverCancelRoundTrip(t *testing.T) {
	in := &HandoverCancel{MMEUES1APID: 7, ENBUES1APID: 2, Cause: Cause{Group: CauseGroupRadioNetwork, Value: 5}}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcHandoverCancel {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverCancel(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID || out.Cause != in.Cause {
		t.Fatalf("cancel = %+v, want %+v", out, in)
	}

	ack := &HandoverCancelAcknowledge{MMEUES1APID: 7, ENBUES1APID: 2}

	ab, err := ack.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	apdu, err := Unmarshal(ab)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := apdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcHandoverCancel {
		t.Fatalf("got %T procedureCode %d", apdu, apdu.procedureCode())
	}

	aout, err := ParseHandoverCancelAcknowledge(so.Value)
	if err != nil {
		t.Fatalf("parse ack: %v", err)
	}

	if aout.MMEUES1APID != ack.MMEUES1APID || aout.ENBUES1APID != ack.ENBUES1APID {
		t.Fatalf("cancel ack = %+v", aout)
	}
}
