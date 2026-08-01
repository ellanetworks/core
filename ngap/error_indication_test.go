// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

// Golden ERROR INDICATION PDUs from a second, independent NGAP implementation
// (free5gc/ngap v1.1.3) encoding the same message.
//
// Neither carries the 5G-S-TMSI: that implementation predates the IE's addition
// to this procedure, so it cannot express it. The IE is pinned separately by
// TestErrorIndicationFiveGSTMSI and TestEncodeBodyGolden instead.
const (
	goldenErrorIndication     = "00094018000003000a400680ffffffffff005540020009000f400162"
	goldenErrorIndicationFull = "00094024000004000a400680ffffffffff005540020009000f400162001340087815000000001b40"
)

func goldErrorIndication() *ErrorIndication {
	return &ErrorIndication{
		// The maximum AMF-UE-NGAP-ID exercises the 40-bit width end to end.
		AMFUENGAPID: Ptr(AMFUENGAPID(1099511627775)),
		RANUENGAPID: Ptr(RANUENGAPID(9)),
		Cause:       Ptr(Cause{Group: CauseGroupProtocol, Value: CauseProtocolAbstractSyntaxErrorReject}),
	}
}

func goldErrorIndicationFull() *ErrorIndication {
	m := goldErrorIndication()
	m.CriticalityDiagnostics = &CriticalityDiagnostics{
		ProcedureCode:        Ptr(ProcNGSetup),
		TriggeringMessage:    Ptr(TriggeringInitiatingMessage),
		ProcedureCriticality: Ptr(CriticalityReject),
		IEsCriticalityDiagnostics: []CriticalityDiagnosticsIEItem{{
			IECriticality: CriticalityReject,
			IEID:          idGlobalRANNodeID,
			TypeOfError:   TypeOfErrorMissing,
		}},
	}

	return m
}

func TestErrorIndicationGoldenEncode(t *testing.T) {
	tests := []struct {
		name    string
		marshal func() ([]byte, error)
		want    string
	}{
		{"cause only", goldErrorIndication().Marshal, goldenErrorIndication},
		{"with criticality diagnostics", goldErrorIndicationFull().Marshal, goldenErrorIndicationFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.marshal()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Fatalf("encode mismatch:\n  got  %x\n  want %x", got, want)
			}
		})
	}
}

func TestErrorIndicationGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenErrorIndicationFull))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcErrorIndication {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	// The procedure is ignore criticality, unlike NG Setup's reject.
	if im.Criticality != CriticalityIgnore {
		t.Errorf("criticality = %v, want ignore", im.Criticality)
	}

	msg, err := ParseErrorIndication(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := goldErrorIndicationFull()

	if deref(msg.AMFUENGAPID) != deref(want.AMFUENGAPID) || deref(msg.RANUENGAPID) != deref(want.RANUENGAPID) {
		t.Errorf("UE ids = %v/%v", msg.AMFUENGAPID, msg.RANUENGAPID)
	}

	if deref(msg.Cause) != deref(want.Cause) {
		t.Errorf("cause = %v, want %v", deref(msg.Cause), deref(want.Cause))
	}

	cd := msg.CriticalityDiagnostics
	if cd == nil || deref(cd.ProcedureCode) != ProcNGSetup ||
		deref(cd.ProcedureCriticality) != CriticalityReject ||
		len(cd.IEsCriticalityDiagnostics) != 1 ||
		cd.IEsCriticalityDiagnostics[0].IEID != idGlobalRANNodeID {
		t.Errorf("criticality diagnostics = %+v", cd)
	}

	// The 5G-S-TMSI was not sent, so it must stay absent.
	if msg.FiveGSTMSI != nil {
		t.Errorf("FiveGSTMSI = %+v, want nil", msg.FiveGSTMSI)
	}
}

// Every IE is optional, so an ERROR INDICATION carrying nothing is still a
// valid message the receiver must accept (TS 38.413 §9.2.6.5).
func TestErrorIndicationEmpty(t *testing.T) {
	b, err := (&ErrorIndication{}).Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseErrorIndication(pdu.value())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.AMFUENGAPID != nil || msg.RANUENGAPID != nil || msg.Cause != nil ||
		msg.CriticalityDiagnostics != nil || msg.FiveGSTMSI != nil {
		t.Errorf("empty message decoded to %+v", msg)
	}

	if !msg.Diagnostics().Empty() {
		t.Errorf("diagnostics = %+v, want empty: nothing is required here", msg.Diagnostics())
	}
}

// The 5G-S-TMSI is the one IE the reference encoder cannot express, so it gets
// its own round trip. Its two bit-string fields are the same widths a GUAMI
// carries, which is what lets a peer find the AMF that allocated the identity.
func TestErrorIndicationFiveGSTMSI(t *testing.T) {
	in := goldErrorIndication()
	in.FiveGSTMSI = &FiveGSTMSI{AMFSetID: 0x3ff, AMFPointer: 0x3f, FiveGTMSI: 0xdeadbeef}

	b, err := in.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseErrorIndication(pdu.value())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.FiveGSTMSI == nil || *out.FiveGSTMSI != *in.FiveGSTMSI {
		t.Fatalf("FiveGSTMSI = %+v, want %+v", out.FiveGSTMSI, in.FiveGSTMSI)
	}
}

// A value wider than its bit string is refused rather than truncated.
func TestFiveGSTMSIRejectsOversizedFields(t *testing.T) {
	for _, in := range []FiveGSTMSI{
		{AMFSetID: 0x400},  // 11 bits into a 10-bit field
		{AMFPointer: 0x40}, // 7 bits into a 6-bit field
	} {
		w := per.NewWriter()
		if err := in.MarshalPER(w, per.Aligned); err == nil {
			t.Errorf("encode accepted %+v", in)
		}
	}
}
