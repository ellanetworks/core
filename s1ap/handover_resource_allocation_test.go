// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestHandoverRequestRoundTrip(t *testing.T) {
	var nh SecurityKey
	for i := range nh {
		nh[i] = byte(i + 1)
	}

	in := &HandoverRequest{
		MMEUES1APID:  0x020000bf,
		HandoverType: HandoverTypeIntraLTE,
		Cause:        Ptr(Cause{Group: CauseGroupRadioNetwork, Value: 16}),
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

func TestHandoverRequestAcknowledgeRoundTrip(t *testing.T) {
	in := &HandoverRequestAcknowledge{
		MMEUES1APID: Ptr(MMEUES1APID(0x020000bf)),
		ENBUES1APID: Ptr(ENBUES1APID(9)),
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

	if deref(out.MMEUES1APID) != deref(in.MMEUES1APID) || deref(out.ENBUES1APID) != deref(in.ENBUES1APID) {
		t.Fatalf("ids: mme=%#x enb=%d", deref(out.MMEUES1APID), deref(out.ENBUES1APID))
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

func TestHandoverRequestAcknowledgeForwardingTunnels(t *testing.T) {
	dlTEID := GTPTEID(0x44556677)

	in := &HandoverRequestAcknowledge{
		MMEUES1APID: Ptr(MMEUES1APID(1)),
		ENBUES1APID: Ptr(ENBUES1APID(9)),
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

func TestHandoverFailureRoundTrip(t *testing.T) {
	in := &HandoverFailure{
		MMEUES1APID: Ptr(MMEUES1APID(7)),
		Cause:       Ptr(Cause{Group: CauseGroupRadioNetwork, Value: 0}),
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

	if deref(out.MMEUES1APID) != deref(in.MMEUES1APID) || deref(out.Cause) != deref(in.Cause) {
		t.Fatalf("failure = %+v, want %+v", out, in)
	}
}

// TS 36.413 §9.3.4: E-RABFailedToSetupListHOReqAck is the one "failed" list
// that is not an E-RABList, so its items ride id-E-RABFailedtoSetupItemHOReqAck
// (21) and not id-E-RABItem (35). Both item types are
// SEQUENCE { e-RAB-ID, cause, iE-Extensions OPTIONAL, ... }, so only the
// container id tells them apart, and a round trip cannot: decodeItemList
// discards the id it read.
func TestHandoverRequestAcknowledgeFailedListItemID(t *testing.T) {
	b, err := (&HandoverRequestAcknowledge{
		MMEUES1APID:       Ptr(MMEUES1APID(1)),
		ENBUES1APID:       Ptr(ENBUES1APID(2)),
		ERABAdmitted:      []ERABAdmittedItem{{ERABID: 0}},
		ERABFailedToSetup: []ERABItem{{ERABID: 4, Cause: Cause{Group: CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource:    TransparentContainer{0x00},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// The outer id-E-RABFailedToSetupListHOReqAck (19) container holds one
	// single container keyed 0x0015 = 21.
	const want = "200100310000050000400200010008400200020012400c000014400700100000000000001340080000154003080000007b00020100"

	if got := hex.EncodeToString(b); got != want {
		t.Fatalf("encoded\n got %s\nwant %s", got, want)
	}
}
