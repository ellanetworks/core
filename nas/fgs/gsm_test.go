// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"net/netip"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestSMBuildersWireBytes(t *testing.T) {
	cases := []struct {
		name string
		got  []byte
		want []byte
	}{
		{
			"GSMStatus",
			mustMarshal(t, (&GSMStatus{PDUSessionID: 5, PTI: 1, Cause: GSMCausePTIMismatch}).MarshalBinary),
			[]byte{uint8(EPD5GSM), 5, 1, uint8(MsgGSMStatus), uint8(GSMCausePTIMismatch)},
		},
		{
			"EstablishmentReject",
			mustMarshal(t, (&PDUSessionEstablishmentReject{PDUSessionID: 5, PTI: 1, Cause: GSMCauseRequestRejectedUnspecified}).MarshalBinary),
			[]byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionEstablishmentReject), uint8(GSMCauseRequestRejectedUnspecified)},
		},
		{
			"ModificationReject",
			mustMarshal(t, (&PDUSessionModificationReject{PDUSessionID: 5, PTI: 1, Cause: GSMCauseInsufficientResources}).MarshalBinary),
			[]byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionModificationReject), uint8(GSMCauseInsufficientResources)},
		},
		{
			"ReleaseCommand",
			mustMarshal(t, (&PDUSessionReleaseCommand{PDUSessionID: 5, PTI: 1, Cause: GSMCauseRegularDeactivation}).MarshalBinary),
			[]byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionReleaseCommand), uint8(GSMCauseRegularDeactivation)},
		},
	}

	for _, c := range cases {
		if !bytes.Equal(c.got, c.want) {
			t.Errorf("%s = %#x, want %#x", c.name, c.got, c.want)
		}
	}
}

func TestStatus5GSMRoundTrip(t *testing.T) {
	orig := &GSMStatus{PDUSessionID: 3, PTI: 7, Cause: GSMCauseProtocolErrorUnspecified}

	b := mustMarshal(t, orig.MarshalBinary)

	got, err := ParseGSMStatus(b)
	if err != nil {
		t.Fatalf("ParseGSMStatus: %v", err)
	}

	if !reflect.DeepEqual(got, orig) {
		t.Fatalf("round-trip = %+v, want %+v", got, orig)
	}
}

func TestParseEstablishmentRequest(t *testing.T) {
	// header | integrity-max-data-rate(2) | PDU session type (0x9, IPv4) |
	// SSC mode (0xA, mode 1) | 5GSM capability (0x28 TLV, 1 octet) |
	// always-on requested (0xB, present)
	b := []byte{
		uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionEstablishmentRequest),
		0xFF, 0xFF,
		uint8(0x90 | PDUSessionTypeIPv4),
		uint8(0xA0 | SSCMode1),
		iei5GSMCapability, 0x01, 0x03,
		0xB0 | 0x01,
	}

	req, err := ParsePDUSessionEstablishmentRequest(b)
	if err != nil {
		t.Fatalf("ParsePDUSessionEstablishmentRequest: %v", err)
	}

	if req.PDUSessionID != 5 || req.PTI != 1 {
		t.Errorf("header psi=%d pti=%d, want 5/1", req.PDUSessionID, req.PTI)
	}

	if req.IntegrityProtMaxDataRate != [2]byte{0xFF, 0xFF} {
		t.Errorf("integrity max data rate = %#x, want ffff", req.IntegrityProtMaxDataRate)
	}

	// always-on must be reached even though a full-octet IE (0x28) precedes it.
	if !req.AlwaysOnRequested {
		t.Error("AlwaysOnRequested = false, want true")
	}

	if req.PDUSessionType == nil || *req.PDUSessionType != PDUSessionTypeIPv4 {
		t.Errorf("PDUSessionType = %v, want IPv4", req.PDUSessionType)
	}

	if req.SSCMode == nil || *req.SSCMode != SSCMode1 {
		t.Errorf("SSCMode = %v, want SSC mode 1", req.SSCMode)
	}

	if req.GSMCapability == nil || !req.GSMCapability.RqoS || !req.GSMCapability.MH6PDU {
		t.Errorf("GSMCapability = %+v, want reflective QoS and multi-homed IPv6", req.GSMCapability)
	}

	round, err := req.MarshalBinary()
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}

	if !bytes.Equal(round, b) {
		t.Errorf("re-encode = % x, want % x", round, b)
	}
}

// TestParseEstablishmentAcceptRQTimerNoDesync guards the RQ timer value IE
// length: its value is a single GPRS-timer-3 octet (TS 24.008 §10.5.7.4a), so
// the IE following it (here S-NSSAI) must still parse. A wrong value length
// over-consumes and desyncs every subsequent IE.
func TestParseEstablishmentAcceptRQTimerNoDesync(t *testing.T) {
	b := []byte{
		uint8(EPD5GSM), 0x01, 0x00, uint8(MsgPDUSessionEstablishmentAccept),
		0x11,       // SSC mode 1 / PDU session type IPv4
		0x00, 0x00, // QoS rules (LV-E), empty
		0x06, 0x06, 0x00, 0x64, 0x06, 0x00, 0x32, // Session-AMBR (LV): 100 Mbps down, 50 Mbps up
		0x56, 0x05, // RQ timer value (TV): IEI + one GPRS-timer-3 octet
		0x22, 0x01, 0x01, // S-NSSAI (TLV): IEI, length 1, SST=1
	}

	m, err := ParsePDUSessionEstablishmentAccept(b)
	if err != nil {
		t.Fatalf("ParsePDUSessionEstablishmentAccept: %v", err)
	}

	if m.RQTimer == nil || m.RQTimer.Unit != nas.GPRSTimer3Unit10Minutes || m.RQTimer.Value != 5 {
		t.Errorf("RQTimer = %v, want 5 × 10 minutes", m.RQTimer)
	}

	if m.SNSSAI == nil || m.SNSSAI.SST != 1 {
		t.Errorf("S-NSSAI after RQ timer = %+v, want SST 1 (a wrong RQ-timer length desynced the walk)", m.SNSSAI)
	}
}

func TestParseReleaseWithOptionalCause(t *testing.T) {
	reqBytes := []byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionReleaseRequest), iei5GSMCause, uint8(GSMCauseRegularDeactivation)}

	req, err := ParsePDUSessionReleaseRequest(reqBytes)
	if err != nil {
		t.Fatalf("ParsePDUSessionReleaseRequest: %v", err)
	}

	if req.Cause == nil || *req.Cause != GSMCauseRegularDeactivation {
		t.Errorf("release request cause = %v, want RegularDeactivation", req.Cause)
	}

	// Release complete with no optional cause.
	compBytes := []byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionReleaseComplete)}

	comp, err := ParsePDUSessionReleaseComplete(compBytes)
	if err != nil {
		t.Fatalf("ParsePDUSessionReleaseComplete: %v", err)
	}

	if comp.Cause != nil {
		t.Errorf("release complete cause = %v, want nil", comp.Cause)
	}
}

func TestParseModification(t *testing.T) {
	comp, err := ParsePDUSessionModificationComplete([]byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionModificationComplete)})
	if err != nil {
		t.Fatalf("ParsePDUSessionModificationComplete: %v", err)
	}

	if comp.PDUSessionID != 5 || comp.PTI != 1 {
		t.Errorf("modification complete psi=%d pti=%d, want 5/1", comp.PDUSessionID, comp.PTI)
	}

	rej, err := ParsePDUSessionModificationCommandReject(
		[]byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionModificationCmdReject), uint8(GSMCauseInvalidPTIValue)})
	if err != nil {
		t.Fatalf("ParsePDUSessionModificationCommandReject: %v", err)
	}

	if rej.Cause != GSMCauseInvalidPTIValue {
		t.Errorf("command reject cause = %#x, want %#x", rej.Cause, GSMCauseInvalidPTIValue)
	}
}

func TestParseRejectsWrongMessageType(t *testing.T) {
	// A 5GSM STATUS body parsed as a release request must fail on the type.
	if _, err := ParsePDUSessionReleaseRequest([]byte{uint8(EPD5GSM), 5, 1, uint8(MsgGSMStatus)}); err == nil {
		t.Error("expected wrong-message-type error, got nil")
	}
}

func mustMarshal(t *testing.T, fn func() ([]byte, error)) []byte {
	t.Helper()

	b, err := fn()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	return b
}

func TestPDUSessionEstablishmentRejectRoundTrip(t *testing.T) {
	in := &PDUSessionEstablishmentReject{PDUSessionID: 5, PTI: 1, Cause: 0x1b}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	out, err := ParsePDUSessionEstablishmentReject(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if out.PDUSessionID != 5 || out.PTI != 1 || out.Cause != 0x1b {
		t.Fatalf("round-trip mismatch: got %+v", out)
	}
}

func TestPDUSessionReleaseCommandRoundTrip(t *testing.T) {
	in := &PDUSessionReleaseCommand{PDUSessionID: 5, PTI: 0, Cause: 0x27}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	out, err := ParsePDUSessionReleaseCommand(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if out.PDUSessionID != 5 || out.PTI != 0 || out.Cause != 0x27 {
		t.Fatalf("round-trip mismatch: got %+v", out)
	}
}

func TestProtocolConfigurationOptionsRoundTrip(t *testing.T) {
	dns := [][]byte{{8, 8, 8, 8}, {0x20, 0x01, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}}

	pco := mustBytes(nas.NewProtocolConfigurationOptions(dns, 1400).MarshalBinary())

	got, err := nas.ParseProtocolConfigurationOptions(pco, nas.PCONetworkToMS)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	gotDNS := got.DNSServers()

	gotMTU, ok := got.IPv4LinkMTU()
	if !ok || gotMTU != 1400 || len(gotDNS) != 2 ||
		gotDNS[0] != netip.AddrFrom4([4]byte(dns[0])) || gotDNS[1] != netip.AddrFrom16([16]byte(dns[1])) {
		t.Fatalf("round-trip mismatch: mtu=%d dns=%v", gotMTU, gotDNS)
	}

	// The IE re-encodes byte-for-byte, including containers the accessors ignore.
	again, err := got.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if !bytes.Equal(again, pco) {
		t.Fatalf("re-encode = %#x, want %#x", again, pco)
	}
}
