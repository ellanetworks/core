// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"encoding/hex"
	"errors"
	"slices"
	"testing"

	"github.com/ellanetworks/core/per"
)

const goldenS1SetupRequest = "0011002d000004003b00090000f1104054f64010003c400903004a4c542d36323100400007000c0e4000f1100089400100"

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex: %v", err)
	}

	return b
}

func TestS1SetupRequestGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenS1SetupRequest))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcS1Setup {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	req, err := ParseS1SetupRequest(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if req.GlobalENBID.PLMNIdentity != (PLMNIdentity{0x00, 0xf1, 0x10}) {
		t.Fatalf("PLMN = % x", req.GlobalENBID.PLMNIdentity)
	}

	if req.GlobalENBID.ENBID.Kind != ENBIDHome || req.GlobalENBID.ENBID.Value != 0x54f6401 {
		t.Fatalf("ENB-ID = %+v", req.GlobalENBID.ENBID)
	}

	if deref(req.ENBName) != "JLT-621" {
		t.Fatalf("eNBname = %q", deref(req.ENBName))
	}

	if len(req.SupportedTAs) != 1 || req.SupportedTAs[0].TAC != 0x3039 {
		t.Fatalf("SupportedTAs = %+v", req.SupportedTAs)
	}

	if len(req.SupportedTAs[0].BroadcastPLMNs) != 1 ||
		req.SupportedTAs[0].BroadcastPLMNs[0] != (PLMNIdentity{0x00, 0xf1, 0x10}) {
		t.Fatalf("broadcastPLMNs = %+v", req.SupportedTAs[0].BroadcastPLMNs)
	}

	if deref(req.DefaultPagingDRX) != PagingDRXv32 {
		t.Fatalf("pagingDRX = %d", deref(req.DefaultPagingDRX))
	}
}

func TestS1SetupRequestGoldenReencode(t *testing.T) {
	want := mustHex(t, goldenS1SetupRequest)

	pdu, err := Unmarshal(want)
	if err != nil {
		t.Fatal(err)
	}

	req, err := ParseS1SetupRequest(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	got, err := req.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("re-encode mismatch:\n  got  % x\n  want % x", got, want)
	}
}

func TestS1SetupRequestRoundTrip(t *testing.T) {
	in := &S1SetupRequest{
		GlobalENBID:      GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{Kind: ENBIDMacro, Value: 0x0abcd}},
		ENBName:          Ptr("eNB-1"),
		SupportedTAs:     SupportedTAs{{TAC: 0x0001, BroadcastPLMNs: BPLMNs{{0x00, 0xf1, 0x10}}}},
		DefaultPagingDRX: Ptr(PagingDRXv128),
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseS1SetupRequest(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.GlobalENBID != in.GlobalENBID || deref(out.ENBName) != deref(in.ENBName) ||
		deref(out.DefaultPagingDRX) != deref(in.DefaultPagingDRX) ||
		len(out.SupportedTAs) != 1 ||
		out.SupportedTAs[0].TAC != in.SupportedTAs[0].TAC {
		t.Fatalf("round-trip mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

// encodePartialS1Setup omits IEs to exercise §9.1.8.4 mandatory-IE handling.
func encodePartialS1Setup(t *testing.T, globalENBID, supportedTAs, pagingDRX bool) []byte {
	t.Helper()

	w := per.NewWriter()
	w.WriteBit(false)

	var fields []ieField

	if globalENBID {
		g := GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{Kind: ENBIDMacro, Value: 0x0abcd}}
		fields = append(fields, ieField{id: idGlobalENBID, crit: CriticalityReject, val: &g})
	}

	if supportedTAs {
		s := SupportedTAs{{TAC: 0x0001, BroadcastPLMNs: BPLMNs{{0x00, 0xf1, 0x10}}}}
		fields = append(fields, ieField{id: idSupportedTAs, crit: CriticalityReject, val: s})
	}

	if pagingDRX {
		d := PagingDRXv128
		fields = append(fields, ieField{id: idDefaultPagingDRX, crit: CriticalityIgnore, val: d})
	}

	if err := encodeIEContainer(w, per.Aligned, fields); err != nil {
		t.Fatalf("encode: %v", err)
	}

	return perBytes(w)
}

func TestParseS1SetupRequestMissingMandatoryIE(t *testing.T) {
	tests := []struct {
		name        string
		globalENBID bool
		supportedTA bool
		pagingDRX   bool
		// wantReject lists the absent reject-criticality IEs; nil means the
		// message is still delivered.
		wantReject   []ProtocolIEID
		wantReported []ProtocolIEID
	}{
		{"missing GlobalENBID", false, true, true, []ProtocolIEID{idGlobalENBID}, nil},
		{"missing SupportedTAs", true, false, true, []ProtocolIEID{idSupportedTAs}, nil},
		{"missing both reject IEs", false, false, true, []ProtocolIEID{idGlobalENBID, idSupportedTAs}, nil},
		// Default Paging DRX is mandatory-ignore (§9.1.8.4), so its absence is
		// reported rather than fatal.
		{"missing only PagingDRX", true, true, false, nil, []ProtocolIEID{idDefaultPagingDRX}},
		{"missing reject and ignore IEs", false, true, false, []ProtocolIEID{idGlobalENBID}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := encodePartialS1Setup(t, tt.globalENBID, tt.supportedTA, tt.pagingDRX)

			out, err := ParseS1SetupRequest(value)

			if tt.wantReject == nil {
				if err != nil {
					t.Fatalf("parse: unexpected error %v", err)
				}

				if got := diagnosticIEIDs(out.Diagnostics().IEs); !slices.Equal(got, tt.wantReported) {
					t.Errorf("diagnostics = %v, want %v", got, tt.wantReported)
				}

				return
			}

			var ase *AbstractSyntaxError
			if !errors.As(err, &ase) {
				t.Fatalf("error = %v, want *AbstractSyntaxError", err)
			}

			if ase.Procedure != ProcS1Setup {
				t.Errorf("procedure = %d, want ProcS1Setup", ase.Procedure)
			}

			want := Cause{Group: CauseGroupProtocol, Value: CauseProtocolAbstractSyntaxErrorReject}
			if ase.Cause != want {
				t.Errorf("cause = %v, want %v", ase.Cause, want)
			}

			if got := diagnosticIEIDs(ase.IEs); !slices.Equal(got, tt.wantReject) {
				t.Errorf("rejected IEs = %v, want %v", got, tt.wantReject)
			}

			for _, ie := range ase.IEs {
				if ie.TypeOfError != TypeOfErrorMissing {
					t.Errorf("IE %v type of error = %v, want missing", ie.IEID, ie.TypeOfError)
				}
			}
		})
	}
}

func diagnosticIEIDs(items []CriticalityDiagnosticsIEItem) []ProtocolIEID {
	ids := make([]ProtocolIEID, len(items))
	for i, ie := range items {
		ids[i] = ie.IEID
	}

	return ids
}
