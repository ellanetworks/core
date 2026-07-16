// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"encoding/hex"
	"errors"
	"slices"
	"testing"

	"github.com/ellanetworks/core/s1ap/aper"
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

	if req.ENBName != "JLT-621" {
		t.Fatalf("eNBname = %q", req.ENBName)
	}

	if len(req.SupportedTAs) != 1 || req.SupportedTAs[0].TAC != 0x3039 {
		t.Fatalf("SupportedTAs = %+v", req.SupportedTAs)
	}

	if len(req.SupportedTAs[0].BroadcastPLMNs) != 1 ||
		req.SupportedTAs[0].BroadcastPLMNs[0] != (PLMNIdentity{0x00, 0xf1, 0x10}) {
		t.Fatalf("broadcastPLMNs = %+v", req.SupportedTAs[0].BroadcastPLMNs)
	}

	if req.DefaultPagingDRX != PagingDRXv32 {
		t.Fatalf("pagingDRX = %d", req.DefaultPagingDRX)
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
		GlobalENBID:      GlobalENBID{PLMNIdentity{0x00, 0xf1, 0x10}, ENBID{Kind: ENBIDMacro, Value: 0x0abcd}},
		ENBName:          "eNB-1",
		SupportedTAs:     SupportedTAs{{TAC: 0x0001, BroadcastPLMNs: BPLMNs{{0x00, 0xf1, 0x10}}}},
		DefaultPagingDRX: PagingDRXv128,
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

	if out.GlobalENBID != in.GlobalENBID || out.ENBName != in.ENBName ||
		out.DefaultPagingDRX != in.DefaultPagingDRX || len(out.SupportedTAs) != 1 ||
		out.SupportedTAs[0].TAC != in.SupportedTAs[0].TAC {
		t.Fatalf("round-trip mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

// encodePartialS1Setup builds an S1SetupRequest open-type value containing only the
// chosen IEs, for exercising missing-mandatory-IE handling (TS 36.413 §9.1.8.4).
func encodePartialS1Setup(t *testing.T, globalENBID, supportedTAs, pagingDRX bool) []byte {
	t.Helper()

	var w aper.Writer
	w.WriteSequencePreamble(true, false, nil)

	var fields []ieField

	if globalENBID {
		g := GlobalENBID{PLMNIdentity{0x00, 0xf1, 0x10}, ENBID{Kind: ENBIDMacro, Value: 0x0abcd}}
		fields = append(fields, ieField{id: idGlobalENBID, crit: CriticalityReject, enc: g.encode})
	}

	if supportedTAs {
		s := SupportedTAs{{TAC: 0x0001, BroadcastPLMNs: BPLMNs{{0x00, 0xf1, 0x10}}}}
		fields = append(fields, ieField{id: idSupportedTAs, crit: CriticalityReject, enc: s.encode})
	}

	if pagingDRX {
		d := PagingDRXv128
		fields = append(fields, ieField{id: idDefaultPagingDRX, crit: CriticalityIgnore, enc: d.encode})
	}

	if err := encodeIEContainer(&w, fields); err != nil {
		t.Fatalf("encode: %v", err)
	}

	return w.Bytes()
}

// TS 36.413 §10.3.5: an S1 Setup Request missing a reject-criticality mandatory IE
// (Global eNB ID or Supported TAs) is reported as a MissingMandatoryIEsError naming
// the absent IE; a missing Default Paging DRX (ignore-criticality) is tolerated.
func TestParseS1SetupRequestMissingMandatoryIE(t *testing.T) {
	tests := []struct {
		name        string
		globalENBID bool
		supportedTA bool
		pagingDRX   bool
		wantMissing []ProtocolIEID
	}{
		{"missing GlobalENBID", false, true, true, []ProtocolIEID{idGlobalENBID}},
		{"missing SupportedTAs", true, false, true, []ProtocolIEID{idSupportedTAs}},
		{"missing both reject IEs", false, false, true, []ProtocolIEID{idGlobalENBID, idSupportedTAs}},
		{"missing only PagingDRX is tolerated", true, true, false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := encodePartialS1Setup(t, tt.globalENBID, tt.supportedTA, tt.pagingDRX)

			_, err := ParseS1SetupRequest(value)

			if tt.wantMissing == nil {
				if err != nil {
					t.Fatalf("parse: unexpected error %v", err)
				}

				return
			}

			var missing *MissingMandatoryIEsError
			if !errors.As(err, &missing) {
				t.Fatalf("error = %v, want *MissingMandatoryIEsError", err)
			}

			if missing.Procedure != ProcS1Setup {
				t.Errorf("procedure = %d, want ProcS1Setup", missing.Procedure)
			}

			if !slices.Equal(missing.IEs, tt.wantMissing) {
				t.Errorf("missing IEs = %v, want %v", missing.IEs, tt.wantMissing)
			}
		})
	}
}
