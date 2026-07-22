// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"crypto/subtle"
	"errors"

	"github.com/ellanetworks/core/nas/common"
)

// nasBearer is the BEARER input to the 5G NAS integrity/cipher algorithms: the
// NAS connection identifier, 0x01 for 3GPP access (TS 33.501 §6.4.3.1, §6.9.2.1).
const nasBearer uint8 = 0x01

// ErrMACMismatch reports a failed integrity check in Unprotect.
var ErrMACMismatch = errors.New("nas/fgs: NAS-MAC verification failed")

// Protect wraps a plain 5GMM message as a security-protected message: it ciphers
// the payload when sht selects a ciphered type, computes the NAS-MAC over the
// sequence-number octet followed by the (ciphered) payload, and assembles the
// wrapper (TS 24.501 §4.4.4.1).
//
// The caller selects the algorithms (integ, ciph) and supplies the keys, the
// 32-bit NAS COUNT (its low octet is the on-wire sequence number), and the
// direction. The lib performs the mechanism, never the algorithm choice.
func Protect(
	plain []byte,
	sht SecurityHeaderType,
	count uint32,
	direction uint8,
	kNASint, kNASenc [16]byte,
	integ common.Integrity,
	ciph common.Cipher,
) ([]byte, error) {
	payload := plain

	if sht.ciphered() {
		c, err := ciph.Apply(kNASenc, count, nasBearer, direction, plain)
		if err != nil {
			return nil, err
		}

		payload = c
	}

	seq := uint8(count)

	mac, err := integ.MAC(kNASint, count, nasBearer, direction, macInput(seq, payload))
	if err != nil {
		return nil, err
	}

	m := &SecurityProtectedMessage{SecurityHeaderType: sht, MAC: mac, SequenceNumber: seq, Payload: payload}

	return m.Marshal(), nil
}

// Unprotect parses a security-protected message, verifies its NAS-MAC against
// the caller-supplied algorithm/key/count, deciphers the payload when the header
// type indicates, and returns the recovered plain 5GMM message.
func Unprotect(
	b []byte,
	count uint32,
	direction uint8,
	kNASint, kNASenc [16]byte,
	integ common.Integrity,
	ciph common.Cipher,
) ([]byte, error) {
	m, err := ParseSecurityProtectedMessage(b)
	if err != nil {
		return nil, err
	}

	want, err := integ.MAC(kNASint, count, nasBearer, direction, macInput(m.SequenceNumber, m.Payload))
	if err != nil {
		return nil, err
	}

	if subtle.ConstantTimeCompare(want[:], m.MAC[:]) != 1 {
		return nil, ErrMACMismatch
	}

	if !m.SecurityHeaderType.ciphered() {
		return m.Payload, nil
	}

	return ciph.Apply(kNASenc, count, nasBearer, direction, m.Payload)
}

// macInput is the integrity-protected span: the sequence-number octet followed
// by the (ciphered) NAS message payload (TS 24.501 §4.4.4.1).
func macInput(seq uint8, payload []byte) []byte {
	out := make([]byte, 0, len(payload)+1)
	out = append(out, seq)
	out = append(out, payload...)

	return out
}
