package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestDecodeNGAPMessage_UEContextReleaseComplete(t *testing.T) {
	const message = "ICkAKQAABAAKQAIAkgBVQAIAnwB5QA9AAPEQABI0UBAA8RAAAAEAPAADAAAB"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "SuccessfulOutcome" {
		t.Errorf("expected PDUType=SuccessfulOutcome, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "UEContextReleaseComplete" {
		t.Errorf("expected MessageType=UEContextReleaseComplete, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "UEContextRelease" {
		t.Errorf("expected ProcedureCode=UEContextRelease, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeUEContextRelease {
		t.Errorf("expected ProcedureCode value=41, got %d", ngapMsg.ProcedureCode.Value)
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

	if item0.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item0.Criticality.Value)
	}

	amfUeNgapID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMFUENGAPID type=uint64, got %T", item0.Value)
	}

	if amfUeNgapID != 146 {
		t.Errorf("expected AMFUENGAPID=146, got %d", amfUeNgapID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item1.ID.Label)
	}

	if item1.ID.Value != ngapType.ProtocolIEIDRANUENGAPID {
		t.Errorf("expected ID value=11, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item1.Criticality.Value)
	}

	ranUeNgapID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RANUENGAPID type=uint64, got %T", item1.Value)
	}

	if ranUeNgapID != 159 {
		t.Errorf("expected RANUENGAPID=159, got %d", ranUeNgapID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Label != "UserLocationInformation" {
		t.Errorf("expected ID=UserLocationInformation, got %s", item2.ID.Label)
	}

	if item2.ID.Value != ngapType.ProtocolIEIDUserLocationInformation {
		t.Errorf("expected ID value=121, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item2.Criticality.Value)
	}

	userLocationInfo, ok := item2.Value.(ngap.UserLocationInformation)
	if !ok {
		t.Fatalf("expected UserLocationInformation value type=ngap.UserLocationInformation, got %T", item2.Value)
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
		t.Errorf("expected NRCGI.PLMNID.MCC=001, got %s", userLocationInfo.NR.NRCGI.PLMNID.Mcc)
	}

	if userLocationInfo.NR.NRCGI.PLMNID.Mnc != "01" {
		t.Errorf("expected NRCGI.PLMNID.MNC=01, got %s", userLocationInfo.NR.NRCGI.PLMNID.Mnc)
	}

	if userLocationInfo.NR.NRCGI.NRCellIdentity != "001234501" {
		t.Errorf("expected NRCGI.NRCellIdentity=001234501, got %v", userLocationInfo.NR.NRCGI.NRCellIdentity)
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

	if item3.ID.Label != "PDUSessionResourceListCxtRelCpl" {
		t.Errorf("expected ID=PDUSessionResourceListCxtRelCpl, got %s", item3.ID.Label)
	}

	if item3.ID.Value != ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl {
		t.Errorf("expected ID value=60, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item3.Criticality.Value)
	}

	pduSessionList, ok := item3.Value.([]ngap.PDUSessionResourceItemCxtRelCpl)
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
