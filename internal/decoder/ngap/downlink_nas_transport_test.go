// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_DownlinkNASTransport(t *testing.T) {
	const message = "AARAPgAAAwAKAAIAAQBVAAIAAQAmACsqfgBWAAIAACEaBwCjbSa9vkiAkRdky8+5IBBH2jhAU2SAAE2CgCRBSs2H"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcDownlinkNASTransport) {
		t.Errorf("expected ProcedureCode=DownlinkNASTransport, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcDownlinkNASTransport) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcDownlinkNASTransport)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 3 {
		t.Errorf("expected 3 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDAMFUENGAPID) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDAMFUENGAPID)
	}

	if item0.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item0.Criticality)
	}

	amfUENGAPID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMF-UE-NGAP-ID to be of type int64, got %T", item0.Value)
	}

	if amfUENGAPID != 1 {
		t.Errorf("expected AMF-UE-NGAP-ID=1, got %d", amfUENGAPID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Value != int64(lib.IDRANUENGAPID) {
		t.Errorf("IE id = %d, want %d", item1.ID.Value, lib.IDRANUENGAPID)
	}

	if item1.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item1.Criticality)
	}

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RAN-UE-NGAP-ID to be of type *int64, got %T", item1.Value)
	}

	if ranUENGAPID != 1 {
		t.Errorf("expected RAN-UE-NGAP-ID=1, got %d", ranUENGAPID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDNASPDU) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDNASPDU)
	}

	if item2.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item2.Criticality)
	}

	nasPdu, ok := item2.Value.(NASPDU)
	if !ok {
		t.Fatalf("expected NAS-PDU to be of type NAS-PDU, got %T", item2.Value)
	}

	expectedNASPDU := "fgBWAAIAACEaBwCjbSa9vkiAkRdky8+5IBBH2jhAU2SAAE2CgCRBSs2H"

	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	expectedHex := hex.EncodeToString(expectedNASPDUraw)
	if nasPdu.RawHex != expectedHex {
		t.Errorf("expected RawHex=%s, got %s", expectedHex, nasPdu.RawHex)
	}
}
