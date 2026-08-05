// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestHandoverPreparationFailureRoundTrip(t *testing.T) {
	in := &HandoverPreparationFailure{
		MMEUES1APID: Ptr(MMEUES1APID(7)),
		ENBUES1APID: Ptr(ENBUES1APID(2)),
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
	if !ok || uo.ProcedureCode != ProcHandoverPreparation {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverPreparationFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if deref(out.MMEUES1APID) != deref(in.MMEUES1APID) || deref(out.ENBUES1APID) != deref(in.ENBUES1APID) ||
		deref(out.Cause) != deref(in.Cause) {
		t.Fatalf("failure = %+v, want %+v", out, in)
	}
}

const goldenHandoverCommand = "2000003b0000060000000200070008000200020001000100000c401000000e" +
	"400b60a1f0c000020111223344000d400800002340030c0000007b000302ccdd"

func TestHandoverCommandGolden(t *testing.T) {
	in := HandoverCommand{
		MMEUES1APID: 7, ENBUES1APID: 2,
		HandoverType: HandoverTypeIntraLTE,
		ERABSubjecttoDataForwarding: []ERABDataForwardingItem{{
			ERABID:               5,
			DLTransportLayerAddr: TransportLayerAddress{192, 0, 2, 1},
			DLGTPTEID:            Ptr(GTPTEID(0x11223344)),
		}},
		ERABToRelease:  []ERABItem{{ERABID: 6, Cause: Cause{Group: CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource: TransparentContainer{0xcc, 0xdd},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenHandoverCommand {
		t.Fatalf("encoded %s, want %s", got, goldenHandoverCommand)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcHandoverPreparation {
		t.Fatalf("decoded %T, want a successful HandoverPreparation outcome", pdu)
	}

	out, err := ParseHandoverCommand(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.ERABSubjecttoDataForwarding) != 1 || out.ERABSubjecttoDataForwarding[0].ERABID != 5 ||
		deref(out.ERABSubjecttoDataForwarding[0].DLGTPTEID) != 0x11223344 ||
		len(out.ERABToRelease) != 1 || out.ERABToRelease[0].ERABID != 6 ||
		!bytes.Equal(out.TargetToSource, []byte{0xcc, 0xdd}) {
		t.Fatalf("round trip %+v", out)
	}
}

// TS 36.413 §9.1.5.2 condition iftoUTRANGERAN: the NAS security parameters are
// carried when, and only when, the handover leaves E-UTRAN.
func TestHandoverCommandNASSecurityParametersCondition(t *testing.T) {
	base := func() HandoverCommand {
		return HandoverCommand{
			MMEUES1APID: 7, ENBUES1APID: 2,
			TargetToSource: TransparentContainer{0xcc, 0xdd},
		}
	}

	leaving := base()
	leaving.HandoverType = HandoverTypeLTEtoUTRAN

	if _, err := leaving.Marshal(); err == nil {
		t.Error("encoded an ltetoutran command with no NAS security parameters")
	}

	staying := base()
	staying.HandoverType = HandoverTypeIntraLTE
	staying.NASSecurityParametersfromEUTRAN = NASSecurityParametersfromEUTRAN{0x01}

	if _, err := staying.Marshal(); err == nil {
		t.Error("encoded an intralte command carrying NAS security parameters")
	}

	leaving.NASSecurityParametersfromEUTRAN = NASSecurityParametersfromEUTRAN{0x01}

	b, err := leaving.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseHandoverCommand(pdu.(*SuccessfulOutcome).Value)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out.NASSecurityParametersfromEUTRAN, []byte{0x01}) {
		t.Errorf("round trip %v", out.NASSecurityParametersfromEUTRAN)
	}
}

// A peer that carries the conditional IE when the condition is false has built
// the message wrongly (§10.3.6), and the receiver says so rather than keeping it.
func TestHandoverCommandRejectsNASSecurityParametersWithoutCondition(t *testing.T) {
	leaving := HandoverCommand{
		MMEUES1APID: 7, ENBUES1APID: 2,
		HandoverType:                    HandoverTypeLTEtoUTRAN,
		NASSecurityParametersfromEUTRAN: NASSecurityParametersfromEUTRAN{0x01},
		TargetToSource:                  TransparentContainer{0xcc, 0xdd},
	}

	b, err := leaving.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Rewriting the encoded HandoverType to intralte leaves the NAS security
	// parameters stranded, which is the message only a peer would build.
	from, to := encodedHandoverTypeIE(t, HandoverTypeLTEtoUTRAN), encodedHandoverTypeIE(t, HandoverTypeIntraLTE)

	i := bytes.Index(b, from)
	if i < 0 {
		t.Fatalf("no HandoverType IE in %s", hex.EncodeToString(b))
	}

	copy(b[i:], to)

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseHandoverCommand(pdu.(*SuccessfulOutcome).Value)

	var abstract *AbstractSyntaxError
	if !errors.As(err, &abstract) {
		t.Fatalf("err = %v, want an AbstractSyntaxError", err)
	}

	if abstract.Cause.Value != CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage {
		t.Errorf("cause %v, want falsely-constructed-message", abstract.Cause)
	}
}

// encodedHandoverTypeIE is the HandoverType IE exactly as a message body
// carries it, built by the same encoder rather than by hand.
func encodedHandoverTypeIE(t *testing.T, v HandoverType) []byte {
	t.Helper()

	w := per.NewWriter()
	if err := encodeContainerField(w, per.Aligned, ieField{id: idHandoverType, crit: CriticalityReject, val: v}); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	return w.Bytes()
}

func sampleTargetID() TargetID {
	return TargetID{TargeteNBID: TargeteNBID{
		GlobalENBID: GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{Kind: ENBIDMacro, Value: 0x00101}},
		SelectedTAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7},
	}}
}

func TestHandoverRequiredRoundTrip(t *testing.T) {
	in := &HandoverRequired{
		MMEUES1APID:    0x020000bf,
		ENBUES1APID:    2,
		HandoverType:   HandoverTypeIntraLTE,
		Cause:          Ptr(Cause{Group: CauseGroupRadioNetwork, Value: 16}), // handover-desirable-for-radio-reasons
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

	if deref(out.Cause) != deref(in.Cause) {
		t.Fatalf("cause = %+v, want %+v", out.Cause, in.Cause)
	}

	if out.TargetID != in.TargetID {
		t.Fatalf("targetID = %+v, want %+v", out.TargetID, in.TargetID)
	}

	if !bytes.Equal(out.SourceToTarget, in.SourceToTarget) {
		t.Fatalf("source-to-target container = %x, want %x", out.SourceToTarget, in.SourceToTarget)
	}
}

// Restricting handover to intralte is MME policy, not a codec concern.
func TestHandoverTypeRootValuesRoundTrip(t *testing.T) {
	for ht := HandoverTypeIntraLTE; ht <= HandoverTypeGERANtoLTE; ht++ {
		in := &HandoverRequired{
			MMEUES1APID:    1,
			ENBUES1APID:    2,
			HandoverType:   ht,
			Cause:          Ptr(Cause{Group: CauseGroupRadioNetwork, Value: 16}),
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

func TestTargetIDNonENBAlternativeRejected(t *testing.T) {
	w := per.NewWriter()

	// Encode TargetID with root choice index 1 (targetRNC-ID), unmodeled.
	if err := func() error {
		w.WriteBit(false)
		return per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, targetIDRootCount-1, 1)
	}(); err != nil {
		t.Fatal(err)
	}

	if _, err := unmarshalPERValue[TargetID](perBytes(w)); err == nil {
		t.Fatal("expected decodeTargetID to reject a non-targeteNB-ID alternative")
	}
}

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
