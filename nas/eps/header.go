// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package eps implements the EPS NAS codec of TS 24.301 (4G): EMM/ESM message
// headers, the security-protected message wrapper, and (in later phases) the
// message and information-element catalog. It builds on github.com/ellanetworks/
// nas/common for octet framing and the integrity/cipher algorithms.
package eps

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas/common"
)

// Protocol discriminators (TS 24.007 §11.2.3.1.1).
const (
	PDEMM uint8 = 0x07 // EPS Mobility Management
	PDESM uint8 = 0x02 // EPS Session Management
)

// SecurityHeaderType identifies the protection applied to an EMM message
// (TS 24.301 §9.3.1).
type SecurityHeaderType uint8

const (
	SHTPlain                                SecurityHeaderType = 0
	SHTIntegrityProtected                   SecurityHeaderType = 1
	SHTIntegrityProtectedCiphered           SecurityHeaderType = 2
	SHTIntegrityProtectedNewContext         SecurityHeaderType = 3
	SHTIntegrityProtectedCipheredNewContext SecurityHeaderType = 4

	// SHTServiceRequest marks the SERVICE REQUEST message, which carries no
	// message identity and a 2-octet short MAC instead of the normal wrapper
	// (TS 24.301 §9.3.1, §8.2.25).
	SHTServiceRequest SecurityHeaderType = 12
)

func (s SecurityHeaderType) ciphered() bool {
	return s == SHTIntegrityProtectedCiphered || s == SHTIntegrityProtectedCipheredNewContext
}

// ErrNotEMM reports a protocol discriminator other than EMM.
var ErrNotEMM = errors.New("nas/eps: not an EMM message")

// ErrNotProtected reports a security-header type of 0 (plain message) where a
// security-protected message was expected.
var ErrNotProtected = errors.New("nas/eps: message is not security protected")

// SecurityProtectedMessage is the outer EMM security wrapper (TS 24.301 §9.1.1,
// figure in §9.5): a security-header-type + protocol-discriminator octet, a
// 4-octet message authentication code, a 1-octet sequence number, and the inner
// NAS message (ciphered when the header type indicates).
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

	w.U8(uint8(m.SecurityHeaderType)<<4 | PDEMM)
	w.Raw(m.MAC[:])
	w.U8(m.SequenceNumber)
	w.Raw(m.Payload)

	return w.Bytes()
}

// ParseSecurityProtectedMessage frames a security-protected EMM message. It does
// not verify the MAC or decipher — that is the caller's step (see Unprotect),
// keeping the codec decoupled from the security algorithms.
func ParseSecurityProtectedMessage(b []byte) (*SecurityProtectedMessage, error) {
	r := common.NewReader(b)

	octet0, err := r.U8()
	if err != nil {
		return nil, err
	}

	if octet0&0x0F != PDEMM {
		return nil, fmt.Errorf("%w (PD %#x)", ErrNotEMM, octet0&0x0F)
	}

	sht := SecurityHeaderType(octet0 >> 4)
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
