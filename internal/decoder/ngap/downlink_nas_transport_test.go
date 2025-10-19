package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
)

func TestDecodeNGAPMessage_DownlinkNASTransport(t *testing.T) {
	const message = "AARAPgAAAwAKAAIAAQBVAAIAAQAmACsqfgBWAAIAACEaBwCjbSa9vkiAkRdky8+5IBBH2jhAU2SAAE2CgCRBSs2H"

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

	if string(item2.NASPDU.Raw) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, item2.NASPDU.Raw)
	}
}
