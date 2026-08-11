// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

const goldenHandoverPreparationFailure = "400c0015000003000a40020001005540020002000f400201c0"

func TestHandoverPreparationFailureGolden(t *testing.T) {
	amfID, ranID := AMFUENGAPID(1), RANUENGAPID(2)
	cause := Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHOFailureInTarget}

	in := HandoverPreparationFailure{AMFUENGAPID: &amfID, RANUENGAPID: &ranID, Cause: &cause}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenHandoverPreparationFailure {
		t.Fatalf("encoded %s, want %s", got, goldenHandoverPreparationFailure)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcHandoverPreparation {
		t.Fatalf("decoded %T, want an unsuccessful HandoverPreparation outcome", pdu)
	}

	out, err := ParseHandoverPreparationFailure(uo.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 1 || out.RANUENGAPID == nil || *out.RANUENGAPID != 2 ||
		out.Cause == nil || *out.Cause != cause {
		t.Fatalf("round trip %+v, want 1/2 and %v", out, cause)
	}

	if out.CriticalityDiagnostics != nil {
		t.Errorf("CriticalityDiagnostics = %+v, want nil", out.CriticalityDiagnostics)
	}
}

// Every IE here is ignore criticality, including the mandatory ones, so §10.3.5
// never rejects the procedure — the AMF sees the absence in diagnostics instead.
func TestHandoverPreparationFailureMissingMandatoryIEsAreIgnored(t *testing.T) {
	out, err := ParseHandoverPreparationFailure(container(t))
	if err != nil {
		t.Fatalf("an all-ignore message must not reject the procedure: %v", err)
	}

	if out.AMFUENGAPID != nil || out.RANUENGAPID != nil || out.Cause != nil {
		t.Errorf("decoded %+v, want every field absent", out)
	}

	ies := out.meta().diagnostics.IEs
	if len(ies) != 3 {
		t.Fatalf("diagnostics = %+v, want three missing entries", ies)
	}

	for _, ie := range ies {
		if ie.TypeOfError != TypeOfErrorMissing {
			t.Errorf("IE %v reported %v, want missing", ie.ID, ie.TypeOfError)
		}
	}
}

// A truncated body is a transfer-syntax error, not a missing-IE one.
func TestHandoverPreparationFailureRejectsTruncated(t *testing.T) {
	if _, err := ParseHandoverPreparationFailure([]byte{0x00}); err == nil {
		t.Fatal("parsed a truncated body")
	} else {
		var tse *TransferSyntaxError
		if !errors.As(err, &tse) {
			t.Fatalf("error = %T (%v), want *TransferSyntaxError", err, err)
		}
	}
}

const goldenHandoverRequired = "000c003d000007000a00020001005500020002001d000100000f4002040000" +
	"69000f0002f839100000010002f839000001003d000500000501000065000302aabb"

func TestHandoverRequiredGolden(t *testing.T) {
	cause := Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHandoverDesirableForRadio}
	plmn := PLMNIdentity{0x02, 0xf8, 0x39}

	transfer, err := (&HandoverRequiredTransfer{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	in := HandoverRequired{
		AMFUENGAPID: 1, RANUENGAPID: 2,
		HandoverType: HandoverTypeIntra5GS,
		Cause:        &cause,
		TargetID: TargetID{TargetRANNodeID: &TargetRANNodeID{
			GlobalRANNodeID: GlobalRANNodeID{Kind: RANNodeIDGNB, PLMNIdentity: plmn, Value: 1, Bits: 24},
			SelectedTAI:     TAI{PLMNIdentity: plmn, TAC: 1},
		}},
		PDUSessionResourceListHORqd:        PDUSessionResourceListHORqd{{PDUSessionID: 5, Transfer: transfer}},
		SourceToTargetTransparentContainer: SourceToTargetTransparentContainer{0xaa, 0xbb},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenHandoverRequired {
		t.Fatalf("encoded %s, want %s", got, goldenHandoverRequired)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcHandoverPreparation {
		t.Fatalf("decoded %T, want an initiating HandoverPreparation", pdu)
	}

	out, err := ParseHandoverRequired(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.HandoverType != HandoverTypeIntra5GS || out.TargetID.TargetRANNodeID.SelectedTAI.TAC != 1 ||
		len(out.PDUSessionResourceListHORqd) != 1 || out.PDUSessionResourceListHORqd[0].PDUSessionID != 5 {
		t.Fatalf("round trip %+v", out)
	}
}

// TestTargetIDTargetENBID covers the targeteNB-ID alternative, which is what a
// source gNB names its target with when Handover Type is fivegs-to-eps. Its
// tracking area is an EPS one — two octets of TAC where the 5GS TAI carries
// three — so it cannot be read as a targetRANNodeID and back again.
func TestTargetIDTargetENBID(t *testing.T) {
	in := TargetID{TargeteNBID: &TargeteNBID{
		GlobalENBID: GlobalNgENBID{
			PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
			NgENBID:      NgENBID{Kind: NgENBIDMacro, Value: 0x00101},
		},
		SelectedEPSTAI: EPSTAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7},
	}}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	var out TargetID
	if err := out.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned); err != nil {
		t.Fatal(err)
	}

	if out.TargeteNBID == nil || out.TargetRANNodeID != nil {
		t.Fatalf("round trip = %+v, want a targeteNB-ID", out)
	}

	if out.TargeteNBID.SelectedEPSTAI.TAC != 7 {
		t.Errorf("selected EPS TAC = %d, want 7", out.TargeteNBID.SelectedEPSTAI.TAC)
	}

	if got := out.TargeteNBID.GlobalENBID.NgENBID; got != in.TargeteNBID.GlobalENBID.NgENBID {
		t.Errorf("ng-eNB id = %+v, want %+v", got, in.TargeteNBID.GlobalENBID.NgENBID)
	}

	// The states that name no target, or two.
	for name, bad := range map[string]TargetID{
		"no alternative":    {},
		"both alternatives": {TargetRANNodeID: &TargetRANNodeID{}, TargeteNBID: &TargeteNBID{}},
	} {
		w := per.NewWriter()
		if err := bad.MarshalPER(w, per.Aligned); err == nil {
			t.Errorf("%s: encoded as %x, want an error", name, w.Bytes())
		}
	}
}

// choice-Extensions still names a target this AMF cannot reach, and is refused
// rather than misread.
func TestTargetIDRejectsChoiceExtensions(t *testing.T) {
	w := per.NewWriter()
	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, targetIDAlternatives-1, targetIDChoiceExtensions); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(idTargetID)); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(CriticalityReject)); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0x00}); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	var v TargetID

	err := v.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned)
	if err == nil {
		t.Fatal("choice-Extensions decoded")
	}

	if !errors.Is(err, errNotComprehended) {
		t.Errorf("err = %v, want errNotComprehended so criticality decides", err)
	}
}

// TS 38.413 gives HandoverType three root members; TS 36.413 gives its own
// HandoverType five, with no shared wire values.
func TestHandoverTypeRootCountMatchesSpec(t *testing.T) {
	if handoverTypeRootCount != 3 {
		t.Errorf("root count is %d, TS 38.413 §9.3.1.22 defines three members before the extension marker", handoverTypeRootCount)
	}
}

const goldenHandoverCommand = "200c002e000006000a00020001005500020002001d000100003b4005000005" +
	"0100004e4006000006020100006a000302ccdd"

func TestHandoverCommandGolden(t *testing.T) {
	admitted, err := (&HandoverCommandTransfer{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	refused, err := (&HandoverPreparationUnsuccessfulTransfer{
		Cause: Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHandoverDesirableForRadio},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	in := HandoverCommand{
		AMFUENGAPID: 1, RANUENGAPID: 2,
		HandoverType:                       HandoverTypeIntra5GS,
		PDUSessionResourceHandoverList:     PDUSessionResourceHandoverList{{PDUSessionID: 5, Transfer: admitted}},
		PDUSessionResourceToReleaseList:    PDUSessionResourceToReleaseListHOCmd{{PDUSessionID: 6, Transfer: refused}},
		TargetToSourceTransparentContainer: TargetToSourceTransparentContainer{0xcc, 0xdd},
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

	if len(out.PDUSessionResourceHandoverList) != 1 || out.PDUSessionResourceHandoverList[0].PDUSessionID != 5 ||
		len(out.PDUSessionResourceToReleaseList) != 1 || out.PDUSessionResourceToReleaseList[0].PDUSessionID != 6 ||
		!bytes.Equal(out.TargetToSourceTransparentContainer, []byte{0xcc, 0xdd}) {
		t.Fatalf("round trip %+v", out)
	}

	unsuccessful, err := ParseHandoverPreparationUnsuccessfulTransfer(out.PDUSessionResourceToReleaseList[0].Transfer)
	if err != nil {
		t.Fatal(err)
	}

	if unsuccessful.Cause.Group != CauseGroupRadioNetwork ||
		unsuccessful.Cause.Value != CauseRadioNetworkHandoverDesirableForRadio {
		t.Errorf("nested cause %+v", unsuccessful.Cause)
	}
}

// TS 38.413 §9.2.3.2 condition iftoEPSUTRA: the NAS security parameters are
// carried when, and only when, the handover leaves 5GS.
func TestHandoverCommandNASSecurityParametersCondition(t *testing.T) {
	base := func() HandoverCommand {
		return HandoverCommand{
			AMFUENGAPID: 1, RANUENGAPID: 2,
			TargetToSourceTransparentContainer: TargetToSourceTransparentContainer{0xcc, 0xdd},
		}
	}

	leaving := base()
	leaving.HandoverType = HandoverTypeFiveGSToEPS

	if _, err := leaving.Marshal(); err == nil {
		t.Error("encoded a 5GStoEPS command with no NAS security parameters")
	}

	staying := base()
	staying.HandoverType = HandoverTypeIntra5GS
	staying.NASSecurityParametersFromNGRAN = NASSecurityParametersFromNGRAN{0x01}

	if _, err := staying.Marshal(); err == nil {
		t.Error("encoded an intra5gs command carrying NAS security parameters")
	}

	leaving.NASSecurityParametersFromNGRAN = NASSecurityParametersFromNGRAN{0x01}

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

	if !bytes.Equal(out.NASSecurityParametersFromNGRAN, []byte{0x01}) {
		t.Errorf("round trip %v", out.NASSecurityParametersFromNGRAN)
	}
}

// A peer that carries the conditional IE when the condition is false has built
// the message wrongly (§10.3.6), and the receiver says so rather than keeping it.
func TestHandoverCommandRejectsNASSecurityParametersWithoutCondition(t *testing.T) {
	leaving := HandoverCommand{
		AMFUENGAPID: 1, RANUENGAPID: 2,
		HandoverType:                       HandoverTypeFiveGSToEPS,
		NASSecurityParametersFromNGRAN:     NASSecurityParametersFromNGRAN{0x01},
		TargetToSourceTransparentContainer: TargetToSourceTransparentContainer{0xcc, 0xdd},
	}

	b, err := leaving.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Rewriting the encoded HandoverType to intra5gs leaves the NAS security
	// parameters stranded, which is the message only a peer would build.
	from, to := encodedHandoverTypeIE(t, HandoverTypeFiveGSToEPS), encodedHandoverTypeIE(t, HandoverTypeIntra5GS)

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
