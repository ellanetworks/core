package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_DownlinkNASTransport(t *testing.T) {
	const message = "AARAPgAAAwAKAAIAAQBVAAIAAQAmACsqfgBWAAIAACEaBwCjbSa9vkiAkRdky8+5IBBH2jhAU2SAAE2CgCRBSs2H"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "DownlinkNASTransport" {
		t.Errorf("expected MessageType=DownlinkNASTransport, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "DownlinkNASTransport" {
		t.Errorf("expected ProcedureCode=DownlinkNASTransport, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeDownlinkNASTransport {
		t.Errorf("expected ProcedureCode value=3, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", ngapMsg.Criticality.Value)
	}

	if len(ngapMsg.Value.IEs) != 3 {
		t.Errorf("expected 3 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %s", item0.ID.Label)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDAMFUENGAPID {
		t.Errorf("expected ID value=85, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	amfUENGAPID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMFUENGAPID to be of type int64, got %T", item0.Value)
	}

	if amfUENGAPID != 1 {
		t.Errorf("expected AMFUENGAPID=1, got %d", amfUENGAPID)
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

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RANUENGAPID to be of type *int64, got %T", item1.Value)
	}

	if ranUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", ranUENGAPID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Label != "NASPDU" {
		t.Errorf("expected ID=NASPDU, got %v", item2.ID)
	}

	if item2.ID.Value != ngapType.ProtocolIEIDNASPDU {
		t.Errorf("expected ID value=38, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item2.Criticality.Value)
	}

	nasPdu, ok := item2.Value.(ngap.NASPDU)
	if !ok {
		t.Fatalf("expected NASPDU to be of type NASPDU, got %T", item2.Value)
	}

	expectedNASPDU := "fgBWAAIAACEaBwCjbSa9vkiAkRdky8+5IBBH2jhAU2SAAE2CgCRBSs2H"
	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(nasPdu.Raw) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, nasPdu.Raw)
	}
}
