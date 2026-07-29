// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/nas"
)

// nasBearer is the BEARER input to the 5G NAS integrity/cipher algorithms: the
// NAS connection identifier, 0x01 for 3GPP access (TS 33.501 §6.4.3.1, §6.9.2.1).
const nasBearer = nas.Bearer3GPP

// ErrMACMismatch reports a failed integrity check in Unprotect.
var ErrMACMismatch = fmt.Errorf("nas/fgs: %w", nas.ErrMACMismatch)

// ErrSecurityHeaderTypeNotPermitted reports a protected message whose security
// header type the caller did not permit.
var ErrSecurityHeaderTypeNotPermitted = fmt.Errorf("nas/fgs: %w", nas.ErrSecurityHeaderTypeNotPermitted)

// ErrSequenceNumberMismatch reports a protected message whose sequence number is
// not the one carried by the NAS COUNT it was verified under.
var ErrSequenceNumberMismatch = fmt.Errorf("nas/fgs: %w", nas.ErrSequenceNumberMismatch)

// Protect wraps a plain 5GMM message as a security-protected message: it ciphers
// the payload when sht selects a ciphered type, computes the NAS-MAC over the
// sequence-number octet followed by the (ciphered) payload, and assembles the
// wrapper (TS 24.501 §4.4.4.1).
//
// The caller owns the security context and the NAS COUNT. This function does not
// advance the count: take it from a [nas.DownlinkCounter], which refuses to
// reuse one, because a repeat under the same key hands an eavesdropper the
// keystream (TS 33.501 §6.4.3.1). The count's low octet becomes the sequence
// number on the wire.
//
// sht may not be [SHTPlain]: a plain message is not protected, and asking for
// one here would emit a wrapper carrying a MAC no receiver would check.
func Protect(
	plain []byte,
	sht SecurityHeaderType,
	count nas.Count,
	dir nas.Direction,
	sc *nas.SecurityContext,
) ([]byte, error) {
	if sc == nil {
		return nil, nas.ErrNoSecurityContext
	}

	if sht == SHTPlain {
		return nil, fmt.Errorf("nas/fgs: %w", nas.ErrNotProtected)
	}

	if !sht.defined() {
		return nil, fmt.Errorf("nas/fgs: security header type %s is not defined", sht)
	}

	payload := plain

	if sht.ciphered() {
		c, err := sc.Cipher(plain, count, nasBearer, dir)
		if err != nil {
			return nil, err
		}

		payload = c
	}

	seq := count.SQN()

	mac, err := sc.MAC(macInput(seq, payload), count, nasBearer, dir)
	if err != nil {
		return nil, err
	}

	m := &SecurityProtectedMessage{
		SecurityHeaderType: sht,
		MAC:                mac,
		SequenceNumber:     seq,
		UnverifiedPayload:  payload,
	}

	return m.MarshalBinary()
}

// Unprotect verifies a security-protected message and returns the plain 5GMM
// message it carries, together with the security header type it verified under.
//
// The caller owns the security context and the NAS COUNT. Estimate the count
// from the message's sequence number with a [nas.UplinkCounter], and commit it
// only once this has returned without error: the counter is what stops a replay,
// since the same message under the same count verifies every time.
//
// permitted lists the security header types the caller accepts at this point in
// its procedure — a new-context type outside the security mode procedure is a
// downgrade attempt (TS 24.501 §4.4.4.2, TS 33.501 §6.7.2). Passing none accepts
// every type TS 24.501 table 9.3.1 defines for a protected message.
func Unprotect(
	b []byte,
	count nas.Count,
	dir nas.Direction,
	sc *nas.SecurityContext,
	permitted ...SecurityHeaderType,
) ([]byte, SecurityHeaderType, error) {
	if sc == nil {
		return nil, 0, nas.ErrNoSecurityContext
	}

	m, err := ParseSecurityProtectedMessage(b)
	if err != nil {
		return nil, 0, err
	}

	if len(permitted) > 0 && !slices.Contains(permitted, m.SecurityHeaderType) {
		return nil, m.SecurityHeaderType, fmt.Errorf("%w: %s", ErrSecurityHeaderTypeNotPermitted, m.SecurityHeaderType)
	}

	// The sequence number is the count's own low octet (TS 24.501 §4.4.3.1). A
	// mismatch means the caller verified under a count that does not belong to
	// this message — which no MAC check would catch under a null algorithm.
	if m.SequenceNumber != count.SQN() {
		return nil, m.SecurityHeaderType, fmt.Errorf("%w: message carries %#02x, NAS COUNT carries %#02x",
			ErrSequenceNumberMismatch, m.SequenceNumber, count.SQN())
	}

	if err := sc.VerifyMAC(macInput(m.SequenceNumber, m.UnverifiedPayload), m.MAC, count, nasBearer, dir); err != nil {
		// The package sentinel wraps the root one, so a caller can match either.
		if errors.Is(err, nas.ErrMACMismatch) {
			return nil, m.SecurityHeaderType, ErrMACMismatch
		}

		return nil, m.SecurityHeaderType, err
	}

	if !m.SecurityHeaderType.ciphered() {
		return m.UnverifiedPayload, m.SecurityHeaderType, nil
	}

	plain, err := sc.Cipher(m.UnverifiedPayload, count, nasBearer, dir)
	if err != nil {
		return nil, m.SecurityHeaderType, err
	}

	return plain, m.SecurityHeaderType, nil
}

// macInput is the integrity-protected span: the sequence-number octet followed
// by the (ciphered) NAS message payload (TS 24.501 §4.4.4.1).
func macInput(seq uint8, payload []byte) []byte {
	out := make([]byte, 0, len(payload)+1)
	out = append(out, seq)
	out = append(out, payload...)

	return out
}
