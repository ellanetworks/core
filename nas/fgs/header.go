// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package fgs implements the 5GS NAS codec of TS 24.501 (5G): 5GMM/5GSM message
// headers, the security-protected message wrapper, and the message and
// information-element catalog. It builds on github.com/ellanetworks/core/nas/common
// for octet framing and the integrity/cipher algorithms.
package fgs

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas/common"
)

// Extended protocol discriminators (TS 24.501 §9.2, TS 24.007 §11.2.3.1.1A).
const (
	EPD5GMM uint8 = 0x7E // 5GS Mobility Management
	EPD5GSM uint8 = 0x2E // 5GS Session Management
)

// SecurityHeaderType identifies the protection applied to a 5GMM message
// (TS 24.501 §9.3). It occupies bits 1-4 of the second octet; bits 5-8 are the
// spare half octet.
type SecurityHeaderType uint8

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

// ErrNotGMM reports an extended protocol discriminator other than 5GMM.
var ErrNotGMM = errors.New("nas/fgs: not a 5GMM message")

// ErrNotProtected reports a security-header type of 0 (plain message) where a
// security-protected message was expected.
var ErrNotProtected = errors.New("nas/fgs: message is not security protected")

// SecurityProtectedMessage is the outer 5GMM security wrapper (TS 24.501
// §9.1.1): a 1-octet extended protocol discriminator (always 5GMM), a 1-octet
// security-header-type with spare half octet, a 4-octet message authentication
// code, a 1-octet sequence number, and the inner plain 5GS NAS message (ciphered
// when the header type indicates). The wrapper is 7 octets before the payload.
type SecurityProtectedMessage struct {
	SecurityHeaderType SecurityHeaderType
	MAC                [4]byte
	SequenceNumber     uint8
	Payload            []byte
}

// Marshal serializes the wrapper. Payload is emitted verbatim (already ciphered
// if the header type so indicates).
func (m *SecurityProtectedMessage) Marshal() []byte {
	var w common.Writer

	w.U8(EPD5GMM)
	w.U8(uint8(m.SecurityHeaderType) & 0x0F)
	w.Raw(m.MAC[:])
	w.U8(m.SequenceNumber)
	w.Raw(m.Payload)

	return w.Bytes()
}

// ParseSecurityProtectedMessage frames a security-protected 5GMM message. It
// does not verify the MAC or decipher — that is the caller's step (see
// Unprotect), keeping the codec decoupled from the security algorithms.
func ParseSecurityProtectedMessage(b []byte) (*SecurityProtectedMessage, error) {
	r := common.NewReader(b)

	epd, err := r.U8()
	if err != nil {
		return nil, err
	}

	if epd != EPD5GMM {
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

	out := &SecurityProtectedMessage{SecurityHeaderType: sht, SequenceNumber: seq, Payload: payload}
	copy(out.MAC[:], mac)

	return out, nil
}
