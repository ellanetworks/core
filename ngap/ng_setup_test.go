// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"encoding/hex"
	"errors"
	"slices"
	"testing"

	"github.com/ellanetworks/core/per"
)

// Golden NG Setup PDUs.
const (
	goldenNGSetupRequest  = "00150038000004001b00080002f839100001020052400a0380656c6c612d676e620066001200000000010002f8390001100801020300100015400140"
	goldenNGSetupResponse = "201500310000040001000a0380656c6c612d616d6600600008000002f83902004300564001ff0050000b0002f83900001008010203"
	goldenNGSetupFailure  = "4015000d000002000f400188006b400130"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex: %v", err)
	}

	return b
}

// goldPLMN is MCC 208 / MNC 93 in TBCD.
func goldPLMN() PLMNIdentity { return PLMNIdentity{0x02, 0xf8, 0x39} }

func goldRequest() *NGSetupRequest {
	return &NGSetupRequest{
		GlobalRANNodeID: GlobalRANNodeID{
			Kind:         RANNodeIDGNB,
			PLMNIdentity: goldPLMN(),
			Value:        0x000102,
			Bits:         24,
		},
		RANNodeName: Ptr("ella-gnb"),
		SupportedTAList: SupportedTAList{{
			TAC: 0x000001,
			BroadcastPLMNList: BroadcastPLMNList{{
				PLMNIdentity: goldPLMN(),
				TAISliceSupportList: SliceSupportList{
					{SNSSAI: SNSSAI{SST: 1, SD: &SD{0x01, 0x02, 0x03}}},
					{SNSSAI: SNSSAI{SST: 2}},
				},
			}},
		}},
		DefaultPagingDRX: Ptr(PagingDRXv128),
	}
}

func goldResponse() *NGSetupResponse {
	return &NGSetupResponse{
		AMFName: "ella-amf",
		ServedGUAMIList: ServedGUAMIList{{
			GUAMI: GUAMI{
				PLMNIdentity: goldPLMN(),
				AMFRegionID:  0x02,
				AMFSetID:     0x001,
				AMFPointer:   0x03,
			},
		}},
		RelativeAMFCapacity: Ptr(uint8(255)),
		PLMNSupportList: PLMNSupportList{{
			PLMNIdentity:     goldPLMN(),
			SliceSupportList: SliceSupportList{{SNSSAI: SNSSAI{SST: 1, SD: &SD{0x01, 0x02, 0x03}}}},
		}},
	}
}

func goldFailure() *NGSetupFailure {
	return &NGSetupFailure{
		Cause:      Ptr(Cause{Group: CauseGroupMisc, Value: CauseMiscUnknownPLMNOrSNPN}),
		TimeToWait: Ptr(TimeToWaitV10s),
	}
}

func TestNGSetupRequestGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenNGSetupRequest))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcNGSetup {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	req, err := ParseNGSetupRequest(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := goldRequest()

	if req.GlobalRANNodeID != want.GlobalRANNodeID {
		t.Errorf("GlobalRANNodeID = %+v, want %+v", req.GlobalRANNodeID, want.GlobalRANNodeID)
	}

	if deref(req.RANNodeName) != "ella-gnb" {
		t.Errorf("RANNodeName = %q", deref(req.RANNodeName))
	}

	if len(req.SupportedTAList) != 1 || req.SupportedTAList[0].TAC != 0x000001 {
		t.Fatalf("SupportedTAList = %+v", req.SupportedTAList)
	}

	slicesGot := req.SupportedTAList[0].BroadcastPLMNList[0].TAISliceSupportList
	if len(slicesGot) != 2 {
		t.Fatalf("slices = %+v", slicesGot)
	}

	if slicesGot[0].SNSSAI.SST != 1 || deref(slicesGot[0].SNSSAI.SD) != (SD{0x01, 0x02, 0x03}) {
		t.Errorf("slice 0 = %+v", slicesGot[0].SNSSAI)
	}

	// The second slice omits SD, which must stay nil rather than read as zero.
	if slicesGot[1].SNSSAI.SST != 2 || slicesGot[1].SNSSAI.SD != nil {
		t.Errorf("slice 1 = %+v", slicesGot[1].SNSSAI)
	}

	if deref(req.DefaultPagingDRX) != PagingDRXv128 {
		t.Errorf("pagingDRX = %d", deref(req.DefaultPagingDRX))
	}
}

// TestNGSetupGoldenEncode is the other half of the differential check: the
// bytes this codec writes must equal the ones the reference implementation
// wrote for the same message.
func TestNGSetupGoldenEncode(t *testing.T) {
	tests := []struct {
		name    string
		marshal func() ([]byte, error)
		want    string
	}{
		{"NGSetupRequest", goldRequest().Marshal, goldenNGSetupRequest},
		{"NGSetupResponse", goldResponse().Marshal, goldenNGSetupResponse},
		{"NGSetupFailure", goldFailure().Marshal, goldenNGSetupFailure},
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

func TestNGSetupRequestRoundTrip(t *testing.T) {
	in := goldRequest()

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseNGSetupRequest(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.GlobalRANNodeID != in.GlobalRANNodeID ||
		deref(out.RANNodeName) != deref(in.RANNodeName) ||
		deref(out.DefaultPagingDRX) != deref(in.DefaultPagingDRX) ||
		len(out.SupportedTAList) != 1 ||
		out.SupportedTAList[0].TAC != in.SupportedTAList[0].TAC {
		t.Fatalf("round-trip mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

// encodePartialNGSetup omits IEs to exercise §9.2.6.1 mandatory-IE handling.
func encodePartialNGSetup(t *testing.T, globalRANNodeID, supportedTAList, pagingDRX bool) []byte {
	t.Helper()

	w := per.NewWriter()
	w.WriteBit(false)

	var fields []ieField

	src := goldRequest()

	if globalRANNodeID {
		fields = append(fields, ieField{id: idGlobalRANNodeID, crit: CriticalityReject, val: &src.GlobalRANNodeID})
	}

	if supportedTAList {
		fields = append(fields, ieField{id: idSupportedTAList, crit: CriticalityReject, val: src.SupportedTAList})
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

func TestParseNGSetupRequestMissingMandatoryIE(t *testing.T) {
	tests := []struct {
		name            string
		globalRANNodeID bool
		supportedTAList bool
		pagingDRX       bool
		// wantReject lists the absent reject-criticality IEs; nil means the
		// message is still delivered.
		wantReject   []ProtocolIEID
		wantReported []ProtocolIEID
	}{
		{"missing GlobalRANNodeID", false, true, true, []ProtocolIEID{idGlobalRANNodeID}, nil},
		{"missing SupportedTAList", true, false, true, []ProtocolIEID{idSupportedTAList}, nil},
		{"missing both reject IEs", false, false, true, []ProtocolIEID{idGlobalRANNodeID, idSupportedTAList}, nil},
		// Default Paging DRX is mandatory-ignore (§9.2.6.1), so its absence is
		// reported and delivered.
		{"missing only PagingDRX", true, true, false, nil, []ProtocolIEID{idDefaultPagingDRX}},
		{"missing reject and ignore IEs", false, true, false, []ProtocolIEID{idGlobalRANNodeID}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := encodePartialNGSetup(t, tt.globalRANNodeID, tt.supportedTAList, tt.pagingDRX)

			out, err := ParseNGSetupRequest(value)

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

			if ase.Procedure != ProcNGSetup {
				t.Errorf("procedure = %d, want ProcNGSetup", ase.Procedure)
			}

			want := Cause{Group: CauseGroupProtocol, Value: CauseProtocolAbstractSyntaxErrorReject}
			if ase.Cause != want {
				t.Errorf("cause = %v, want %v", ase.Cause, want)
			}

			if got := rejectedIEIDs(ase.IEs); !slices.Equal(got, tt.wantReject) {
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

// NG SETUP FAILURE has an unsuccessful outcome to reject with, which is what
// decides between it and an Error Indication (§10.3.5).
func TestNGSetupRejectionHasUnsuccessfulOutcome(t *testing.T) {
	value := encodePartialNGSetup(t, false, true, true)

	_, err := ParseNGSetupRequest(value)

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %v, want *AbstractSyntaxError", err)
	}

	if !ase.HasUnsuccessfulOutcome() {
		t.Error("HasUnsuccessfulOutcome() = false, want true")
	}

	d := ase.OutcomeDiagnostics()
	if d.ProcedureCode != nil || d.TriggeringMessage != nil {
		t.Errorf("outcome diagnostics carry procedure code or triggering message: %+v", d)
	}

	if deref(d.ProcedureCriticality) != CriticalityReject {
		t.Errorf("procedure criticality = %v, want reject", deref(d.ProcedureCriticality))
	}

	e := ase.ErrorIndicationDiagnostics()
	if deref(e.ProcedureCode) != ProcNGSetup || deref(e.TriggeringMessage) != TriggeringInitiatingMessage {
		t.Errorf("error indication diagnostics = %+v", e)
	}
}

func diagnosticIEIDs(items []DiagnosticIE) []ProtocolIEID {
	ids := make([]ProtocolIEID, len(items))
	for i, ie := range items {
		ids[i] = ie.ID
	}

	return ids
}

func rejectedIEIDs(items []CriticalityDiagnosticsIEItem) []ProtocolIEID {
	ids := make([]ProtocolIEID, len(items))
	for i, ie := range items {
		ids[i] = ie.IEID
	}

	return ids
}
