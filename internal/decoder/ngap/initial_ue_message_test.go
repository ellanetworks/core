package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/free5gc/ngap/ngapType"
)

func TestDecodeNGAPMessage_InitialUEMessage(t *testing.T) {
	const message = "AA9ASAAABQBVAAIAAQAmABoZfgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8AB5ABNQAPEQAAAAAQAA8RAAAAHsmTVKAFpAARgAcEABAA=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "InitialUEMessage" {
		t.Errorf("expected MessageType=InitialUEMessage, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "InitialUEMessage" {
		t.Errorf("expected ProcedureCode=InitialUEMessage, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeInitialUEMessage {
		t.Errorf("expected ProcedureCode value=9, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Value != 1 {
		t.Errorf("expected Criticality=Ignore (1), got %d", ngapMsg.Criticality.Value)
	}

	if len(ngapMsg.Value.IEs) != 5 {
		t.Errorf("expected 5 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item0.ID.Label)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDRANUENGAPID {
		t.Errorf("expected ID value=85, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	ranUENGAPID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected RANUENGAPID to be of type int64, got %T", item0.Value)
	}

	if ranUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", ranUENGAPID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "NASPDU" {
		t.Errorf("expected ID=NASPDU, got %s", item1.ID.Label)
	}

	if item1.ID.Value != ngapType.ProtocolIEIDNASPDU {
		t.Errorf("expected ID value=38, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item1.Criticality.Value)
	}

	nasPdu, ok := item1.Value.(ngap.NASPDU)
	if !ok {
		t.Fatalf("expected NASPDU to be of type ngap.NASPDU, got %T", item1.Value)
	}

	expectedNASPDU := "fgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8A=="
	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(nasPdu.Raw) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, nasPdu.Raw)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Label != "UserLocationInformation" {
		t.Errorf("expected ID=UserLocationInformation, got %s", item2.ID.Label)
	}

	if item2.ID.Value != ngapType.ProtocolIEIDUserLocationInformation {
		t.Errorf("expected ID value=116, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item2.Criticality.Value)
	}

	userLocationInfo, ok := item2.Value.(ngap.UserLocationInformation)
	if !ok {
		t.Fatalf("expected UserLocationInformation to be of type ngap.UserLocationInformation, got %T", item2.Value)
	}

	if userLocationInfo.NR == nil {
		t.Fatalf("expected NR, got nil")
	}

	if userLocationInfo.NR.TAI.TAC != "000001" {
		t.Errorf("expected TAC=000001, got %s", userLocationInfo.NR.TAI.TAC)
	}

	if userLocationInfo.NR.TAI.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", userLocationInfo.NR.TAI.PLMNID.Mcc)
	}

	if userLocationInfo.NR.TAI.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", userLocationInfo.NR.TAI.PLMNID.Mnc)
	}

	if userLocationInfo.NR.TimeStamp == nil {
		t.Fatalf("expected TimeStamp, got nil")
	}

	if *userLocationInfo.NR.TimeStamp != "2025-10-14T20:47:06Z" {
		t.Errorf("expected TimeStamp=2025-10-14T20:47:06Z, got %s", *userLocationInfo.NR.TimeStamp)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Label != "RRCEstablishmentCause" {
		t.Errorf("expected ID=RRCEstablishmentCause, got %s", item3.ID.Label)
	}

	if item3.ID.Value != ngapType.ProtocolIEIDRRCEstablishmentCause {
		t.Errorf("expected ID value=90, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item3.Criticality.Value)
	}

	rrcEstabCause, ok := item3.Value.(utils.EnumField[uint64])
	if !ok {
		t.Fatalf("expected RRCEstablishmentCause to be of type ngap.EnumField, got %T", item3.Value)
	}

	if rrcEstabCause.Label != "MoSignalling" {
		t.Errorf("expected RRCEstablishmentCause=MoSignalling, got %s", rrcEstabCause.Label)
	}

	if rrcEstabCause.Value != uint64(ngapType.RRCEstablishmentCausePresentMoSignalling) {
		t.Errorf("expected RRCEstablishmentCause value=3, got %d", rrcEstabCause.Value)
	}

	item4 := ngapMsg.Value.IEs[4]

	if item4.ID.Label != "UEContextRequest" {
		t.Errorf("expected ID=UEContextRequest, got %s", item4.ID.Label)
	}

	if item4.ID.Value != ngapType.ProtocolIEIDUEContextRequest {
		t.Errorf("expected ID value=112, got %d", item4.ID.Value)
	}

	if item4.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item4.Criticality)
	}

	if item4.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item4.Criticality.Value)
	}

	ueContextRequest, ok := item4.Value.(utils.EnumField[uint64])
	if !ok {
		t.Fatalf("expected UEContextRequest to be of type ngap.EnumField, got %T", item4.Value)
	}

	if ueContextRequest.Label != "Requested" {
		t.Errorf("expected UEContextRequest=Requested, got %v", ueContextRequest.Label)
	}

	if ueContextRequest.Value != uint64(ngapType.UEContextRequestPresentRequested) {
		t.Errorf("expected UEContextRequest value=0, got %d", ueContextRequest.Value)
	}
}
