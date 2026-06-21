// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package eps decodes EPS NAS (4G EMM/ESM) messages carried in S1AP into a
// structured, JSON-friendly view for the UI events drawer. It reads plaintext only:
// a ciphered message is reported with its security header and an encrypted flag.
package eps

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type SecurityHeader struct {
	ProtocolDiscriminator     utils.EnumField[uint64] `json:"protocol_discriminator"`
	SecurityHeaderType        utils.EnumField[uint64] `json:"security_header_type"`
	MessageAuthenticationCode uint32                  `json:"authentication_code,omitempty"`
	SequenceNumber            uint8                   `json:"sequence_number,omitempty"`
}

// NASMessage is the decoded EPS NAS message. Its JSON shape matches the 5GS
// decoder's (security_header, then an emm_message or esm_message, with an
// encrypted flag) so the shared UI renderer treats both alike.
type NASMessage struct {
	SecurityHeader SecurityHeader `json:"security_header"`
	EMMMessage     *EMMMessage    `json:"emm_message,omitempty"`
	ESMMessage     *ESMMessage    `json:"esm_message,omitempty"`

	Encrypted bool   `json:"encrypted"`
	Error     string `json:"error,omitempty"`
}

// DecodeEPSNASMessage decodes a raw EPS NAS PDU. Decode problems are reported in
// the returned message rather than as a Go error, matching the 5GS decoder.
func DecodeEPSNASMessage(raw []byte) *NASMessage {
	if len(raw) < 2 {
		return &NASMessage{Error: "NAS message too short"}
	}

	pd := raw[0] & 0x0f
	sht := raw[0] >> 4

	msg := &NASMessage{
		SecurityHeader: SecurityHeader{
			ProtocolDiscriminator: pdToEnum(pd),
			SecurityHeaderType:    shtToEnum(sht),
		},
	}

	// Only EMM messages carry the security wrapper; ESM messages are always
	// plain (they ride integrity-protected inside an EMM message or container).
	if pd != eps.PDEMM || sht == uint8(eps.SHTPlain) {
		return decodePlain(msg, raw)
	}

	if sht == uint8(eps.SHTServiceRequest) {
		return decodeServiceRequest(msg, raw)
	}

	// Security-protected EMM wrapper (TS 24.301 §9.1.1): SHT|PD octet, 4-octet
	// MAC, 1-octet sequence number, then the inner NAS message.
	const wrapper = 6

	if len(raw) < wrapper {
		msg.Error = "security-protected NAS message too short"
		return msg
	}

	msg.SecurityHeader.MessageAuthenticationCode = binary.BigEndian.Uint32(raw[1:5])
	msg.SecurityHeader.SequenceNumber = raw[5]

	// Ciphered (types 2 and 4) cannot be decoded without the NAS keys.
	if sht == uint8(eps.SHTIntegrityProtectedCiphered) || sht == uint8(eps.SHTIntegrityProtectedCipheredNewContext) {
		msg.Encrypted = true
		return msg
	}

	return decodePlain(msg, raw[wrapper:])
}

func decodePlain(msg *NASMessage, b []byte) *NASMessage {
	if len(b) < 2 {
		msg.Error = "plain NAS message too short"
		return msg
	}

	switch b[0] & 0x0f {
	case eps.PDEMM:
		msg.EMMMessage = buildEMMMessage(b)
	case eps.PDESM:
		msg.ESMMessage = buildESMMessage(b)
	default:
		msg.Error = fmt.Sprintf("unknown protocol discriminator %#x", b[0]&0x0f)
	}

	return msg
}

func pdToEnum(pd uint8) utils.EnumField[uint64] {
	switch pd {
	case eps.PDEMM:
		return utils.MakeEnum(uint64(pd), "EPS Mobility Management", false)
	case eps.PDESM:
		return utils.MakeEnum(uint64(pd), "EPS Session Management", false)
	default:
		return utils.MakeEnum(uint64(pd), "", true)
	}
}

func shtToEnum(sht uint8) utils.EnumField[uint64] {
	names := map[uint8]string{
		uint8(eps.SHTPlain):                                "Plain NAS",
		uint8(eps.SHTIntegrityProtected):                   "Integrity protected",
		uint8(eps.SHTIntegrityProtectedCiphered):           "Integrity protected and ciphered",
		uint8(eps.SHTIntegrityProtectedNewContext):         "Integrity protected with new EPS security context",
		uint8(eps.SHTIntegrityProtectedCipheredNewContext): "Integrity protected and ciphered with new EPS security context",
		uint8(eps.SHTServiceRequest):                       "Service request",
	}

	name, ok := names[sht]

	return utils.MakeEnum(uint64(sht), name, !ok)
}
