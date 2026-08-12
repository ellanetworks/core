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
	ProtocolDiscriminator     utils.EnumField `json:"protocol_discriminator"`
	SecurityHeaderType        utils.EnumField `json:"security_header_type"`
	MessageAuthenticationCode uint32          `json:"authentication_code,omitempty"`
	SequenceNumber            uint8           `json:"sequence_number,omitempty"`
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
	// The lib accessors hide the header octet placement and guard the length.
	pd, err := eps.PeekProtocolDiscriminator(raw)
	if err != nil {
		return &NASMessage{Error: "NAS message too short"}
	}

	sht, err := eps.PeekSecurityHeaderType(raw)
	if err != nil {
		return &NASMessage{Error: "NAS message too short"}
	}

	msg := &NASMessage{
		SecurityHeader: SecurityHeader{
			ProtocolDiscriminator: pdToEnum(pd),
			SecurityHeaderType:    shtToEnum(uint8(sht)),
		},
	}

	// Only EMM messages carry the security wrapper; ESM messages are always
	// plain (they ride integrity-protected inside an EMM message or container).
	if pd != eps.PDEMM || sht == eps.SHTPlain {
		return decodePlain(msg, raw)
	}

	if sht == eps.SHTServiceRequest {
		return decodeServiceRequest(msg, raw)
	}

	spm, err := eps.ParseSecurityProtectedMessage(raw)
	if err != nil {
		msg.Error = "security-protected NAS message too short"
		return msg
	}

	msg.SecurityHeader.MessageAuthenticationCode = binary.BigEndian.Uint32(spm.MAC[:])
	msg.SecurityHeader.SequenceNumber = spm.SequenceNumber

	switch sht {
	case eps.SHTIntegrityProtected, eps.SHTIntegrityProtectedNewContext:
		// Integrity-protected but NOT ciphered — the inner NAS is plaintext.
		return decodePlain(msg, spm.UnverifiedPayload)
	case eps.SHTIntegrityProtectedCiphered, eps.SHTIntegrityProtectedCipheredNewContext:
		// The security header does not name the cipher, so decode the payload and keep
		// the result only if it resolves to a recognized message: under null ciphering
		// (EEA0) the payload is plaintext, whereas a real cipher does not frame as a
		// known message and the PDU is reported encrypted. Symmetric with the 5G decoder.
		decoded := decodePlain(msg, spm.UnverifiedPayload)
		if looksLikePlaintext(decoded) {
			return decoded
		}

		msg.EMMMessage = nil
		msg.ESMMessage = nil
		msg.Error = ""
		msg.Encrypted = true

		return msg
	default:
		// Reserved security header type — unknown format, cannot decode.
		msg.Encrypted = true
		return msg
	}
}

// looksLikePlaintext reports whether a decode attempt resolved to a recognized
// NAS message, distinguishing a null-ciphered (EEA0) plaintext payload from a
// real ciphertext that only happens to frame.
func looksLikePlaintext(m *NASMessage) bool {
	if m.EMMMessage != nil && !m.EMMMessage.EMMHeader.MessageType.Unknown {
		return true
	}

	if m.ESMMessage != nil && !m.ESMMessage.ESMHeader.MessageType.Unknown {
		return true
	}

	return false
}

func decodePlain(msg *NASMessage, b []byte) *NASMessage {
	if len(b) < 2 {
		msg.Error = "plain NAS message too short"
		return msg
	}

	switch eps.ProtocolDiscriminator(b[0] & 0x0f) {
	case eps.PDEMM:
		msg.EMMMessage = buildEMMMessage(b)
	case eps.PDESM:
		msg.ESMMessage = buildESMMessage(b)
	default:
		msg.Error = fmt.Sprintf("unknown protocol discriminator %#x", b[0]&0x0f)
	}

	return msg
}

func pdToEnum(pd eps.ProtocolDiscriminator) utils.EnumField {
	return utils.NamedEnum(uint8(pd), pd.Name())
}

func shtToEnum(sht uint8) utils.EnumField {
	return utils.NamedEnum(sht, eps.SecurityHeaderType(sht).Name())
}
