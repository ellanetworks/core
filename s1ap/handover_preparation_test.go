// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

func TestHandoverPreparationFailureRoundTrip(t *testing.T) {
	in := &HandoverPreparationFailure{
		MMEUES1APID: Ptr(MMEUES1APID(7)),
		ENBUES1APID: Ptr(ENBUES1APID(2)),
		Cause:       Ptr(CauseRadioNetworkUnspecified.Cause()),
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
		!deref(out.Cause).Equal(deref(in.Cause)) {
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
			DLTransportLayerAddr: Ptr(tlaOf(192, 0, 2, 1)),
			DLGTPTEID:            Ptr(GTPTEID(0x11223344)),
		}},
		ERABToRelease:  []ERABItem{{ERABID: 6, Cause: CauseRadioNetworkUnspecified.Cause()}},
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

	if !abstract.Cause.Equal(CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage.Cause()) {
		t.Errorf("cause %v, want falsely-constructed-message", abstract.Cause)
	}

	decoded, err := DecodeHandoverCommand(pdu.(*SuccessfulOutcome).Value)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	again, err := decoded.Marshal()
	if err != nil || !bytes.Equal(again, b) {
		t.Fatalf("Marshal(Decode(b)) = %x (%v), want %x", again, err, b)
	}
}

// encodedHandoverTypeIE is the HandoverType IE exactly as a message body
// carries it, built by the same encoder rather than by hand.
func encodedHandoverTypeIE(t *testing.T, v HandoverType) []byte {
	t.Helper()

	w := per.NewWriter()
	if err := encodeContainerField(w, per.Aligned, ieField{ID: IDHandoverType, Criticality: CriticalityReject, Value: v}); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	return w.Bytes()
}

func sampleTargetID() TargetID {
	return TargetID{TargeteNBID: &TargeteNBID{
		GlobalENBID: GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{MacroENBID: Ptr(MacroENBID(0x00101))}},
		SelectedTAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7},
	}}
}

func TestHandoverRequiredRoundTrip(t *testing.T) {
	in := &HandoverRequired{
		MMEUES1APID:    0x020000bf,
		ENBUES1APID:    2,
		HandoverType:   HandoverTypeIntraLTE,
		Cause:          Ptr(CauseRadioNetworkHandoverDesirableForRadio.Cause()), // handover-desirable-for-radio-reasons
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

	if !deref(out.Cause).Equal(deref(in.Cause)) {
		t.Fatalf("cause = %+v, want %+v", out.Cause, in.Cause)
	}

	if !out.TargetID.Equal(in.TargetID) {
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
			Cause:          Ptr(CauseRadioNetworkHandoverDesirableForRadio.Cause()),
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

func TestTargetIDOtherAlternativesRoundTrip(t *testing.T) {
	plmn := PLMNIdentity{0x00, 0xf1, 0x10}

	cases := []TargetID{
		{TargetRNCID: &TargetRNCID{LAI: LAI{PLMNIdentity: plmn, LAC: 0x1234}, RAC: Ptr(RAC(5)), RNCID: 4095, ExtendedRNCID: Ptr(uint16(65535))}},
		{CGI: &CGI{PLMNIdentity: plmn, LAC: 0x1234, CI: 0xbeef}},
		{Ext: &per.ExtensionAlternative{Index: 1, Value: "\x00"}},
	}

	for _, in := range cases {
		b := ieRaw(t, &in)

		got, err := unmarshalPERValue[TargetID](b)
		if err != nil {
			t.Fatalf("%+v: %v", in, err)
		}

		if !got.Equal(in) {
			t.Fatalf("decoded %+v, want %+v", got, in)
		}

		if out := ieRaw(t, &got); !bytes.Equal(out, b) {
			t.Fatalf("re-encoded % x, want % x", out, b)
		}

		if want := in.Ext != nil; got.Diagnose(&ap.Diagnosis[ProtocolIEID]{}) != want {
			t.Fatalf("%+v: Diagnose != %v", in, want)
		}
	}
}

func TestHandoverCommandRoundTrip(t *testing.T) {
	in := &HandoverCommand{
		MMEUES1APID:    7,
		ENBUES1APID:    2,
		HandoverType:   HandoverTypeIntraLTE,
		ERABToRelease:  []ERABItem{{ERABID: 6, Cause: CauseRadioNetworkUnspecified.Cause()}},
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

// TS 36.413 §9.1.5.4 condition iffromUTRANGERAN: the NAS Security Parameters to
// E-UTRAN IE is present exactly when Handover Type is UTRANtoLTE or GERANtoLTE.
// It is the mirror of the iftoUTRANGERAN condition on HANDOVER COMMAND, and both
// halves are enforced so a HANDOVER REQUEST cannot go out missing a mandatory
// conditional reject IE.
func TestHandoverRequestNASSecurityParametersCondition(t *testing.T) {
	base := func(ht HandoverType) *HandoverRequest {
		return &HandoverRequest{
			MMEUES1APID:            1,
			HandoverType:           ht,
			Cause:                  Ptr(CauseRadioNetworkUnspecified.Cause()),
			UEAMBR:                 UEAggregateMaximumBitRate{DL: 1000, UL: 1000},
			ERABToBeSetup:          []ERABToBeSetupItemHOReq{{ERABID: 5, TransportLayerAddress: goldTLA()}},
			SourceToTarget:         TransparentContainer{0x01},
			UESecurityCapabilities: UESecurityCapabilities{EncryptionAlgorithms: SecurityAlgorithms{Value: 0x8000}},
		}
	}

	tests := []struct {
		name    string
		ht      HandoverType
		params  NASSecurityParameterstoEUTRAN
		wantErr bool
	}{
		{"intra-LTE without the IE", HandoverTypeIntraLTE, nil, false},
		{"intra-LTE with the IE", HandoverTypeIntraLTE, NASSecurityParameterstoEUTRAN{0xaa}, true},
		{"UTRAN to LTE without the IE", HandoverTypeUTRANtoLTE, nil, true},
		{"UTRAN to LTE with the IE", HandoverTypeUTRANtoLTE, NASSecurityParameterstoEUTRAN{0xaa}, false},
		{"GERAN to LTE without the IE", HandoverTypeGERANtoLTE, nil, true},
		{"GERAN to LTE with the IE", HandoverTypeGERANtoLTE, NASSecurityParameterstoEUTRAN{0xaa}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := base(tt.ht)
			req.NASSecurityParameterstoEUTRAN = tt.params

			_, err := req.Marshal()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestHandoverTypeInterSystemValues pins the two values TS 36.413 §9.2.1.13 adds
// after its extension marker. They are extension additions here and root values
// in TS 38.413 §9.3.1.22, so the two enumerations share no wire form and a value
// copied across without re-encoding names a different handover.
func TestHandoverTypeInterSystemValues(t *testing.T) {
	tests := []struct {
		name string
		typ  HandoverType
		want []byte
	}{
		// Extension bit set, then a normally-small index into the additions.
		{"eps-to-5gs", HandoverTypeEPSToFiveGS, []byte{0x80}},
		{"fivegs-to-eps", HandoverTypeFiveGSToEPS, []byte{0x81}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := tc.typ.MarshalPER(w, per.Aligned); err != nil {
				t.Fatal(err)
			}

			w.AlignToByte()

			if raw := w.Bytes(); !bytes.Equal(raw, tc.want) {
				t.Fatalf("%s = %x, want %x", tc.name, raw, tc.want)
			}

			var back HandoverType
			if err := back.UnmarshalPER(per.NewReader(tc.want), per.Aligned); err != nil {
				t.Fatal(err)
			}

			if back != tc.typ {
				t.Fatalf("round trip = %d, want %d", back, tc.typ)
			}
		})
	}

	// The root values still encode as they did, unshifted by the additions.
	w := per.NewWriter()
	if err := HandoverTypeIntraLTE.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	if raw := w.Bytes(); !bytes.Equal(raw, []byte{0x00}) {
		t.Fatalf("intralte = %x, want 00", raw)
	}

	for _, k := range []int64{2, 3, 200} {
		var v HandoverType

		in := extensionEnum(t, int64(len(enumHandoverType.Root)), k)
		if err := v.UnmarshalPER(per.NewReader(in), per.Aligned); err != nil {
			t.Fatalf("extension %d: %v", k, err)
		}

		if !v.Diagnose(&ap.Diagnosis[ProtocolIEID]{}) {
			t.Errorf("extension %d decoded as known value %s", k, v.Name())
		}

		if out := ieRaw(t, &v); !bytes.Equal(out, in) {
			t.Errorf("extension %d re-encoded % x, want % x", k, out, in)
		}
	}
}

func sampleTargetNgRanNodeID() TargetID {
	return TargetID{TargetNgRanNodeID: &TargetNgRanNodeID{
		GlobalRANNodeID: GlobalRANNodeID{GNB: &GNB{GlobalGNBID: GlobalGNBID{
			PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
			GNBID:        GNBIdentity{GNBID: &GNBID{Value: 0x00101, Bits: 22}},
		}}},
		SelectedTAI: FiveGSTAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 0x000007},
	}}
}

// TestHandoverRequiredTargetsNGRAN covers the targetgNgRanNode-ID extension
// alternative TS 36.413 adds to TargetID, which is what an eps-to-5gs HANDOVER
// REQUIRED names its target with. Before this the whole extension branch was
// refused, so the message could not be read at all.
func TestHandoverRequiredTargetsNGRAN(t *testing.T) {
	in := &HandoverRequired{
		MMEUES1APID:    0x020000bf,
		ENBUES1APID:    2,
		HandoverType:   HandoverTypeEPSToFiveGS,
		Cause:          Ptr(CauseRadioNetworkHandoverDesirableForRadio.Cause()),
		TargetID:       sampleTargetNgRanNodeID(),
		SourceToTarget: TransparentContainer{0x01, 0x02, 0x03, 0x04},
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
	if !ok || im.ProcedureCode != ProcHandoverPreparation {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverRequired(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.HandoverType != HandoverTypeEPSToFiveGS {
		t.Errorf("handover type = %d, want eps-to-5gs", out.HandoverType)
	}

	node := out.TargetID.TargetNgRanNodeID
	if node == nil {
		t.Fatalf("target = %+v, want a targetgNgRanNode-ID", out.TargetID)
	}

	if node.GlobalRANNodeID.GNB == nil {
		t.Fatalf("global RAN node = %+v, want a gNB", node.GlobalRANNodeID)
	}

	if got := node.GlobalRANNodeID.GNB.GlobalGNBID.GNBID; got.GNBID == nil || *got.GNBID != (GNBID{Value: 0x00101, Bits: 22}) {
		t.Errorf("gNB-ID = %+v, want {0x00101 22}", got)
	}

	if node.SelectedTAI.TAC != 0x000007 {
		t.Errorf("selected 5GS TAC = %d, want 7", node.SelectedTAI.TAC)
	}

	// The eNB alternative still reads, so adding the extension did not shift the
	// root.
	if _, err := ParseHandoverRequired(mustMarshalHandoverRequired(t, sampleTargetID())); err != nil {
		t.Fatalf("targeteNB-ID: %v", err)
	}
}

func mustMarshalHandoverRequired(t *testing.T, target TargetID) []byte {
	t.Helper()

	m := &HandoverRequired{
		MMEUES1APID:    1,
		ENBUES1APID:    2,
		HandoverType:   HandoverTypeIntraLTE,
		Cause:          Ptr(CauseRadioNetworkHandoverDesirableForRadio.Cause()),
		TargetID:       target,
		SourceToTarget: TransparentContainer{0x01},
	}

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return pdu.(*InitiatingMessage).Value
}

// TestGlobalRANNodeIDAlternatives covers the ng-eNB half of the CHOICE and the
// states that name no node at all.
func TestGlobalRANNodeIDAlternatives(t *testing.T) {
	ngENB := GlobalRANNodeID{NgENB: &NgENB{GlobalNgENBID: GlobalENBID{
		PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
		ENBID:        ENBID{LongMacroENBID: Ptr(LongMacroENBID(0x1F0FF))},
	}}}

	w := per.NewWriter()
	if err := ngENB.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	var back GlobalRANNodeID
	if err := back.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned); err != nil {
		t.Fatal(err)
	}

	if back.NgENB == nil || back.GNB != nil {
		t.Fatalf("round trip = %+v, want an ng-eNB", back)
	}

	if !back.NgENB.GlobalNgENBID.ENBID.Equal(ngENB.NgENB.GlobalNgENBID.ENBID) {
		t.Fatalf("ng-eNB id = %+v, want %+v", back.NgENB.GlobalNgENBID.ENBID, ngENB.NgENB.GlobalNgENBID.ENBID)
	}

	bad := []struct {
		name string
		id   GlobalRANNodeID
	}{
		{"no alternative", GlobalRANNodeID{}},
		{"both alternatives", GlobalRANNodeID{GNB: &GNB{}, NgENB: &NgENB{}}},
		{"gNB-ID narrower than 22 bits", GlobalRANNodeID{GNB: &GNB{GlobalGNBID: GlobalGNBID{
			GNBID: GNBIdentity{GNBID: &GNBID{Value: 1, Bits: 21}},
		}}}},
		{"gNB-ID wider than 32 bits", GlobalRANNodeID{GNB: &GNB{GlobalGNBID: GlobalGNBID{
			GNBID: GNBIdentity{GNBID: &GNBID{Value: 1, Bits: 33}},
		}}}},
		{"gNB id wider than its own bits", GlobalRANNodeID{GNB: &GNB{GlobalGNBID: GlobalGNBID{
			GNBID: GNBIdentity{GNBID: &GNBID{Value: 1 << 23, Bits: 22}},
		}}}},
	}

	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := tc.id.MarshalPER(w, per.Aligned); err == nil {
				t.Errorf("encoded as %x, want an error", w.Bytes())
			}
		})
	}
}

// TestHandoverRequiredDirectForwardingPathAvailability covers the IE
// TS 36.413 §9.1.5.1 gives HANDOVER REQUIRED and §9.2.1.44 defines: the source
// eNB reporting that a direct forwarding path to the target exists. It is the
// EPS→5GS input to the forwarding decision the SMF then signals as the
// "Direct Forwarding Path Availability" indication (TS 23.502 §4.11.1.2.2.2
// step 4), and being optional-ignore it was silently dropped before — decoded
// into the unmodelled-IE bucket rather than reaching the MME.
func TestHandoverRequiredDirectForwardingPathAvailability(t *testing.T) {
	in := &HandoverRequired{
		MMEUES1APID:                      1,
		ENBUES1APID:                      2,
		HandoverType:                     HandoverTypeEPSToFiveGS,
		Cause:                            Ptr(CauseRadioNetworkHandoverDesirableForRadio.Cause()),
		TargetID:                         sampleTargetNgRanNodeID(),
		DirectForwardingPathAvailability: Ptr(DirectForwardingPathAvailable),
		SourceToTarget:                   TransparentContainer{0x01},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseHandoverRequired(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.DirectForwardingPathAvailability == nil ||
		*out.DirectForwardingPathAvailability != DirectForwardingPathAvailable {
		t.Fatalf("direct forwarding path availability = %v, want available", out.DirectForwardingPathAvailability)
	}

	// "Not available" is the IE's absence, not a second enumeration value, so it
	// has to stay a nil pointer rather than decode as DirectForwardingPathAvailable.
	in.DirectForwardingPathAvailability = nil

	b, err = in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err = Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err = ParseHandoverRequired(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.DirectForwardingPathAvailability != nil {
		t.Fatalf("availability = %v, want absent", *out.DirectForwardingPathAvailability)
	}
}
