// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"strings"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func TestDecodeInitialUEMessage(t *testing.T) {
	msg := decodeHex(t, "000c40610000060008000340032e001a002e2d17659a6d010d0748000bf699f910000101000000015807f07000001800805299f9100001570220005d0106e0c1004300060099f9100001006440080099f9100019b010008640013000600006004000000001")

	if msg.PDUType != "InitiatingMessage" || msg.ProcedureCode.Value != int64(s1ap.ProcInitialUEMessage) {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	// This live Initial UE Message carries an integrity-protected TAU Request,
	// which the embedded NAS decoder names in the summary.
	if msg.Summary != "Initial UE Message (eNB-UE 814, NAS=TRACKING AREA UPDATE REQUEST)" {
		t.Fatalf("summary = %q", msg.Summary)
	}

	if v := mustIE(t, msg, s1ap.IDENBUES1APID).Value; v != uint32(814) {
		t.Fatalf("eNB-UE-S1AP-ID = %v", v)
	}

	nas, ok := mustIE(t, msg, s1ap.IDNASPDU).Value.(NASPDU)
	if !ok || nas.Protocol != "NAS" || !strings.HasPrefix(nas.RawHex, "17659a6d") {
		t.Fatalf("NAS-PDU = %+v", mustIE(t, msg, s1ap.IDNASPDU).Value)
	}

	if nas.Decoded == nil || nas.Decoded.EMMMessage == nil ||
		nas.Decoded.EMMMessage.EMMHeader.MessageType.Value != int64(eps.MsgTrackingAreaUpdateRequest) {
		t.Fatalf("decoded NAS = %+v", nas.Decoded)
	}

	taiv := mustIE(t, msg, s1ap.IDTAIList).Value.(TAI)
	if taiv.PLMNID.Mcc != "999" || taiv.PLMNID.Mnc != "01" || taiv.TAC != 1 {
		t.Fatalf("TAI = %+v", taiv)
	}

	cgi := mustIE(t, msg, s1ap.IDEUTRANCGI).Value.(EUTRANCGI)
	if cgi.CellID != "0019b01" {
		t.Fatalf("EUTRAN-CGI cell = %q", cgi.CellID)
	}

	if mustIE(t, msg, s1ap.IDRRCEstablishmentCause).ValueType != "enum" {
		t.Fatal("RRC cause not an enum")
	}

	st := mustIE(t, msg, s1ap.IDSTMSI).Value.(STMSI)
	if st.MMEC != 1 || st.MTMSI != 1 {
		t.Fatalf("S-TMSI = %+v", st)
	}
}

func TestDecodeUplinkNASTransport(t *testing.T) {
	msg := decodeHex(t, "000d403800000500000002000200080003400332001a000e0d2774a88ff701128f7ddc4907f3006440080099f9100019b010004340060099f9100001")

	if msg.ProcedureCode.Value != int64(s1ap.ProcUplinkNASTransport) {
		t.Fatalf("proc = %q", msg.ProcedureCode.Label)
	}

	if mustIE(t, msg, s1ap.IDMMEUES1APID).Value != uint32(2) || mustIE(t, msg, s1ap.IDENBUES1APID).Value != uint32(818) {
		t.Fatal("UE id mismatch")
	}

	// This NAS-PDU is integrity-protected and ciphered (SHT=2), so the decoder
	// reports it encrypted rather than decoding the inner message.
	nas := mustIE(t, msg, s1ap.IDNASPDU).Value.(NASPDU)
	if !strings.HasPrefix(nas.RawHex, "2774a88f") {
		t.Fatalf("NAS raw_hex = %q", nas.RawHex)
	}

	if nas.Decoded == nil || !nas.Decoded.Encrypted {
		t.Fatalf("expected encrypted NAS, got %+v", nas.Decoded)
	}

	cgi := mustIE(t, msg, s1ap.IDEUTRANCGI).Value.(EUTRANCGI)
	if cgi.CellID != "0019b01" {
		t.Fatalf("EUTRAN-CGI cell = %q", cgi.CellID)
	}
}

func TestDecodeDownlinkNASTransport(t *testing.T) {
	msg := decodeHex(t, "000b402600000300000002000300080003400333001a001211277aacef53032d11d997029fd028cf59aa")

	if msg.ProcedureCode.Value != int64(s1ap.ProcDownlinkNASTransport) {
		t.Fatalf("proc = %q", msg.ProcedureCode.Label)
	}

	if mustIE(t, msg, s1ap.IDMMEUES1APID).Value != uint32(3) || mustIE(t, msg, s1ap.IDENBUES1APID).Value != uint32(819) {
		t.Fatal("UE id mismatch")
	}

	nas := mustIE(t, msg, s1ap.IDNASPDU).Value.(NASPDU)
	if nas.RawHex != "277aacef53032d11d997029fd028cf59aa" {
		t.Fatalf("NAS raw_hex = %q", nas.RawHex)
	}

	if nas.Decoded == nil || !nas.Decoded.Encrypted {
		t.Fatalf("expected encrypted NAS, got %+v", nas.Decoded)
	}
}
