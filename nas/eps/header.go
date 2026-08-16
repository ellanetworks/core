// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// ProtocolDiscriminator names the NAS protocol a message belongs to. EPS packs
// it into bits 1-4 of the first octet, where 5GS gives its extended protocol
// discriminator the whole octet (TS 24.301 §9.2, TS 24.007 §11.2.3.1.1).
type ProtocolDiscriminator uint8

// Protocol discriminators (TS 24.007 §11.2.3.1.1).
const (
	PDEMM ProtocolDiscriminator = 0x07 // EPS Mobility Management
	PDESM ProtocolDiscriminator = 0x02 // EPS Session Management
)

var protocolDiscriminatorNames = map[ProtocolDiscriminator]string{
	PDEMM: "EMM",
	PDESM: "ESM",
}

// Name returns the discriminator's spec description, or the empty string when
// the value is not one TS 24.007 assigns.
func (p ProtocolDiscriminator) Name() string { return protocolDiscriminatorNames[p] }

func (p ProtocolDiscriminator) String() string {
	if name, ok := protocolDiscriminatorNames[p]; ok {
		return name
	}

	return fmt.Sprintf("unknown protocol discriminator (%#x)", uint8(p))
}

// SecurityHeaderType identifies the protection applied to an EMM message
// (TS 24.301).
type SecurityHeaderType uint8

// Security header types (TS 24.301 §9.3.1, table 9.3.1).
const (
	SHTPlain                                SecurityHeaderType = 0
	SHTIntegrityProtected                   SecurityHeaderType = 1
	SHTIntegrityProtectedCiphered           SecurityHeaderType = 2
	SHTIntegrityProtectedNewContext         SecurityHeaderType = 3
	SHTIntegrityProtectedCipheredNewContext SecurityHeaderType = 4

	// SHTIntegrityProtectedPartiallyCiphered marks a CONTROL PLANE SERVICE
	// REQUEST whose ESM container and NAS message container are ciphered while
	// the rest of the message is not (TS 24.301 table 9.3.1).
	SHTIntegrityProtectedPartiallyCiphered SecurityHeaderType = 5

	// SHTServiceRequest marks the SERVICE REQUEST message, which carries no
	// message identity and a 2-octet short MAC in place of the normal wrapper
	// (TS 24.301).
	SHTServiceRequest SecurityHeaderType = 12
)

// Ciphered reports whether this header type marks a ciphered message (TS 24.301 §4.4.5).
func (s SecurityHeaderType) Ciphered() bool {
	return s == SHTIntegrityProtectedCiphered || s == SHTIntegrityProtectedCipheredNewContext
}

// defined reports whether TS 24.301 table 9.3.1 assigns this value. Everything
// else is reserved, and a receiver must not guess what protection it names.
func (s SecurityHeaderType) defined() bool {
	return s <= SHTIntegrityProtectedPartiallyCiphered || s == SHTServiceRequest
}

var securityHeaderTypeNames = map[SecurityHeaderType]string{
	SHTPlain:                                "plain",
	SHTIntegrityProtected:                   "integrity protected",
	SHTIntegrityProtectedCiphered:           "integrity protected and ciphered",
	SHTIntegrityProtectedNewContext:         "integrity protected with new EPS security context",
	SHTIntegrityProtectedCipheredNewContext: "integrity protected and ciphered with new EPS security context",
	SHTIntegrityProtectedPartiallyCiphered:  "integrity protected and partially ciphered",
	SHTServiceRequest:                       "service request",
}

// Name returns the type's spec description, or the empty string for a value
// TS 24.301 table 9.3.1 reserves.
func (s SecurityHeaderType) Name() string { return securityHeaderTypeNames[s] }

func (s SecurityHeaderType) String() string {
	if name, ok := securityHeaderTypeNames[s]; ok {
		return name
	}

	return fmt.Sprintf("reserved security header type (%d)", uint8(s))
}

// ErrNotEMM reports a protocol discriminator other than EMM.
var ErrNotEMM = errors.New("nas/eps: not an EMM message")

// ErrNotProtected reports a security-header type of 0 (plain message) where a
// security-protected message was expected.
var ErrNotProtected = fmt.Errorf("nas/eps: %w", nas.ErrNotProtected)

// SecurityProtectedMessage is the outer EMM security wrapper (TS 24.301,
// figure in): a security-header-type + protocol-discriminator octet, a
// 4-octet message authentication code, a 1-octet sequence number, and the inner
// NAS message (ciphered when the header type indicates).
type SecurityProtectedMessage struct {
	SecurityHeaderType SecurityHeaderType
	MAC                [4]byte
	SequenceNumber     uint8

	// UnverifiedPayload is the inner message as it arrived: still ciphered when
	// the header type says so, and — until [Unprotect] has verified the MAC —
	// attacker-controlled. Nothing decoded from it may be acted on before that.
	UnverifiedPayload []byte
}

// AppendBinary serializes the wrapper. Payload is emitted verbatim (already ciphered
// if the header type so indicates).
// The encoding is appended to b.
func (m *SecurityProtectedMessage) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	w.U8(uint8(m.SecurityHeaderType)<<4 | uint8(PDEMM))
	w.Raw(m.MAC[:])
	w.U8(m.SequenceNumber)
	w.Raw(m.UnverifiedPayload)

	return messageResult(w, b)
}

// MarshalBinary encodes the SecurityProtectedMessage information element value.
func (m *SecurityProtectedMessage) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// PeekSecurityHeaderType returns the security-header type of a NAS message
// without decoding it: octet 0, high nibble (TS 24.301 §9.3.1). The counterpart
// to fgs.PeekSecurityHeaderType, which reads the 5G placement (octet 1, low nibble).
func PeekSecurityHeaderType(b []byte) (SecurityHeaderType, error) {
	octet0, err := nas.NewReader(b).U8()
	if err != nil {
		return 0, err
	}

	return SecurityHeaderType(octet0 >> 4), nil
}

// ParseSecurityProtectedMessage frames a security-protected EMM message. It does
// not verify the MAC or decipher — that is the caller's step (see Unprotect),
// keeping the codec decoupled from the security algorithms.
func ParseSecurityProtectedMessage(b []byte) (*SecurityProtectedMessage, error) {
	if err := nas.CheckPDULen(len(b)); err != nil {
		return nil, err
	}

	r := nas.NewReader(b)

	octet0, err := r.U8()
	if err != nil {
		return nil, err
	}

	if ProtocolDiscriminator(octet0&0x0F) != PDEMM {
		return nil, fmt.Errorf("%w (PD %#x)", ErrNotEMM, octet0&0x0F)
	}

	sht := SecurityHeaderType(octet0 >> 4)
	if sht == SHTPlain {
		return nil, ErrNotProtected
	}

	// A reserved value names no protection, so nothing can be verified under it,
	// and a SERVICE REQUEST is framed differently: it carries a 2-octet short MAC
	// in place of this wrapper (TS 24.301 table 9.3.1, §8.2.25).
	if !sht.defined() || sht == SHTServiceRequest {
		return nil, fmt.Errorf("nas/eps: %s is not a security-protected message wrapper", sht)
	}

	mac, err := r.Bytes(4)
	if err != nil {
		return nil, err
	}

	seq, err := r.U8()
	if err != nil {
		return nil, err
	}

	payload, err := r.Bytes(r.Remaining())
	if err != nil {
		return nil, err
	}

	out := &SecurityProtectedMessage{SecurityHeaderType: sht, SequenceNumber: seq, UnverifiedPayload: payload}
	copy(out.MAC[:], mac)

	return out, nil
}
