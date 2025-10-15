package decoder_test

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/decoder"
)

func decodeB64(s string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}

	return nil, fmt.Errorf("not valid base64")
}

func TestDecode_NGSetupRequest(t *testing.T) {
	const message = "ABUAQQAABAAbAAkAAPEQUAAAAAEAUkAUCIBVRVJBTlNJTS1nbmItMS0xLTEAZgAQAAAAAAEAAPEQAAAQCBAgMAAVQAFA"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := decoder.DecodeNetworkLog(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.InitiatingMessage == nil {
		t.Fatalf("expected InitiatingMessage, got nil")
	}

	if ngap.InitiatingMessage.ProcedureCode != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %s", ngap.InitiatingMessage.ProcedureCode)
	}

	if ngap.InitiatingMessage.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", ngap.InitiatingMessage.Criticality)
	}

	if ngap.InitiatingMessage.Value.NGSetupRequest == nil {
		t.Fatalf("expected NGSetupRequest, got nil")
	}

	if len(ngap.InitiatingMessage.Value.NGSetupRequest.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngap.InitiatingMessage.Value.NGSetupRequest.IEs))
	}

	item0 := ngap.InitiatingMessage.Value.NGSetupRequest.IEs[0]

	if item0.ID != "GlobalRANNodeID (27)" {
		t.Errorf("expected ID=GlobalRANNodeID (27), got %s", item0.ID)
	}

	if item0.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item0.Criticality)
	}

	if item0.GlobalRANNodeID == nil {
		t.Fatalf("expected GlobalRANNodeID, got nil")
	}

	if item0.GlobalRANNodeID.GlobalGNBID != "00000001" {
		t.Errorf("expected GlobalGNBID=00000001, got %s", item0.GlobalRANNodeID.GlobalGNBID)
	}

	if item0.GlobalRANNodeID.GlobalNgENBID != "" {
		t.Errorf("expected empty globalNgENBID, got %s", item0.GlobalRANNodeID.GlobalNgENBID)
	}

	if item0.GlobalRANNodeID.GlobalN3IWFID != "" {
		t.Errorf("expected empty GlobalN3IWFID, got %s", item0.GlobalRANNodeID.GlobalN3IWFID)
	}

	item1 := ngap.InitiatingMessage.Value.NGSetupRequest.IEs[1]

	if item1.ID != "RANNodeName (82)" {
		t.Errorf("expected ID=RANNodeName (82), got %s", item1.ID)
	}

	if item1.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item1.Criticality)
	}

	if item1.RANNodeName == nil {
		t.Fatalf("expected RANNodeName, got nil")
	}

	if *item1.RANNodeName != "UERANSIM-gnb-1-1-1" {
		t.Errorf("expected RANNodeName=UERANSIM-gnb-1-1-1, got %s", *item1.RANNodeName)
	}

	item2 := ngap.InitiatingMessage.Value.NGSetupRequest.IEs[2]

	if item2.ID != "SupportedTAList (102)" {
		t.Errorf("expected ID=SupportedTAList (102), got %s", item2.ID)
	}

	if item2.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item2.Criticality)
	}

	if item2.SupportedTAList == nil {
		t.Fatalf("expected SupportedTAList, got nil")
	}

	if len(item2.SupportedTAList) != 1 {
		t.Fatalf("expected 1 SupportedTAItem, got %d", len(item2.SupportedTAList))
	}

	supportedTAItem := item2.SupportedTAList[0]

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
		t.Fatalf("expected 1 SNSSAI, got %d", len(supportedTAItem.BroadcastPLMNList[0].SliceSupportList))
	}

	snssai := supportedTAItem.BroadcastPLMNList[0].SliceSupportList[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}

	item3 := ngap.InitiatingMessage.Value.NGSetupRequest.IEs[3]

	if item3.ID != "DefaultPagingDRX (21)" {
		t.Errorf("expected ID=DefaultPagingDRX (21), got %s", item3.ID)
	}

	if item3.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item3.Criticality)
	}

	if item3.DefaultPagingDRX == nil {
		t.Fatalf("expected DefaultPagingDRX, got nil")
	}

	expectedDRX := "v128"
	if *item3.DefaultPagingDRX != expectedDRX {
		t.Errorf("expected DefaultPagingDRX=%s, got %s", expectedDRX, *item3.DefaultPagingDRX)
	}
}

func TestDecode_NGSetupResponse(t *testing.T) {
	const message = "IBUALAAABAABAAUBAGFtZgBgAAgAAADxEMr+AABWQAH/AFAACwAA8RAAABAIECAw"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := decoder.DecodeNetworkLog(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.SuccessfulOutcome == nil {
		t.Fatalf("expected SuccessfulOutcome, got nil")
	}

	if ngap.SuccessfulOutcome.ProcedureCode != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %s", ngap.SuccessfulOutcome.ProcedureCode)
	}

	if ngap.SuccessfulOutcome.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", ngap.SuccessfulOutcome.Criticality)
	}

	if ngap.SuccessfulOutcome.Value.NGSetupResponse == nil {
		t.Fatalf("expected NGSetupResponse, got nil")
	}

	if len(ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs))
	}

	item0 := ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs[0]

	if item0.ID != "AMFName (1)" {
		t.Errorf("expected ID=AMFName (1), got %s", item0.ID)
	}

	if item0.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item0.Criticality)
	}

	if item0.AMFName == nil {
		t.Fatalf("expected AMFName, got nil")
	}

	if *item0.AMFName != "amf" {
		t.Errorf("expected AMFName=amf, got %s", *item0.AMFName)
	}

	item1 := ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs[1]

	if item1.ID != "ServedGUAMIList (96)" {
		t.Errorf("expected ID=ServedGUAMIList (96), got %s", item1.ID)
	}

	if item1.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item1.Criticality)
	}

	if item1.ServedGUAMIList == nil {
		t.Fatalf("expected ServedGUAMIList, got nil")
	}

	if len(item1.ServedGUAMIList) != 1 {
		t.Fatalf("expected 1 GUAMI, got %d", len(item1.ServedGUAMIList))
	}

	guami := item1.ServedGUAMIList[0]

	if guami.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", guami.PLMNID.Mcc)
	}

	if guami.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", guami.PLMNID.Mnc)
	}

	if guami.AMFID != "cafe00" {
		t.Errorf("expected AMFID=cafe00, got %s", guami.AMFID)
	}

	item2 := ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs[2]

	if item2.ID != "RelativeAMFCapacity (86)" {
		t.Errorf("expected ID=RelativeAMFCapacity (86), got %s", item2.ID)
	}

	if item2.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item2.Criticality)
	}

	if item2.RelativeAMFCapacity == nil {
		t.Fatalf("expected RelativeAMFCapacity, got nil")
	}

	if *item2.RelativeAMFCapacity != 255 {
		t.Errorf("expected RelativeAMFCapacity=255, got %d", *item2.RelativeAMFCapacity)
	}

	item3 := ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs[3]

	if item3.ID != "PLMNSupportList (80)" {
		t.Errorf("expected ID=PLMNSupportList (80), got %s", item3.ID)
	}

	if item3.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item3.Criticality)
	}

	if item3.PLMNSupportList == nil {
		t.Fatalf("expected PLMNSupportList, got nil")
	}

	if len(item3.PLMNSupportList) != 1 {
		t.Fatalf("expected 1 PLMNSupportItem, got %d", len(item3.PLMNSupportList))
	}

	plmnItem := item3.PLMNSupportList[0]

	if plmnItem.PLMNID.Mcc != "001" {
		t.Errorf("expected Mcc=001, got %s", plmnItem.PLMNID.Mcc)
	}

	if plmnItem.PLMNID.Mnc != "01" {
		t.Errorf("expected Mnc=01, got %s", plmnItem.PLMNID.Mnc)
	}

	if len(plmnItem.SliceSupportList) != 1 {
		t.Fatalf("expected 1 SNSSAI, got %d", len(plmnItem.SliceSupportList))
	}

	snssai := plmnItem.SliceSupportList[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}
}

func TestDecode_NGSetupFailure(t *testing.T) {
	const message = "QBUACAAAAQAPQAGI"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := decoder.DecodeNetworkLog(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.UnsuccessfulOutcome == nil {
		t.Fatalf("expected UnsuccessfulOutcome, got nil")
	}

	if ngap.UnsuccessfulOutcome.ProcedureCode != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %s", ngap.UnsuccessfulOutcome.ProcedureCode)
	}

	if ngap.UnsuccessfulOutcome.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", ngap.UnsuccessfulOutcome.Criticality)
	}

	if ngap.UnsuccessfulOutcome.Value.NGSetupFailure == nil {
		t.Fatalf("expected NGSetupFailure, got nil")
	}

	if len(ngap.UnsuccessfulOutcome.Value.NGSetupFailure.IEs) != 1 {
		t.Errorf("expected 1 ProtocolIEs, got %d", len(ngap.UnsuccessfulOutcome.Value.NGSetupFailure.IEs))
	}

	item0 := ngap.UnsuccessfulOutcome.Value.NGSetupFailure.IEs[0]

	if item0.ID != "Cause (15)" {
		t.Errorf("expected ID=Cause (15), got %s", item0.ID)
	}

	if item0.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item0.Criticality)
	}

	if item0.Cause == nil {
		t.Fatalf("expected Cause, got nil")
	}

	if *item0.Cause != "UnknownPLMN (4)" {
		t.Errorf("expected Cause=UnknownPLMN (4), got %s", *item0.Cause)
	}
}

func TestDecode_InitialUEMessage(t *testing.T) {
	const message = "AA9ASAAABQBVAAIAAQAmABoZfgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8AB5ABNQAPEQAAAAAQAA8RAAAAHsmTVKAFpAARgAcEABAA=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := decoder.DecodeNetworkLog(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.InitiatingMessage == nil {
		t.Fatalf("expected InitiatingMessage, got nil")
	}

	if ngap.InitiatingMessage.ProcedureCode != "InitialUEMessage" {
		t.Errorf("expected ProcedureCode=InitialUEMessage, got %s", ngap.InitiatingMessage.ProcedureCode)
	}

	if ngap.InitiatingMessage.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", ngap.InitiatingMessage.Criticality)
	}

	if ngap.InitiatingMessage.Value.InitialUEMessage == nil {
		t.Fatalf("expected InitialUEMessage, got nil")
	}

	if len(ngap.InitiatingMessage.Value.InitialUEMessage.IEs) != 5 {
		t.Errorf("expected 5 ProtocolIEs, got %d", len(ngap.InitiatingMessage.Value.InitialUEMessage.IEs))
	}

	item0 := ngap.InitiatingMessage.Value.InitialUEMessage.IEs[0]

	if item0.ID != "RANUENGAPID (85)" {
		t.Errorf("expected ID=RANUENGAPID (85), got %s", item0.ID)
	}

	if item0.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item0.Criticality)
	}

	if item0.RANUENGAPID == nil {
		t.Fatalf("expected RANUENGAPID, got nil")
	}

	if *item0.RANUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", *item0.RANUENGAPID)
	}

	item1 := ngap.InitiatingMessage.Value.InitialUEMessage.IEs[1]

	if item1.ID != "NASPDU (38)" {
		t.Errorf("expected ID=NASPDU (38), got %s", item1.ID)
	}

	if item1.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item1.Criticality)
	}

	if item1.NASPDU == nil {
		t.Fatalf("expected NASPDU, got nil")
	}

	expectedNASPDU := "fgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8A=="
	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(item1.NASPDU) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, item1.NASPDU)
	}

	item2 := ngap.InitiatingMessage.Value.InitialUEMessage.IEs[2]

	if item2.ID != "UserLocationInformation (121)" {
		t.Errorf("expected ID=UserLocationInformation (121), got %s", item2.ID)
	}

	if item2.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item2.Criticality)
	}

	if item2.UserLocationInformation == nil {
		t.Fatalf("expected UserLocationInformation, got nil")
	}

	if item2.UserLocationInformation.NR == nil {
		t.Fatalf("expected NR, got nil")
	}

	if item2.UserLocationInformation.NR.TAI.TAC != "000001" {
		t.Errorf("expected TAC=000001, got %s", item2.UserLocationInformation.NR.TAI.TAC)
	}

	if item2.UserLocationInformation.NR.TAI.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", item2.UserLocationInformation.NR.TAI.PLMNID.Mcc)
	}

	if item2.UserLocationInformation.NR.TAI.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", item2.UserLocationInformation.NR.TAI.PLMNID.Mnc)
	}

	// read timestamp and convert to time
	if item2.UserLocationInformation.NR.TimeStamp == nil {
		t.Fatalf("expected TimeStamp, got nil")
	}

	if *item2.UserLocationInformation.NR.TimeStamp != "2025-10-14T20:47:06Z" {
		t.Errorf("expected TimeStamp=2025-10-14T20:47:06Z, got %s", *item2.UserLocationInformation.NR.TimeStamp)
	}

	item3 := ngap.InitiatingMessage.Value.InitialUEMessage.IEs[3]

	if item3.ID != "RRCEstablishmentCause (90)" {
		t.Errorf("expected ID=RRCEstablishmentCause (90), got %s", item3.ID)
	}

	if item3.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item3.Criticality)
	}

	if item3.RRCEstablishmentCause == nil {
		t.Fatalf("expected RRCEstablishmentCause, got nil")
	}

	if *item3.RRCEstablishmentCause != "MoSignalling" {
		t.Errorf("expected RRCEstablishmentCause=MoSignalling, got %s", *item3.RRCEstablishmentCause)
	}

	item4 := ngap.InitiatingMessage.Value.InitialUEMessage.IEs[4]

	if item4.ID != "UEContextRequest (112)" {
		t.Errorf("expected ID=UEContextRequest (112), got %s", item4.ID)
	}

	if item4.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item4.Criticality)
	}

	if item4.UEContextRequest == nil {
		t.Fatalf("expected UEContextRequest, got nil")
	}

	if *item4.UEContextRequest != "Requested" {
		t.Errorf("expected UEContextRequest=Requested, got %v", *item4.UEContextRequest)
	}
}

func TestDecode_DownlinkNASTransport(t *testing.T) {
	const message = "AARAPgAAAwAKAAIAAQBVAAIAAQAmACsqfgBWAAIAACEaBwCjbSa9vkiAkRdky8+5IBBH2jhAU2SAAE2CgCRBSs2H"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := decoder.DecodeNetworkLog(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.InitiatingMessage == nil {
		t.Fatalf("expected InitiatingMessage, got nil")
	}

	if ngap.InitiatingMessage.ProcedureCode != "DownlinkNASTransport" {
		t.Errorf("expected ProcedureCode=DownlinkNASTransport, got %s", ngap.InitiatingMessage.ProcedureCode)
	}

	if ngap.InitiatingMessage.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", ngap.InitiatingMessage.Criticality)
	}

	if ngap.InitiatingMessage.Value.DownlinkNASTransport == nil {
		t.Fatalf("expected DownlinkNASTransport, got nil")
	}

	if len(ngap.InitiatingMessage.Value.DownlinkNASTransport.IEs) != 3 {
		t.Errorf("expected 3 ProtocolIEs, got %d", len(ngap.InitiatingMessage.Value.DownlinkNASTransport.IEs))
	}

	item0 := ngap.InitiatingMessage.Value.DownlinkNASTransport.IEs[0]

	if item0.ID != "AMFUENGAPID (10)" {
		t.Errorf("expected ID=AMFUENGAPID (10), got %s", item0.ID)
	}

	if item0.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item0.Criticality)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 1 {
		t.Errorf("expected AMFUENGAPID=1, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngap.InitiatingMessage.Value.DownlinkNASTransport.IEs[1]

	if item1.ID != "RANUENGAPID (85)" {
		t.Errorf("expected ID=RANUENGAPID (85), got %s", item1.ID)
	}

	if item1.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item1.Criticality)
	}

	if item1.RANUENGAPID == nil {
		t.Fatalf("expected RANUENGAPID, got nil")
	}

	if *item1.RANUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", *item1.RANUENGAPID)
	}

	item2 := ngap.InitiatingMessage.Value.DownlinkNASTransport.IEs[2]

	if item2.ID != "NASPDU (38)" {
		t.Errorf("expected ID=NASPDU (38), got %s", item2.ID)
	}

	if item2.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item2.Criticality)
	}

	if item2.NASPDU == nil {
		t.Fatalf("expected NASPDU, got nil")
	}

	expectedNASPDU := "fgBWAAIAACEaBwCjbSa9vkiAkRdky8+5IBBH2jhAU2SAAE2CgCRBSs2H"
	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(item2.NASPDU) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, item2.NASPDU)
	}
}

func TestDecode_UplinkNASTransport(t *testing.T) {
	const message = "AC5APwAABAAKAAIAAQBVAAIAAQAmABUUfgLpGbfKA34AZwEABS4BANZREgEAeUATUADxEAAAAAEAAPEQAAAB7JlGUQ=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := decoder.DecodeNetworkLog(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.InitiatingMessage == nil {
		t.Fatalf("expected InitiatingMessage, got nil")
	}

	if ngap.InitiatingMessage.ProcedureCode != "UplinkNASTransport" {
		t.Errorf("expected ProcedureCode=UplinkNASTransport, got %s", ngap.InitiatingMessage.ProcedureCode)
	}

	if ngap.InitiatingMessage.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", ngap.InitiatingMessage.Criticality)
	}

	if ngap.InitiatingMessage.Value.UplinkNASTransport == nil {
		t.Fatalf("expected UplinkNASTransport, got nil")
	}

	if len(ngap.InitiatingMessage.Value.UplinkNASTransport.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngap.InitiatingMessage.Value.UplinkNASTransport.IEs))
	}

	item0 := ngap.InitiatingMessage.Value.UplinkNASTransport.IEs[0]

	if item0.ID != "AMFUENGAPID (10)" {
		t.Errorf("expected ID=AMFUENGAPID (10), got %s", item0.ID)
	}

	if item0.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item0.Criticality)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 1 {
		t.Errorf("expected AMFUENGAPID=1, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngap.InitiatingMessage.Value.UplinkNASTransport.IEs[1]

	if item1.ID != "RANUENGAPID (85)" {
		t.Errorf("expected ID=RANUENGAPID (85), got %s", item1.ID)
	}

	if item1.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item1.Criticality)
	}

	if item1.RANUENGAPID == nil {
		t.Fatalf("expected RANUENGAPID, got nil")
	}

	if *item1.RANUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", *item1.RANUENGAPID)
	}

	item2 := ngap.InitiatingMessage.Value.UplinkNASTransport.IEs[2]

	if item2.ID != "NASPDU (38)" {
		t.Errorf("expected ID=NASPDU (38), got %s", item2.ID)
	}

	if item2.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item2.Criticality)
	}

	if item2.NASPDU == nil {
		t.Fatalf("expected NASPDU, got nil")
	}

	expectedNASPDU := "fgLpGbfKA34AZwEABS4BANZREgE="
	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(item2.NASPDU) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, item2.NASPDU)
	}

	item3 := ngap.InitiatingMessage.Value.UplinkNASTransport.IEs[3]

	if item3.ID != "UserLocationInformation (121)" {
		t.Errorf("expected ID=UserLocationInformation (121), got %s", item3.ID)
	}

	if item3.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item3.Criticality)
	}

	if item3.UserLocationInformation == nil {
		t.Fatalf("expected UserLocationInformation, got nil")
	}

	if item3.UserLocationInformation.NR == nil {
		t.Fatalf("expected NR, got nil")
	}

	if item3.UserLocationInformation.NR.TAI.TAC != "000001" {
		t.Errorf("expected TAC=000001, got %s", item3.UserLocationInformation.NR.TAI.TAC)
	}

	if item3.UserLocationInformation.NR.TAI.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", item3.UserLocationInformation.NR.TAI.PLMNID.Mcc)
	}

	if item3.UserLocationInformation.NR.TAI.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", item3.UserLocationInformation.NR.TAI.PLMNID.Mnc)
	}

	// read timestamp and convert to time
	if item3.UserLocationInformation.NR.TimeStamp == nil {
		t.Fatalf("expected TimeStamp, got nil")
	}

	if *item3.UserLocationInformation.NR.TimeStamp != "2025-10-14T21:59:45Z" {
		t.Errorf("expected TimeStamp=2025-10-14T21:59:45Z, got %s", *item3.UserLocationInformation.NR.TimeStamp)
	}
}

func TestDecode_InitialContextSetupRequest(t *testing.T) {
	const message = "AA4AgJQAAAgACgACAAQAVQACAAIAHAAHAADxEMr+AAAAAAUCARAgMAB3AAkcAA4AAAAAAAAAXgAgmoWQH+QL60OhHSJbbTHIzCPUPAVPceX9UqhcE2VOITwAJEAEAADxEAAmQDQzfgKx/lSdAX4AQgEBdwAL8gDxEMr+AAAAAAFKAwDxEFQHAADxEAAAARUFBAEQIDAhAgAA"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := decoder.DecodeNetworkLog(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.InitiatingMessage == nil {
		t.Fatalf("expected InitiatingMessage, got nil")
	}

	if ngap.InitiatingMessage.ProcedureCode != "InitialContextSetup" {
		t.Errorf("expected ProcedureCode=InitialContextSetup, got %s", ngap.InitiatingMessage.ProcedureCode)
	}

	if ngap.InitiatingMessage.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", ngap.InitiatingMessage.Criticality)
	}

	if ngap.InitiatingMessage.Value.InitialContextSetupRequest == nil {
		t.Fatalf("expected InitialContextSetupRequest, got nil")
	}

	if len(ngap.InitiatingMessage.Value.InitialContextSetupRequest.IEs) != 8 {
		t.Errorf("expected 8 ProtocolIEs, got %d", len(ngap.InitiatingMessage.Value.InitialContextSetupRequest.IEs))
	}

	item0 := ngap.InitiatingMessage.Value.InitialContextSetupRequest.IEs[0]

	if item0.ID != "AMFUENGAPID (10)" {
		t.Errorf("expected ID=AMFUENGAPID (10), got %s", item0.ID)
	}

	if item0.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item0.Criticality)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 4 {
		t.Errorf("expected AMFUENGAPID=4, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngap.InitiatingMessage.Value.InitialContextSetupRequest.IEs[1]

	if item1.ID != "RANUENGAPID (85)" {
		t.Errorf("expected ID=RANUENGAPID (85), got %s", item1.ID)
	}

	if item1.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item1.Criticality)
	}

	if item1.RANUENGAPID == nil {
		t.Fatalf("expected RANUENGAPID, got nil")
	}

	if *item1.RANUENGAPID != 2 {
		t.Errorf("expected RANUENGAPID=2, got %d", *item1.RANUENGAPID)
	}

	item2 := ngap.InitiatingMessage.Value.InitialContextSetupRequest.IEs[2]

	if item2.ID != "GUAMI (28)" {
		t.Errorf("expected ID=GUAMI (28), got %s", item2.ID)
	}

	if item2.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item2.Criticality)
	}

	if item2.GUAMI == nil {
		t.Fatalf("expected GUAMI, got nil")
	}

	if item2.GUAMI.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", item2.GUAMI.PLMNID.Mcc)
	}

	if item2.GUAMI.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", item2.GUAMI.PLMNID.Mnc)
	}

	if item2.GUAMI.AMFID != "cafe00" {
		t.Errorf("expected AMFID=cafe00, got %s", item2.GUAMI.AMFID)
	}

	item3 := ngap.InitiatingMessage.Value.InitialContextSetupRequest.IEs[3]

	if item3.ID != "AllowedNSSAI (0)" {
		t.Errorf("expected ID=AllowedNSSAI (0), got %s", item3.ID)
	}

	if item3.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item3.Criticality)
	}

	if item3.AllowedNSSAI == nil {
		t.Fatalf("expected AllowedNSSAI, got nil")
	}

	if len(item3.AllowedNSSAI) != 1 {
		t.Fatalf("expected 1 SNSSAI, got %d", len(item3.AllowedNSSAI))
	}

	snssai := item3.AllowedNSSAI[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}

	item4 := ngap.InitiatingMessage.Value.InitialContextSetupRequest.IEs[4]

	if item4.ID != "UESecurityCapabilities (119)" {
		t.Errorf("expected ID=UESecurityCapabilities (119), got %s", item4.ID)
	}

	if item4.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item4.Criticality)
	}

	if item4.UESecurityCapabilities == nil {
		t.Fatalf("expected UESecurityCapabilities, got nil")
	}

	if item4.UESecurityCapabilities.NRencryptionAlgorithms != "e000" {
		t.Fatalf("expected NRIntegrityProtectionAlgorithms=e000, got %s", item4.UESecurityCapabilities.NRencryptionAlgorithms)
	}

	if item4.UESecurityCapabilities.NRintegrityProtectionAlgorithms != "e000" {
		t.Fatalf("expected NRIntegrityProtectionAlgorithms=e000, got %s", item4.UESecurityCapabilities.NRintegrityProtectionAlgorithms)
	}

	if item4.UESecurityCapabilities.EUTRAencryptionAlgorithms != "0000" {
		t.Fatalf("expected EUTRAencryptionAlgorithms=0000, got %s", item4.UESecurityCapabilities.EUTRAencryptionAlgorithms)
	}

	if item4.UESecurityCapabilities.EUTRAintegrityProtectionAlgorithms != "0000" {
		t.Fatalf("expected EUTRAintegrityProtectionAlgorithms=0000, got %s", item4.UESecurityCapabilities.EUTRAintegrityProtectionAlgorithms)
	}

	item5 := ngap.InitiatingMessage.Value.InitialContextSetupRequest.IEs[5]

	if item5.ID != "SecurityKey (94)" {
		t.Errorf("expected ID=SecurityKey (94), got %s", item5.ID)
	}

	if item5.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item5.Criticality)
	}

	if item5.SecurityKey == nil {
		t.Fatalf("expected SecurityKey, got nil")
	}

	expectedKey := "9a85901fe40beb43a11d225b6d31c8cc23d43c054f71e5fd52a85c13654e213c"
	if *item5.SecurityKey != expectedKey {
		t.Errorf("expected SecurityKey=%s, got %s", expectedKey, *item5.SecurityKey)
	}

	item6 := ngap.InitiatingMessage.Value.InitialContextSetupRequest.IEs[6]

	if item6.ID != "MobilityRestrictionList (36)" {
		t.Errorf("expected ID=MobilityRestrictionList (36), got %s", item6.ID)
	}

	if item6.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item6.Criticality)
	}

	if item6.MobilityRestrictionList == nil {
		t.Fatalf("expected MobilityRestrictionList, got nil")
	}

	if item6.MobilityRestrictionList.ServingPLMN.Mcc != "001" {
		t.Errorf("expected ServingPLMN.Mcc=001, got %s", item6.MobilityRestrictionList.ServingPLMN.Mcc)
	}

	if item6.MobilityRestrictionList.ServingPLMN.Mnc != "01" {
		t.Errorf("expected ServingPLMN.Mnc=01, got %s", item6.MobilityRestrictionList.ServingPLMN.Mnc)
	}

	if item6.MobilityRestrictionList.EquivalentPLMNs != nil {
		t.Fatalf("expected EquivalentPLMNs=nil, got %v", item6.MobilityRestrictionList.EquivalentPLMNs)
	}

	if item6.MobilityRestrictionList.RATRestrictions != nil {
		t.Fatalf("expected RATRestrictions=nil, got %v", item6.MobilityRestrictionList.RATRestrictions)
	}

	if item6.MobilityRestrictionList.ForbiddenAreaInformation != nil {
		t.Fatalf("expected ForbiddenAreaInformation=nil, got %v", item6.MobilityRestrictionList.ForbiddenAreaInformation)
	}

	if item6.MobilityRestrictionList.ServiceAreaInformation != nil {
		t.Fatalf("expected ServiceAreaInformation=nil, got %v", item6.MobilityRestrictionList.ServiceAreaInformation)
	}

	item7 := ngap.InitiatingMessage.Value.InitialContextSetupRequest.IEs[7]

	if item7.ID != "NASPDU (38)" {
		t.Errorf("expected ID=NASPDU (38), got %s", item7.ID)
	}

	if item7.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item7.Criticality)
	}

	if item7.NASPDU == nil {
		t.Fatalf("expected NASPDU, got nil")
	}

	expectedNASPDU := "fgKx/lSdAX4AQgEBdwAL8gDxEMr+AAAAAAFKAwDxEFQHAADxEAAAARUFBAEQIDAhAgAA"

	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(item7.NASPDU) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, item7.NASPDU)
	}
}

func TestDecode_InitialContextSetupResponse(t *testing.T) {
	const message = "IA4ADwAAAgAKQAIAAgBVQAIAAg=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := decoder.DecodeNetworkLog(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.SuccessfulOutcome == nil {
		t.Fatalf("expected SuccessfulOutcome, got nil")
	}

	if ngap.SuccessfulOutcome.ProcedureCode != "InitialContextSetup" {
		t.Errorf("expected ProcedureCode=InitialContextSetup, got %s", ngap.SuccessfulOutcome.ProcedureCode)
	}

	if ngap.SuccessfulOutcome.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", ngap.SuccessfulOutcome.Criticality)
	}

	if ngap.SuccessfulOutcome.Value.InitialContextSetupResponse == nil {
		t.Fatalf("expected InitialContextSetupResponse, got nil")
	}

	if len(ngap.SuccessfulOutcome.Value.InitialContextSetupResponse.IEs) != 2 {
		t.Errorf("expected 2 ProtocolIEs, got %d", len(ngap.SuccessfulOutcome.Value.InitialContextSetupResponse.IEs))
	}

	item0 := ngap.SuccessfulOutcome.Value.InitialContextSetupResponse.IEs[0]

	if item0.ID != "AMFUENGAPID (10)" {
		t.Errorf("expected ID=AMFUENGAPID (10), got %s", item0.ID)
	}

	if item0.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item0.Criticality)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 2 {
		t.Errorf("expected AMFUENGAPID=2, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngap.SuccessfulOutcome.Value.InitialContextSetupResponse.IEs[1]

	if item1.ID != "RANUENGAPID (85)" {
		t.Errorf("expected ID=RANUENGAPID (85), got %s", item1.ID)
	}

	if item1.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item1.Criticality)
	}

	if item1.RANUENGAPID == nil {
		t.Fatalf("expected RANUENGAPID, got nil")
	}

	if *item1.RANUENGAPID != 2 {
		t.Errorf("expected RANUENGAPID=2, got %d", *item1.RANUENGAPID)
	}
}
