// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "testing"

func TestTrackingAreaUpdateRequestParse(t *testing.T) {
	// octet0: SHT plain | PD EMM; octet1: message type; octet2: EPS update type
	// (active flag 0x08 | type 3 "periodic") | NAS key set id (1) in the high half.
	b := []byte{0x07, byte(MsgTrackingAreaUpdateRequest), 0x1b}

	req, err := ParseTrackingAreaUpdateRequest(b)
	if err != nil {
		t.Fatal(err)
	}

	if req.EPSUpdateType != 3 {
		t.Fatalf("EPSUpdateType = %d, want 3", req.EPSUpdateType)
	}

	if !req.ActiveFlag {
		t.Fatal("ActiveFlag = false, want true")
	}

	if req.NASKeySetID != 1 {
		t.Fatalf("NASKeySetID = %d, want 1", req.NASKeySetID)
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
	status := uint16(1<<5 | 1<<6)

	in := &TrackingAreaUpdateRequest{
		EPSUpdateType:          EPSUpdateTypeTA,
		ActiveFlag:             true,
		NASKeySetID:            1,
		EPSBearerContextStatus: &status,
	}

	wire, err := in.Marshal()
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
	preceded = append(preceded, 0x52, 1, 2, 3, 4, 5)          // Last visited TAI (TV3, 5)
	preceded = append(preceded, 0x58, 0x03, 0xe0, 0xe0, 0x00) // UE network capability (TLV, 3)
	preceded = append(preceded, epsBearerContextStatusIEI, 0x02, byte(status), byte(status>>8))

	out2, err := ParseTrackingAreaUpdateRequest(preceded)
	if err != nil {
		t.Fatal(err)
	}

	if out2.EPSBearerContextStatus == nil || *out2.EPSBearerContextStatus != status {
		t.Fatalf("bearer status behind other IEs = %v, want %#x", out2.EPSBearerContextStatus, status)
	}
}

func TestTrackingAreaUpdateAcceptRoundtrip(t *testing.T) {
	b, err := (&TrackingAreaUpdateAccept{EPSUpdateResult: EPSUpdateResultTA}).Marshal()
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
	taiList := []byte{0x01, 0x00, 0xf1, 0x10, 0x00, 0x01} // representative TAI list value

	b, err := (&TrackingAreaUpdateAccept{EPSUpdateResult: EPSUpdateResultTA, TAIList: taiList}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseTrackingAreaUpdateAccept(b)
	if err != nil {
		t.Fatal(err)
	}

	if string(parsed.TAIList) != string(taiList) {
		t.Fatalf("TAIList = %x, want %x", parsed.TAIList, taiList)
	}
}

func TestTrackingAreaUpdateAcceptEMMCause(t *testing.T) {
	taiList := []byte{0x01, 0x00, 0xf1, 0x10, 0x00, 0x01}
	cause := uint8(18) // CS domain not available

	b, err := (&TrackingAreaUpdateAccept{EPSUpdateResult: EPSUpdateResultTA, TAIList: taiList, EMMCause: &cause}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseTrackingAreaUpdateAccept(b)
	if err != nil {
		t.Fatal(err)
	}

	if string(parsed.TAIList) != string(taiList) {
		t.Fatalf("TAIList = %x, want %x", parsed.TAIList, taiList)
	}

	if parsed.EMMCause == nil || *parsed.EMMCause != cause {
		t.Fatalf("EMMCause = %v, want %d", parsed.EMMCause, cause)
	}
}

func TestTrackingAreaUpdateAcceptGUTI(t *testing.T) {
	taiList := []byte{0x01, 0x00, 0xf1, 0x10, 0x00, 0x01}
	cause := uint8(18)
	guti := &EPSMobileIdentity{Type: IdentityGUTI, MCC: "999", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 0x01020304}

	b, err := (&TrackingAreaUpdateAccept{
		EPSUpdateResult: EPSUpdateResultTA,
		GUTI:            guti,
		TAIList:         taiList,
		EMMCause:        &cause,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseTrackingAreaUpdateAccept(b)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.GUTI == nil || *parsed.GUTI != *guti {
		t.Fatalf("GUTI = %+v, want %+v", parsed.GUTI, guti)
	}

	if string(parsed.TAIList) != string(taiList) {
		t.Fatalf("TAIList = %x, want %x", parsed.TAIList, taiList)
	}

	if parsed.EMMCause == nil || *parsed.EMMCause != cause {
		t.Fatalf("EMMCause = %v, want %d", parsed.EMMCause, cause)
	}
}

// TestTrackingAreaUpdateAcceptBearerContextStatus confirms the EPS bearer context
// status IE round-trips in the accept and is decoded behind the GUTI and TAI list
// it follows in the canonical order (TS 24.301 §8.2.26).
func TestTrackingAreaUpdateAcceptBearerContextStatus(t *testing.T) {
	status := uint16(1<<5 | 1<<7)
	cause := uint8(18)

	in := &TrackingAreaUpdateAccept{
		EPSUpdateResult:        EPSUpdateResultTA,
		GUTI:                   &EPSMobileIdentity{Type: IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 0x01020304},
		TAIList:                []byte{0x01, 0x00, 0xf1, 0x10, 0x00, 0x01},
		EPSBearerContextStatus: &status,
		EMMCause:               &cause,
	}

	wire, err := in.Marshal()
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

	if out.GUTI == nil || *out.GUTI != *in.GUTI {
		t.Fatalf("GUTI = %+v, want %+v", out.GUTI, in.GUTI)
	}

	if out.EMMCause == nil || *out.EMMCause != cause {
		t.Fatalf("EMMCause = %v, want %d", out.EMMCause, cause)
	}
}

func TestTrackingAreaUpdateCompleteRoundtrip(t *testing.T) {
	b, err := (&TrackingAreaUpdateComplete{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseTrackingAreaUpdateComplete(b); err != nil {
		t.Fatalf("parse: %v", err)
	}
}

func TestTrackingAreaUpdateRejectMarshal(t *testing.T) {
	b, err := (&TrackingAreaUpdateReject{Cause: 9}).Marshal()
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
