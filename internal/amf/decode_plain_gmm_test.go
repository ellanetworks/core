package amf

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/nas"
)

// TestDecodePlainGmm_NeverRejectsWhatFree5gcAccepts is the safety invariant: the
// fgs validator must never reject a plain NAS body that free5gc's PlainNasDecode
// accepts — otherwise a valid message would draw a 5GMM STATUS instead of being
// processed, breaking a genuine UE. (The reverse is allowed: the validator accepts
// a defined type it does not deeply parse — downlink or header-only — on its header;
// a malformed such message is dropped rather than answered, and is never processed.)
func TestDecodePlainGmm_NeverRejectsWhatFree5gcAccepts(t *testing.T) {
	wellFormed := [][]byte{
		encodePlainRegistrationRequest(t),
		encodePlainServiceRequest(t),
		encodePlainULNasTransport(t),
		encodePlainDeregistrationRequest(t),
		{0x7e, 0x00, nas.MsgTypeStatus5GMM, 0x6f},                               // 5GMM status
		{0x7e, 0x00, nas.MsgTypeRegistrationComplete},                           // header-only uplink
		{0x7e, 0x00, nas.MsgTypeConfigurationUpdateComplete},                    // header-only uplink
		{0x7e, 0x00, nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration}, // header-only uplink
		{0x7e, 0x00, nas.MsgTypeRegistrationAccept, 0x00, 0x00, 0x00, 0x00},     // downlink, well-formed enough
	}

	for i, body := range wellFormed {
		m := new(nas.Message)

		cp := append([]byte(nil), body...)
		if refErr := m.PlainNasDecode(&cp); refErr != nil {
			continue // only well-formed (free5gc-accepted) inputs constrain the invariant
		}

		if _, _, gotErr := DecodePlainGmm(body); gotErr != nil {
			t.Errorf("case %d (% x): free5gc accepts but DecodePlainGmm rejects: %v", i, body, gotErr)
		}
	}
}

// TestDecodePlainGmm_RejectsUnknownAndMalformed confirms the validator rejects the
// inputs the STATUS #96/#97 contract depends on: an unassigned type and a truncated
// body of a type Ella parses.
func TestDecodePlainGmm_RejectsUnknownAndMalformed(t *testing.T) {
	reject := [][]byte{
		nil,
		{0x7e, 0x00},       // too short to carry a type
		{0x7e, 0x00, 0xff}, // unassigned type
		{0x7e, 0x00, uint8(fgs.MsgRegistrationRequest)}, // truncated
		{0x7e, 0x00, uint8(fgs.MsgServiceRequest)},      // truncated
		{0x7e, 0x00, uint8(fgs.MsgULNASTransport)},      // truncated
		{0x99, 0x00, 0x41},                              // disallowed EPD
	}

	for i, body := range reject {
		if _, _, err := DecodePlainGmm(body); err == nil {
			t.Errorf("case %d (% x): expected DecodePlainGmm to reject, but it accepted", i, body)
		}
	}
}
