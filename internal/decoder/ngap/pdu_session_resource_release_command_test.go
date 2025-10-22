package ngap_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_PDUSessionResourceReleaseCommand(t *testing.T) {
	const message = "ABwAMQAABAAKAAIAlABVAAIAAQAmQBUUfgKGQUZ3A34AaAEABS4BBNMAEgEATwAFAAABARA="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "PDUSessionResourceReleaseCommand" {
		t.Errorf("expected MessageType=PDUSessionResourceReleaseCommand, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "PDUSessionResourceRelease" {
		t.Errorf("expected ProcedureCode=PDUSessionResourceRelease, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodePDUSessionResourceRelease {
		t.Errorf("expected ProcedureCode value=28, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngapMsg.Criticality.Value)
	}

	if len(ngapMsg.Value.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %s", item0.ID.Label)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDAMFUENGAPID {
		t.Errorf("expected ID value=10, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	amfUeNgapID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMFUENGAPID value type=int64, got %T", item0.Value)
	}

	if amfUeNgapID != 148 {
		t.Errorf("expected AMFUENGAPID=148, got %d", amfUeNgapID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item1.ID.Label)
	}

	if item1.ID.Value != ngapType.ProtocolIEIDRANUENGAPID {
		t.Errorf("expected ID value=85, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item1.Criticality.Value)
	}

	ranUeNgapID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RANUENGAPID value type=int64, got %T", item1.Value)
	}

	if ranUeNgapID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", ranUeNgapID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Label != "NASPDU" {
		t.Errorf("expected ID=NASPDU, got %s", item2.ID.Label)
	}

	if item2.ID.Value != ngapType.ProtocolIEIDNASPDU {
		t.Errorf("expected ID value=21, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item2.Criticality.Value)
	}

	nasPdu, ok := item2.Value.(ngap.NASPDU)
	if !ok {
		t.Fatalf("expected NASPDU value type=ngap.NASPDU, got %T", item2.Value)
	}

	expectedNASPDU := "fgKGQUZ3A34AaAEABS4BBNMAEgE="
	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(nasPdu.Raw) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, nasPdu.Raw)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Label != "PDUSessionResourceToReleaseListRelCmd" {
		t.Errorf("expected ID=PDUSessionResourceToReleaseListRelCmd, got %s", item3.ID.Label)
	}

	if item3.ID.Value != ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd {
		t.Errorf("expected ID value=132, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item3.Criticality.Value)
	}

	pduSessionList, ok := item3.Value.([]ngap.PDUSessionResourceToReleaseListRelCmd)
	if !ok {
		t.Fatalf("expected PDUSessionResourceToReleaseListRelCmd value type=[]PDUSessionResourceToReleaseListRelCmd, got %T", item3.Value)
	}

	if len(pduSessionList) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceToReleaseListRelCmd, got %d", len(pduSessionList))
	}

	if pduSessionList[0].PDUSessionID != 1 {
		t.Errorf("expected PDUSessionID=1, got %d", pduSessionList[0].PDUSessionID)
	}

	expectedTransfer := "EA=="
	expectedTransferRaw, err := decodeB64(expectedTransfer)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if !bytes.Equal(pduSessionList[0].PDUSessionResourceReleaseCommandTransfer, expectedTransferRaw) {
		t.Errorf("expected PDUSessionResourceReleaseCommandTransfer=%s, got %s", expectedTransfer, encodeB64(pduSessionList[0].PDUSessionResourceReleaseCommandTransfer))
	}
}
