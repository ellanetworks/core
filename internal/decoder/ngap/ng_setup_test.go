// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/utils"
	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_NGSetupRequest(t *testing.T) {
	raw, err := decodeB64(ngSetupRequestCapture)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcNGSetup) {
		t.Errorf("expected ProcedureCode=NGSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcNGSetup) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcNGSetup)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDGlobalRANNodeID) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDGlobalRANNodeID)
	}

	if item0.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item0.Criticality)
	}

	globalRANNodeID, ok := item0.Value.(GlobalRANNodeIDIE)
	if !ok {
		t.Fatalf("expected GlobalRANNodeIDIE, got %T", item0.Value)
	}

	if globalRANNodeID.PLMNIdentity.Mcc != "001" {
		t.Errorf("expected PLMNIdentity.Mcc=001, got %s", globalRANNodeID.PLMNIdentity.Mcc)
	}

	if globalRANNodeID.PLMNIdentity.Mnc != "01" {
		t.Errorf("expected PLMNIdentity.Mnc=01, got %s", globalRANNodeID.PLMNIdentity.Mnc)
	}

	if globalRANNodeID.GlobalGNBID != "00000001" {
		t.Errorf("expected GlobalGNBID=00000001, got %s", globalRANNodeID.GlobalGNBID)
	}

	if globalRANNodeID.GlobalNgENBID != "" {
		t.Errorf("expected empty globalNgENBID, got %s", globalRANNodeID.GlobalNgENBID)
	}

	if globalRANNodeID.GlobalN3IWFID != "" {
		t.Errorf("expected empty GlobalN3IWFID, got %s", globalRANNodeID.GlobalN3IWFID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Value != int64(lib.IDRANNodeName) {
		t.Errorf("IE id = %d, want %d", item1.ID.Value, lib.IDRANNodeName)
	}

	if item1.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item1.Criticality)
	}

	ranNodeName, ok := item1.Value.(string)
	if !ok {
		t.Fatalf("expected string, got %T", item1.Value)
	}

	if ranNodeName != "UERANSIM-gnb-1-1-1" {
		t.Errorf("expected RANNodeName=UERANSIM-gnb-1-1-1, got %s", ranNodeName)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDSupportedTAList) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDSupportedTAList)
	}

	if item2.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item2.Criticality)
	}

	supportedTAList, ok := item2.Value.([]SupportedTA)
	if !ok {
		t.Fatalf("expected SupportedTAList, got %T", item2.Value)
	}

	if supportedTAList == nil {
		t.Fatalf("expected SupportedTAList, got nil")
	}

	if len(supportedTAList) != 1 {
		t.Fatalf("expected 1 SupportedTAItem, got %d", len(supportedTAList))
	}

	supportedTAItem := supportedTAList[0]

	if supportedTAItem.TAC != "000001" {
		t.Errorf("expected TAC=000001, got %s", supportedTAItem.TAC)
	}

	if len(supportedTAItem.BroadcastPLMNList) != 1 {
		t.Fatalf("expected 1 BroadcastPLMN, got %d", len(supportedTAItem.BroadcastPLMNList))
	}

	if supportedTAItem.BroadcastPLMNList[0].PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", supportedTAItem.BroadcastPLMNList[0].PLMNID.Mcc)
	}

	if supportedTAItem.BroadcastPLMNList[0].PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", supportedTAItem.BroadcastPLMNList[0].PLMNID.Mnc)
	}

	if len(supportedTAItem.BroadcastPLMNList[0].SliceSupportList) != 1 {
		t.Fatalf("expected 1 S-NSSAI, got %d", len(supportedTAItem.BroadcastPLMNList[0].SliceSupportList))
	}

	snssai := supportedTAItem.BroadcastPLMNList[0].SliceSupportList[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Value != int64(lib.IDDefaultPagingDRX) {
		t.Errorf("IE id = %d, want %d", item3.ID.Value, lib.IDDefaultPagingDRX)
	}

	if item3.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item3.Criticality)
	}

	defaultPagingDRX, ok := item3.Value.(utils.EnumField)
	if !ok {
		t.Fatalf("expected EnumField, got %T", item3.Value)
	}

	if defaultPagingDRX.Value != int64(lib.PagingDRXv128) {
		t.Errorf("expected DefaultPagingDRX=v128, got %s", defaultPagingDRX.Label)
	}

	if defaultPagingDRX.Value != int64(lib.PagingDRXv128) {
		t.Errorf("expected DefaultPagingDRX value=2, got %d", defaultPagingDRX.Value)
	}
}

func TestDecodeNGAPMessage_NGSetupResponse(t *testing.T) {
	raw, err := decodeB64(ngSetupResponseCapture)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "SuccessfulOutcome" {
		t.Errorf("expected PDUType=SuccessfulOutcome, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcNGSetup) {
		t.Errorf("expected ProcedureCode=NGSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcNGSetup) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcNGSetup)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDAMFName) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDAMFName)
	}

	if item0.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item0.Criticality)
	}

	amfName, ok := item0.Value.(string)
	if !ok {
		t.Fatalf("expected string, got %T", item0.Value)
	}

	if amfName != "amf" {
		t.Errorf("expected AMFName=amf, got %s", amfName)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Value != int64(lib.IDServedGUAMIList) {
		t.Errorf("IE id = %d, want %d", item1.ID.Value, lib.IDServedGUAMIList)
	}

	if item1.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item1.Criticality)
	}

	servedGUAMIList, ok := item1.Value.([]Guami)
	if !ok {
		t.Fatalf("expected ServedGUAMIList, got %T", item1.Value)
	}

	if servedGUAMIList == nil {
		t.Fatalf("expected ServedGUAMIList, got nil")
	}

	if len(servedGUAMIList) != 1 {
		t.Fatalf("expected 1 GUAMI, got %d", len(servedGUAMIList))
	}

	guami := servedGUAMIList[0]

	if guami.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", guami.PLMNID.Mcc)
	}

	if guami.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", guami.PLMNID.Mnc)
	}

	if guami.AMFRegionID != "ca" {
		t.Errorf("expected AMFRegionID=ca, got %s", guami.AMFRegionID)
	}

	if guami.AMFSetID != "fe0" {
		t.Errorf("expected AMFSetID=fe0, got %s", guami.AMFSetID)
	}

	if guami.AMFPointer != "00" {
		t.Errorf("expected AMFPointer=00, got %s", guami.AMFPointer)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDRelativeAMFCapacity) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDRelativeAMFCapacity)
	}

	if item2.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item2.Criticality)
	}

	relativeAMFCapacity, ok := item2.Value.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", item2.Value)
	}

	if relativeAMFCapacity != 255 {
		t.Errorf("expected RelativeAMFCapacity=255, got %d", relativeAMFCapacity)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Value != int64(lib.IDPLMNSupportList) {
		t.Errorf("IE id = %d, want %d", item3.ID.Value, lib.IDPLMNSupportList)
	}

	if item3.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item3.Criticality)
	}

	plmnSupportList, ok := item3.Value.([]PLMN)
	if !ok {
		t.Fatalf("expected PLMNSupportList, got %T", item3.Value)
	}

	if plmnSupportList == nil {
		t.Fatalf("expected PLMNSupportList, got nil")
	}

	if len(plmnSupportList) != 1 {
		t.Fatalf("expected 1 PLMNSupportItem, got %d", len(plmnSupportList))
	}

	plmnItem := plmnSupportList[0]

	if plmnItem.PLMNID.Mcc != "001" {
		t.Errorf("expected Mcc=001, got %s", plmnItem.PLMNID.Mcc)
	}

	if plmnItem.PLMNID.Mnc != "01" {
		t.Errorf("expected Mnc=01, got %s", plmnItem.PLMNID.Mnc)
	}

	if len(plmnItem.SliceSupportList) != 1 {
		t.Fatalf("expected 1 S-NSSAI, got %d", len(plmnItem.SliceSupportList))
	}

	snssai := plmnItem.SliceSupportList[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}
}

func TestDecodeNGAPMessage_NGSetupFailure(t *testing.T) {
	raw, err := decodeB64(ngSetupFailureCapture)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "UnsuccessfulOutcome" {
		t.Errorf("expected PDUType=UnsuccessfulOutcome, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcNGSetup) {
		t.Errorf("expected ProcedureCode=NGSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcNGSetup) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcNGSetup)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 1 {
		t.Errorf("expected 1 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDCause) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDCause)
	}

	if item0.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item0.Criticality)
	}

	cause, ok := item0.Value.(Cause)
	if !ok {
		t.Fatalf("expected Cause, got %T", item0.Value)
	}

	if cause.Value.Value != int64(lib.CauseMiscUnknownPLMNOrSNPN) {
		t.Errorf("expected Cause=unknown-PLMN-or-SNPN, got %v", cause.Value.Label)
	}

	if cause.Value.Value != int64(lib.CauseMiscUnknownPLMNOrSNPN) {
		t.Errorf("expected Cause value=%d, got %d", int64(lib.CauseMiscUnknownPLMNOrSNPN), cause.Value.Value)
	}
}

// An NGSetupRequest captured on the 001/01 test PLMN.
const ngSetupRequestCapture = "ABUAQQAABAAbAAkAAPEQUAAAAAEAUkAUCIBVRVJBTlNJTS1nbmItMS0xLTEAZgAQAAAAAAEAAPEQAAAQCBAgMAAVQAFA"

// An NGSetupResponse captured on the 001/01 test PLMN.
const ngSetupResponseCapture = "IBUALAAABAABAAUBAGFtZgBgAAgAAADxEMr+AABWQAH/AFAACwAA8RAAABAIECAw"

// An NGSetupFailure captured on the 001/01 test PLMN.
const ngSetupFailureCapture = "QBUACAAAAQAPQAGI"
