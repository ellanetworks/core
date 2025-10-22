package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_UEContextReleaseRequest(t *testing.T) {
	const message = "ACpAHAAABAAKAAIAGwBVAAIAGwCFAAMAAAEAD0ACBUA="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "UEContextReleaseRequest" {
		t.Errorf("expected MessageType=UEContextReleaseRequest, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "UEContextReleaseRequest" {
		t.Errorf("expected ProcedureCode=UEContextReleaseRequest, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeUEContextReleaseRequest {
		t.Errorf("expected ProcedureCode value=42, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", ngapMsg.Criticality.Value)
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
		t.Errorf("expected AMFUENGAPID value type=int64, got %T", item0.Value)
	}

	if amfUeNgapID != 27 {
		t.Errorf("expected AMFUENGAPID=12345, got %d", amfUeNgapID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item1.ID.Label)
	}

	if item1.ID.Value != ngapType.ProtocolIEIDRANUENGAPID {
		t.Errorf("expected ID value=11, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item1.Criticality.Value)
	}

	ranUeNgapID, ok := item1.Value.(int64)
	if !ok {
		t.Errorf("expected RANUENGAPID value type=int64, got %T", item1.Value)
	}

	if ranUeNgapID != 27 {
		t.Errorf("expected RANUENGAPID=27, got %d", ranUeNgapID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Label != "PDUSessionResourceListCxtRelReq" {
		t.Errorf("expected ID=PDUSessionResourceListCxtRelReq, got %s", item2.ID.Label)
	}

	if item2.ID.Value != ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq {
		t.Errorf("expected ID value=133, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item2.Criticality.Value)
	}

	pduSessionList, ok := item2.Value.([]ngap.PDUSessionResourceListCxtRelReq)
	if !ok {
		t.Fatalf("expected PDUSessionResourceListCxtRelReq value type=[]PDUSessionResourceListCxtRelReq, got %T", item2.Value)
	}

	if len(pduSessionList) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceListCxtRelReq, got %d", len(pduSessionList))
	}

	if pduSessionList[0].PDUSessionID != 1 {
		t.Errorf("expected PDUSessionID=1, got %d", pduSessionList[0].PDUSessionID)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Label != "Cause" {
		t.Errorf("expected ID=Cause, got %s", item3.ID.Label)
	}

	if item3.ID.Value != ngapType.ProtocolIEIDCause {
		t.Errorf("expected ID value=15, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item3.Criticality.Value)
	}

	cause, ok := item3.Value.(utils.EnumField[uint64])
	if !ok {
		t.Fatalf("expected Cause value type=utils.EnumField[uint64], got %T", item3.Value)
	}

	if cause.Label != "RadioConnectionWithUeLost" {
		t.Errorf("expected Cause=RadioConnectionWithUeLost, got %s", cause.Label)
	}

	if cause.Value != uint64(ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost) {
		t.Errorf("expected Cause value=21, got %d", cause.Value)
	}
}
