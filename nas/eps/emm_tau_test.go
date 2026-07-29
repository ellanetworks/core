// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestTrackingAreaUpdateRequestParse(t *testing.T) {
	// octet0: SHT plain | PD EMM; octet1: message type; octet2: EPS update type
	// (active flag 0x08 | type 3 "periodic") | NAS key set id (1) in the high half;
	// then the mandatory Old GUTI (LV, 11-octet EPS mobile identity).
	b := []byte{0x07, byte(MsgTrackingAreaUpdateRequest), 0x1b}
	b = append(b, 0x0b, 0xf6, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01)

	req, err := ParseTrackingAreaUpdateRequest(b)
	if err != nil {
		t.Fatal(err)
	}

	if req.OldGUTI.GUTI == nil {
		t.Fatalf("OldGUTI = %+v, want a GUTI", req.OldGUTI)
	}

	if req.EPSUpdateType != 3 {
		t.Fatalf("EPSUpdateType = %d, want 3", req.EPSUpdateType)
	}

	if !req.ActiveFlag {
		t.Fatal("ActiveFlag = false, want true")
	}

	if req.NASKeySetIdentifier != (nas.KeySetIdentifier{Value: 1}) {
		t.Fatalf("NAS KSI = %v, want native 1", req.NASKeySetIdentifier)
	}

	if req.EPSBearerContextStatus != nil {
		t.Fatalf("EPSBearerContextStatus = %#x, want nil (IE absent)", *req.EPSBearerContextStatus)
	}
}

// TestTrackingAreaUpdateRequestBearerContextStatus confirms the optional EPS
// bearer context status IE round-trips and, crucially, is reached even when it
// sits behind other optional IEs in the variable part (the walker must delimit
// and skip them) — EBI 5 and EBI 6 active here.
func TestTrackingAreaUpdateRequestBearerContextStatus(t *testing.T) {
	status := nas.EPSBearerContextStatus{}
	status.Active[5], status.Active[6] = true, true

	in := &TrackingAreaUpdateRequest{
		EPSUpdateType:          EPSUpdateTypeTA,
		ActiveFlag:             true,
		NASKeySetIdentifier:    nas.KeySetIdentifier{Value: 1},
		OldGUTI:                GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 0, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01}}),
		EPSBearerContextStatus: &status,
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseTrackingAreaUpdateRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.EPSBearerContextStatus == nil || *out.EPSBearerContextStatus != status {
		t.Fatalf("EPSBearerContextStatus = %v, want %#x", out.EPSBearerContextStatus, status)
	}

	// Same status, but now preceded by a TV3 (Last visited TAI, 0x52) and a TLV
	// (UE network capability, 0x58) the walker must skip to reach 0x57.
	preceded := []byte{0x07, byte(MsgTrackingAreaUpdateRequest), 0x1b}
	preceded = append(preceded, 0x0b, 0xf2, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01) // Old GUTI (LV, 11)
	preceded = append(preceded, 0x52, 1, 2, 3, 4, 5)                                                    // Last visited TAI (TV3, 5)
	preceded = append(preceded, 0x58, 0x03, 0xe0, 0xe0, 0x00)                                           // UE network capability (TLV, 3)

	statusRaw, err := status.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	preceded = append(preceded, ieiEPSBearerContextStatus, 0x02)
	preceded = append(preceded, statusRaw...)

	out2, err := ParseTrackingAreaUpdateRequest(preceded)
	if err != nil {
		t.Fatal(err)
	}

	if out2.EPSBearerContextStatus == nil || *out2.EPSBearerContextStatus != status {
		t.Fatalf("bearer status behind other IEs = %v, want %v", out2.EPSBearerContextStatus, status)
	}
}

func TestTrackingAreaUpdateAcceptRoundtrip(t *testing.T) {
	b, err := (&TrackingAreaUpdateAccept{EPSUpdateResult: EPSUpdateResultTA}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	mt, err := PeekMessageType(b)
	if err != nil {
		t.Fatal(err)
	}

	if mt != MsgTrackingAreaUpdateAccept {
		t.Fatalf("message type = %#x, want %#x", mt, MsgTrackingAreaUpdateAccept)
	}

	parsed, err := ParseTrackingAreaUpdateAccept(b)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.EPSUpdateResult != EPSUpdateResultTA {
		t.Fatalf("EPSUpdateResult = %d, want %d", parsed.EPSUpdateResult, EPSUpdateResultTA)
	}
}

func TestTrackingAreaUpdateAcceptTAIList(t *testing.T) {
	b, err := (&TrackingAreaUpdateAccept{EPSUpdateResult: EPSUpdateResultTA, TAIList: ptr(testTAIList())}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseTrackingAreaUpdateAccept(b)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.TAIList == nil || !reflect.DeepEqual(*parsed.TAIList, testTAIList()) {
		t.Fatalf("TAIList = %+v, want %+v", parsed.TAIList, testTAIList())
	}
}

func TestTrackingAreaUpdateAcceptEMMCause(t *testing.T) {
	cause := uint8(18) // CS domain not available

	b, err := (&TrackingAreaUpdateAccept{EPSUpdateResult: EPSUpdateResultTA, TAIList: ptr(testTAIList()), Cause: ptr(EMMCause(cause))}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseTrackingAreaUpdateAccept(b)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.TAIList == nil || !reflect.DeepEqual(*parsed.TAIList, testTAIList()) {
		t.Fatalf("TAIList = %+v, want %+v", parsed.TAIList, testTAIList())
	}

	if parsed.Cause == nil || *parsed.Cause != EMMCause(cause) {
		t.Fatalf("EMMCause = %v, want %d", parsed.Cause, cause)
	}
}

func TestTrackingAreaUpdateAcceptGUTI(t *testing.T) {
	cause := uint8(18)
	guti := GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "999", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x01, 0x02, 0x03, 0x04}})

	b, err := (&TrackingAreaUpdateAccept{
		EPSUpdateResult: EPSUpdateResultTA,
		GUTI:            &guti,
		TAIList:         ptr(testTAIList()),
		Cause:           ptr(EMMCause(cause)),
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseTrackingAreaUpdateAccept(b)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.GUTI == nil {
		t.Fatal("GUTI absent")
	}

	if !reflect.DeepEqual(*parsed.GUTI, guti) {
		t.Fatalf("GUTI = %+v, want %+v", parsed.GUTI, guti)
	}

	if parsed.TAIList == nil || !reflect.DeepEqual(*parsed.TAIList, testTAIList()) {
		t.Fatalf("TAIList = %+v, want %+v", parsed.TAIList, testTAIList())
	}

	if parsed.Cause == nil || *parsed.Cause != EMMCause(cause) {
		t.Fatalf("EMMCause = %v, want %d", parsed.Cause, cause)
	}
}

// TestTrackingAreaUpdateAcceptBearerContextStatus confirms the EPS bearer context
// status IE round-trips in the accept and is decoded behind the GUTI and TAI list
// it follows in the canonical order (TS 24.301).
func TestTrackingAreaUpdateAcceptBearerContextStatus(t *testing.T) {
	status := nas.EPSBearerContextStatus{}
	status.Active[5], status.Active[7] = true, true
	cause := uint8(18)

	guti := GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x01, 0x02, 0x03, 0x04}})

	in := &TrackingAreaUpdateAccept{
		EPSUpdateResult:        EPSUpdateResultTA,
		GUTI:                   &guti,
		TAIList:                ptr(testTAIList()),
		EPSBearerContextStatus: &status,
		Cause:                  ptr(EMMCause(cause)),
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseTrackingAreaUpdateAccept(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.EPSBearerContextStatus == nil || *out.EPSBearerContextStatus != status {
		t.Fatalf("EPSBearerContextStatus = %v, want %#x", out.EPSBearerContextStatus, status)
	}

	if out.GUTI == nil {
		t.Fatal("GUTI absent")
	}

	if !reflect.DeepEqual(*out.GUTI, guti) {
		t.Fatalf("GUTI = %+v, want %+v", out.GUTI, guti)
	}

	if out.Cause == nil || *out.Cause != EMMCause(cause) {
		t.Fatalf("EMMCause = %v, want %d", out.Cause, cause)
	}
}

func TestTrackingAreaUpdateCompleteRoundtrip(t *testing.T) {
	b, err := (&TrackingAreaUpdateComplete{}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseTrackingAreaUpdateComplete(b); err != nil {
		t.Fatalf("parse: %v", err)
	}
}

func TestTrackingAreaUpdateRejectMarshal(t *testing.T) {
	b, err := (&TrackingAreaUpdateReject{Cause: 9}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	mt, err := PeekMessageType(b)
	if err != nil {
		t.Fatal(err)
	}

	if mt != MsgTrackingAreaUpdateReject {
		t.Fatalf("message type = %#x, want %#x", mt, MsgTrackingAreaUpdateReject)
	}

	parsed, err := ParseTrackingAreaUpdateReject(b)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Cause != 9 {
		t.Fatalf("cause = %d, want 9", parsed.Cause)
	}
}
