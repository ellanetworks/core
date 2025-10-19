package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
)

func TestDecodeNGAPMessage_InitialUEMessage(t *testing.T) {
	const message = "AA9ASAAABQBVAAIAAQAmABoZfgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8AB5ABNQAPEQAAAAAQAA8RAAAAHsmTVKAFpAARgAcEABAA=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := ngap.DecodeNGAPMessage(raw)
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

	if string(item1.NASPDU.Raw) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, item1.NASPDU.Raw)
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
