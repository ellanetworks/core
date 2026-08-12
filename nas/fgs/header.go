// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// ProtocolDiscriminator names the NAS protocol a message belongs to. 5GS calls
// it the extended protocol discriminator and gives it the whole first octet,
// where EPS packs a four-bit discriminator into half of one (TS 24.501 §9.2,
// TS 24.007 §11.2.3.1.1A).
type ProtocolDiscriminator uint8

// Extended protocol discriminators (TS 24.501 §9.2, TS 24.007 §11.2.3.1.1A).
const (
	EPD5GMM ProtocolDiscriminator = 0x7E // 5GS Mobility Management
	EPD5GSM ProtocolDiscriminator = 0x2E // 5GS Session Management
)

var protocolDiscriminatorNames = map[ProtocolDiscriminator]string{
	EPD5GMM: "5GMM",
	EPD5GSM: "5GSM",
}

// Name returns the discriminator's spec description, or the empty string when
// the value is not one TS 24.501 assigns.
func (p ProtocolDiscriminator) Name() string { return protocolDiscriminatorNames[p] }

func (p ProtocolDiscriminator) String() string {
	if name, ok := protocolDiscriminatorNames[p]; ok {
		return name
	}

	return fmt.Sprintf("unknown protocol discriminator (%#x)", uint8(p))
}

// SecurityHeaderType identifies the protection applied to a 5GMM message
// (TS 24.501 §9.3). It occupies bits 1-4 of the second octet; bits 5-8 are the
// spare half octet.
type SecurityHeaderType uint8

// Security header types (TS 24.501 §9.3, table 9.3.1).
const (
	SHTPlain                                SecurityHeaderType = 0
	SHTIntegrityProtected                   SecurityHeaderType = 1
	SHTIntegrityProtectedCiphered           SecurityHeaderType = 2
	SHTIntegrityProtectedNewContext         SecurityHeaderType = 3 // SECURITY MODE COMMAND only
	SHTIntegrityProtectedCipheredNewContext SecurityHeaderType = 4 // SECURITY MODE COMPLETE only
)

func (s SecurityHeaderType) ciphered() bool {
	return s == SHTIntegrityProtectedCiphered || s == SHTIntegrityProtectedCipheredNewContext
}

// defined reports whether TS 24.501 table 9.3.1 assigns this value. Everything
// else is reserved, and a receiver must not guess what protection it names.
func (s SecurityHeaderType) defined() bool { return s <= SHTIntegrityProtectedCipheredNewContext }

var securityHeaderTypeNames = map[SecurityHeaderType]string{
	SHTPlain:                                "plain",
	SHTIntegrityProtected:                   "integrity protected",
	SHTIntegrityProtectedCiphered:           "integrity protected and ciphered",
	SHTIntegrityProtectedNewContext:         "integrity protected with new 5G NAS security context",
	SHTIntegrityProtectedCipheredNewContext: "integrity protected and ciphered with new 5G NAS security context",
}

// Name returns the type's spec description, or the empty string for a value
// TS 24.501 table 9.3.1 reserves.
func (s SecurityHeaderType) Name() string { return securityHeaderTypeNames[s] }

func (s SecurityHeaderType) String() string {
	if name, ok := securityHeaderTypeNames[s]; ok {
		return name
	}

	return fmt.Sprintf("reserved security header type (%d)", uint8(s))
}

// ErrNotGMM reports an extended protocol discriminator other than 5GMM.
var ErrNotGMM = errors.New("nas/fgs: not a 5GMM message")

// ErrNotProtected reports a security-header type of 0 (plain message) where a
// security-protected message was expected.
var ErrNotProtected = fmt.Errorf("nas/fgs: %w", nas.ErrNotProtected)

// SecurityProtectedMessage is the outer 5GMM security wrapper (TS 24.501
// §9.1.1): a 1-octet extended protocol discriminator (always 5GMM), a 1-octet
// security-header-type with spare half octet, a 4-octet message authentication
// code, a 1-octet sequence number, and the inner plain 5GS NAS message (ciphered
// when the header type indicates). The wrapper is 7 octets before the payload.
type SecurityProtectedMessage struct {
	SecurityHeaderType SecurityHeaderType
	MAC                [4]byte
	SequenceNumber     uint8

	// UnverifiedPayload is the inner message as it arrived: still ciphered when
	// the header type says so, and — until [Unprotect] has verified the MAC —
	// attacker-controlled. Nothing decoded from it may be acted on before that.
	UnverifiedPayload []byte
}

// AppendBinary serializes the wrapper. The payload is emitted verbatim (already
// ciphered if the header type so indicates).
// The encoding is appended to b.
func (m *SecurityProtectedMessage) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	w.U8(uint8(EPD5GMM))
	w.U8(uint8(m.SecurityHeaderType) & 0x0F)
	w.Raw(m.MAC[:])
	w.U8(m.SequenceNumber)
	w.Raw(m.UnverifiedPayload)

	return messageResult(w, b)
}

// MarshalBinary encodes the SecurityProtectedMessage information element value.
func (m *SecurityProtectedMessage) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// PeekSecurityHeaderType returns the security-header type of a NAS message
// without decoding it: octet 1, low nibble (TS 24.501 §9.3). The counterpart to
// eps.PeekSecurityHeaderType, which reads the 4G placement (octet 0, high nibble).
func PeekSecurityHeaderType(b []byte) (SecurityHeaderType, error) {
	r := nas.NewReader(b)

	if _, err := r.U8(); err != nil { // extended protocol discriminator
		return 0, err
	}

	octet1, err := r.U8()
	if err != nil {
		return 0, err
	}

	return SecurityHeaderType(octet1 & 0x0F), nil
}

// ParseSecurityProtectedMessage frames a security-protected 5GMM message. It
// does not verify the MAC or decipher — that is the caller's step (see
// Unprotect), keeping the codec decoupled from the security algorithms.
func ParseSecurityProtectedMessage(b []byte) (*SecurityProtectedMessage, error) {
	if err := nas.CheckPDULen(len(b)); err != nil {
		return nil, err
	}

	r := nas.NewReader(b)

	epd, err := r.U8()
	if err != nil {
		return nil, err
	}

	if ProtocolDiscriminator(epd) != EPD5GMM {
		return nil, fmt.Errorf("%w (EPD %#x)", ErrNotGMM, epd)
	}

	octet1, err := r.U8()
	if err != nil {
		return nil, err
	}

	sht := SecurityHeaderType(octet1 & 0x0F)
	if sht == SHTPlain {
		return nil, ErrNotProtected
	}

	// A reserved value names no protection, so nothing can be verified under it
	// (TS 24.501 table 9.3.1).
	if !sht.defined() {
		return nil, fmt.Errorf("nas/fgs: %s", sht)
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
