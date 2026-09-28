// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestAttachAcceptSMSOnlyWire(t *testing.T) {
	b := mustHex("07420221060000f110000100040201d0111300f110fffe2305f401020304f2")

	m, err := ParseAttachAccept(b)
	if err != nil {
		t.Fatal(err)
	}

	if m.EPSAttachResult != AttachResultCombined {
		t.Fatalf("attach result = %s, want combined", m.EPSAttachResult)
	}

	wantLAI := nas.LAI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, LAC: 0xfffe}
	if m.LAI == nil || *m.LAI != wantLAI {
		t.Fatalf("LAI = %v, want %v", m.LAI, wantLAI)
	}

	if m.MSIdentity == nil || m.MSIdentity.TMSI == nil || *m.MSIdentity.TMSI != [4]byte{1, 2, 3, 4} {
		t.Fatalf("MS identity = %v, want TMSI 01020304", m.MSIdentity)
	}

	if m.AdditionalUpdateResult == nil || *m.AdditionalUpdateResult != AdditionalUpdateResultSMSOnly {
		t.Fatalf("additional update result = %v, want SMS only", m.AdditionalUpdateResult)
	}

	if m.SMSServicesStatus != nil || len(m.Unrecognized) != 0 {
		t.Fatalf("SMS services status = %v, unrecognized = %v, want neither", m.SMSServicesStatus, m.Unrecognized)
	}

	out, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, b) {
		t.Fatalf("re-encode = % x, want % x", out, b)
	}
}

func TestTrackingAreaUpdateAcceptSMSServicesStatus(t *testing.T) {
	status := SMSServicesNotAvailableInPLMN
	m := &TrackingAreaUpdateAccept{EPSUpdateResult: EPSUpdateResultTA, SMSServicesStatus: &status}

	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if want := mustHex("074900e1"); !bytes.Equal(b, want) {
		t.Fatalf("TAU ACCEPT = % x, want % x", b, want)
	}

	got, err := ParseTrackingAreaUpdateAccept(b)
	if err != nil {
		t.Fatal(err)
	}

	if got.SMSServicesStatus == nil || *got.SMSServicesStatus != status {
		t.Fatalf("SMS services status = %v, want %s", got.SMSServicesStatus, status)
	}
}

func TestTrackingAreaUpdateRequestAdditionalUpdateType(t *testing.T) {
	m := &TrackingAreaUpdateRequest{
		EPSUpdateType:        EPSUpdateTypeCombinedTALA,
		NASKeySetIdentifier:  nas.NoKeySet,
		OldGUTI:              GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0, 0, 0, 1}}),
		AdditionalUpdateType: &AdditionalUpdateType{AUTV: true},
	}

	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if b[len(b)-1] != 0xF1 {
		t.Fatalf("last octet = %#x, want the SMS-only additional update type 0xF1", b[len(b)-1])
	}

	got, err := ParseTrackingAreaUpdateRequest(b)
	if err != nil {
		t.Fatal(err)
	}

	if got.AdditionalUpdateType == nil || !got.AdditionalUpdateType.AUTV {
		t.Fatalf("additional update type = %v, want SMS only", got.AdditionalUpdateType)
	}
}

func TestAdditionalUpdateResultIgnoresSpareBits(t *testing.T) {
	m, err := ParseTrackingAreaUpdateAccept(mustHex("074900fe"))
	if err != nil {
		t.Fatal(err)
	}

	if m.AdditionalUpdateResult == nil || *m.AdditionalUpdateResult != AdditionalUpdateResultSMSOnly {
		t.Fatalf("additional update result = %v, want SMS only", m.AdditionalUpdateResult)
	}
}
