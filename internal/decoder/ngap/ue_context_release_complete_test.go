// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_UEContextReleaseComplete(t *testing.T) {
	raw, err := decodeB64(ueContextReleaseCompleteCapture)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "SuccessfulOutcome" {
		t.Errorf("expected PDUType=SuccessfulOutcome, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcUEContextRelease) {
		t.Errorf("expected ProcedureCode=UEContextRelease, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcUEContextRelease) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcUEContextRelease)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDAMFUENGAPID) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDAMFUENGAPID)
	}

	if item0.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item0.Criticality)
	}

	amfUeNgapID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMF-UE-NGAP-ID type=uint64, got %T", item0.Value)
	}

	if amfUeNgapID != 146 {
		t.Errorf("expected AMF-UE-NGAP-ID=146, got %d", amfUeNgapID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Value != int64(lib.IDRANUENGAPID) {
		t.Errorf("IE id = %d, want %d", item1.ID.Value, lib.IDRANUENGAPID)
	}

	if item1.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item1.Criticality)
	}

	ranUeNgapID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RAN-UE-NGAP-ID type=uint64, got %T", item1.Value)
	}

	if ranUeNgapID != 159 {
		t.Errorf("expected RAN-UE-NGAP-ID=159, got %d", ranUeNgapID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDUserLocationInformation) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDUserLocationInformation)
	}

	if item2.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item2.Criticality)
	}

	userLocationInfo, ok := item2.Value.(UserLocationInformation)
	if !ok {
		t.Fatalf("expected UserLocationInformation value type=UserLocationInformation, got %T", item2.Value)
	}

	if userLocationInfo.NR == nil {
		t.Errorf("expected UserLocationInformation.NR to be non-nil")
	}

	if userLocationInfo.NR.TAI.PLMNID.Mcc != "001" {
		t.Errorf("expected TAI.PLMNID.MCC=001, got %s", userLocationInfo.NR.TAI.PLMNID.Mcc)
	}

	if userLocationInfo.NR.TAI.PLMNID.Mnc != "01" {
		t.Errorf("expected TAI.PLMNID.MNC=01, got %s", userLocationInfo.NR.TAI.PLMNID.Mnc)
	}

	if userLocationInfo.NR.TAI.TAC != "000001" {
		t.Errorf("expected TAI.TAC=000001, got %v", userLocationInfo.NR.TAI.TAC)
	}

	if userLocationInfo.NR.NRCGI.PLMNID.Mcc != "001" {
		t.Errorf("expected NR-CGI.PLMNID.MCC=001, got %s", userLocationInfo.NR.NRCGI.PLMNID.Mcc)
	}

	if userLocationInfo.NR.NRCGI.PLMNID.Mnc != "01" {
		t.Errorf("expected NR-CGI.PLMNID.MNC=01, got %s", userLocationInfo.NR.NRCGI.PLMNID.Mnc)
	}

	if userLocationInfo.NR.NRCGI.NRCellIdentity != "001234501" {
		t.Errorf("expected NR-CGI.NRCellIdentity=001234501, got %v", userLocationInfo.NR.NRCGI.NRCellIdentity)
	}

	if userLocationInfo.NR.TimeStamp != nil {
		t.Errorf("expected NR.TimeStamp to be nil, got %v", *userLocationInfo.NR.TimeStamp)
	}

	if userLocationInfo.EUTRA != nil {
		t.Errorf("expected UserLocationInformation.EUTRA to be nil, got %v", userLocationInfo.EUTRA)
	}

	if userLocationInfo.N3IWF != nil {
		t.Errorf("expected UserLocationInformation.N3IWF to be nil, got %v", userLocationInfo.N3IWF)
	}

	if userLocationInfo.Error != "" {
		t.Errorf("expected UserLocationInformation.Error to be empty, got %s", userLocationInfo.Error)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Value != int64(lib.IDPDUSessionResourceListCxtRelCpl) {
		t.Errorf("IE id = %d, want %d", item3.ID.Value, lib.IDPDUSessionResourceListCxtRelCpl)
	}

	if item3.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item3.Criticality)
	}

	pduSessionList, ok := item3.Value.([]PDUSessionResourceItemCxtRelCpl)
	if !ok {
		t.Fatalf("expected PDUSessionResourceListCxtRelCpl type=[]PDUSessionResourceItemCxtRelCpl, got %T", item3.Value)
	}

	if len(pduSessionList) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceItemCxtRelCpl, got %d", len(pduSessionList))
	}

	if pduSessionList[0].PDUSessionID != 1 {
		t.Errorf("expected PDUSessionID=1, got %d", pduSessionList[0].PDUSessionID)
	}
}

// A UEContextReleaseComplete captured on the 001/01 test PLMN.
const ueContextReleaseCompleteCapture = "ICkAKQAABAAKQAIAkgBVQAIAnwB5QA9AAPEQABI0UBAA8RAAAAEAPAADAAAB"
