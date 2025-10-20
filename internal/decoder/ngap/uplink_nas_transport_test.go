package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_UplinkNASTransport(t *testing.T) {
	const message = "AC5APwAABAAKAAIAAQBVAAIAAQAmABUUfgLpGbfKA34AZwEABS4BANZREgEAeUATUADxEAAAAAEAAPEQAAAB7JlGUQ=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg, err := ngap.DecodeNGAPMessage(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngapMsg.InitiatingMessage == nil {
		t.Fatalf("expected InitiatingMessage, got nil")
	}

	if ngapMsg.InitiatingMessage.ProcedureCode.Label != "UplinkNASTransport" {
		t.Errorf("expected ProcedureCode=UplinkNASTransport, got %v", ngapMsg.InitiatingMessage.ProcedureCode)
	}

	if ngapMsg.InitiatingMessage.ProcedureCode.Value != int(ngapType.ProcedureCodeUplinkNASTransport) {
		t.Errorf("expected ProcedureCode value=2, got %d", ngapMsg.InitiatingMessage.ProcedureCode.Value)
	}

	if ngapMsg.InitiatingMessage.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore (1), got %v", ngapMsg.InitiatingMessage.Criticality)
	}

	if ngapMsg.InitiatingMessage.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", ngapMsg.InitiatingMessage.Criticality.Value)
	}

	if ngapMsg.InitiatingMessage.Value.UplinkNASTransport == nil {
		t.Fatalf("expected UplinkNASTransport, got nil")
	}

	if len(ngapMsg.InitiatingMessage.Value.UplinkNASTransport.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngapMsg.InitiatingMessage.Value.UplinkNASTransport.IEs))
	}

	item0 := ngapMsg.InitiatingMessage.Value.UplinkNASTransport.IEs[0]

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %s", item0.ID.Label)
	}

	if item0.ID.Value != int(ngapType.ProtocolIEIDAMFUENGAPID) {
		t.Errorf("expected ID value=10, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject (0), got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 1 {
		t.Errorf("expected AMFUENGAPID=1, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngapMsg.InitiatingMessage.Value.UplinkNASTransport.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item1.ID.Label)
	}

	if item1.ID.Value != int(ngapType.ProtocolIEIDRANUENGAPID) {
		t.Errorf("expected ID value=85, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject (0), got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item1.Criticality.Value)
	}

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RANUENGAPID to be of type int64, got %T", item1.Value)
	}

	if ranUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", ranUENGAPID)
	}

	item2 := ngapMsg.InitiatingMessage.Value.UplinkNASTransport.IEs[2]

	if item2.ID.Label != "NASPDU" {
		t.Errorf("expected ID=NASPDU, got %s", item2.ID.Label)
	}

	if item2.ID.Value != int(ngapType.ProtocolIEIDNASPDU) {
		t.Errorf("expected ID value=38, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject (0), got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item2.Criticality.Value)
	}

	nasPdu, ok := item2.Value.(ngap.NASPDU)
	if !ok {
		t.Fatalf("expected NASPDU to be of type ngap.NASPDU, got %T", item2.Value)
	}

	expectedNASPDU := "fgLpGbfKA34AZwEABS4BANZREgE="
	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(nasPdu.Raw) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, nasPdu.Raw)
	}

	item3 := ngapMsg.InitiatingMessage.Value.UplinkNASTransport.IEs[3]

	if item3.ID.Label != "UserLocationInformation" {
		t.Errorf("expected ID=UserLocationInformation, got %s", item3.ID.Label)
	}

	if item3.ID.Value != int(ngapType.ProtocolIEIDUserLocationInformation) {
		t.Errorf("expected ID value=121, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore (1), got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item3.Criticality.Value)
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
