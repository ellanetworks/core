// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// The two NAS transparent containers of an N26-based inter-system handover.
// Neither is carried by a 5GMM message, so TS 24.501 assigns neither an IEI:
// each reaches the UE inside a RAN protocol — NGAP relays them as opaque octet
// strings (TS 38.413 §9.3.3.26 and the NASC IE of §9.2.3.4) and the RAN puts
// them in an RRC element that supplies its own identifier and length
// (TS 24.501 §9.11.2.7, §9.11.2.9, both deferring to TS 38.331). So the codecs
// below encode and decode the *value part* only; the carrier frames it.

// N1ModeToS1ModeNASTransparentContainer is the N1 mode to S1 mode NAS
// transparent container (TS 24.501 §9.11.2.7), a type 3 element with a one-octet
// value part. It is what lets the UE reproduce the K'ASME the AMF derived for a
// 5GS to EPS handover.
type N1ModeToS1ModeNASTransparentContainer struct {
	// SequenceNumber is the Sequence number IE of TS 24.501 §9.10: the 8 least
	// significant bits of the downlink NAS COUNT the AMF used as P0 of the
	// K'ASME derivation (TS 33.501 §8.3.2, annex A.14.2). The UE holds the other
	// 16 bits already, so this is all it needs to rebuild the COUNT.
	SequenceNumber uint8
}

// NewN1ModeToS1ModeNASTransparentContainer builds the container for the downlink
// NAS COUNT that seeded the K'ASME derivation. Taking the whole count rather
// than an octet keeps the caller from truncating it by hand, which is the one
// step that has to agree with the derivation.
func NewN1ModeToS1ModeNASTransparentContainer(dlNASCount nas.Count) N1ModeToS1ModeNASTransparentContainer {
	return N1ModeToS1ModeNASTransparentContainer{SequenceNumber: dlNASCount.SQN()}
}

// ParseN1ModeToS1ModeNASTransparentContainer decodes the container's value part.
func ParseN1ModeToS1ModeNASTransparentContainer(b []byte) (N1ModeToS1ModeNASTransparentContainer, error) {
	if len(b) != n1ModeToS1ModeContainerLen {
		return N1ModeToS1ModeNASTransparentContainer{}, fmt.Errorf(
			"nas/fgs: N1 mode to S1 mode NAS transparent container is %d octets, want %d", len(b), n1ModeToS1ModeContainerLen)
	}

	return N1ModeToS1ModeNASTransparentContainer{SequenceNumber: b[0]}, nil
}

// n1ModeToS1ModeContainerLen is the container's value part: one Sequence number
// octet, the whole of a 2-octet type 3 element beyond its IEI.
const n1ModeToS1ModeContainerLen = 1

// AppendBinary encodes the container's value part onto b.
func (c N1ModeToS1ModeNASTransparentContainer) AppendBinary(b []byte) ([]byte, error) {
	return append(b, c.SequenceNumber), nil
}

// MarshalBinary encodes the container's value part.
func (c N1ModeToS1ModeNASTransparentContainer) MarshalBinary() ([]byte, error) {
	return c.AppendBinary(nil)
}

// S1ModeToN1ModeNASTransparentContainer is the S1 mode to N1 mode NAS
// transparent container (TS 24.501 §9.11.2.9), a type 4 element of 10 octets and
// so an 8-octet value part. It carries the parameters a UE handed over from EPS
// needs to build the mapped 5G NAS security context, and TS 24.501 §4.4.3.1
// makes the AMF treat sending it as sending an initial SECURITY MODE COMMAND —
// which is why it is integrity protected in its own right.
type S1ModeToN1ModeNASTransparentContainer struct {
	// MessageAuthenticationCode is the Message authentication code IE of
	// TS 24.501 §9.8, computed over MACProtected with the mapped context's
	// integrity key (TS 33.501 §6.9.2.3.3).
	MessageAuthenticationCode [4]byte

	// The selected 5G NAS algorithms of the mapped context, coded as in the NAS
	// security algorithms IE (TS 24.501 §9.11.3.34): ciphering in bits 5-8 of
	// octet 7 and integrity in bits 1-4.
	CipheringAlgorithm nas.CipheringAlgorithm
	IntegrityAlgorithm nas.IntegrityAlgorithm

	// NCC is the next hop chaining counter (octet 8, bits 5-7) *associated with
	// the NH used to derive K'AMF* — the EPS NCC received from the MME, not zero.
	// The UE walks its EPS NH chain to it (TS 33.401 §7.2.8.4.4), so a wrong
	// value fails the MAC for every UE.
	NCC uint8

	// NgKSI is the NAS key set identifier of the mapped context (octet 8,
	// bits 1-4): the value taken from the eKSI with the type of security context
	// flag set to mapped (TS 24.501 §9.11.3.32).
	NgKSI nas.KeySetIdentifier

	// Spare holds octet 9 onwards. TS 24.501 §9.11.2.9 codes octets 9 and 10 as
	// zero, which is what encoding emits when Spare is nil; a decoded container
	// keeps whatever arrived, because §4.4.3.3 b) puts every octet from octet 7
	// on under the MAC and earlier versions of the protocol did not zero these.
	Spare []byte
}

const (
	// s1ModeToN1ModeContainerLen is the container's value part: octets 3 to 10 of
	// a 10-octet type 4 element.
	s1ModeToN1ModeContainerLen = 8
	// s1ModeToN1ModeMACLen is the Message authentication code, octets 3 to 6.
	s1ModeToN1ModeMACLen = 4
	// s1ModeToN1ModeSpareLen is octets 9 and 10.
	s1ModeToN1ModeSpareLen = 2
)

// ParseS1ModeToN1ModeNASTransparentContainer decodes the container's value part.
// A container longer than the 8 octets this release defines is accepted and its
// excess kept in Spare: TS 24.501 §4.4.3.3's note has the receiver take every
// octet from octet 7 on into the integrity check whatever release it supports,
// so dropping the excess would break the MAC rather than tolerate the sender.
func ParseS1ModeToN1ModeNASTransparentContainer(b []byte) (S1ModeToN1ModeNASTransparentContainer, error) {
	if len(b) < s1ModeToN1ModeContainerLen {
		return S1ModeToN1ModeNASTransparentContainer{}, fmt.Errorf(
			"nas/fgs: S1 mode to N1 mode NAS transparent container is %d octets, want at least %d",
			len(b), s1ModeToN1ModeContainerLen)
	}

	out := S1ModeToN1ModeNASTransparentContainer{
		CipheringAlgorithm: nas.CipheringAlgorithm(b[4] >> 4 & 0x0F),
		IntegrityAlgorithm: nas.IntegrityAlgorithm(b[4] & 0x0F),
		NCC:                b[5] >> 4 & 0x07,
		NgKSI:              nas.ParseKeySetIdentifier(b[5] & 0x0F),
		Spare:              bytes.Clone(b[6:]),
	}

	copy(out.MessageAuthenticationCode[:], b[:s1ModeToN1ModeMACLen])

	return out, nil
}

// MACProtected returns the octets the message authentication code is computed
// over: the value part from octet 7 on (TS 24.501 §4.4.3.3 b)), which is
// everything after the code itself. The sender fills MessageAuthenticationCode
// from it and the receiver recomputes over the same span.
func (c S1ModeToN1ModeNASTransparentContainer) MACProtected() ([]byte, error) {
	spare := c.Spare
	if spare == nil {
		spare = make([]byte, s1ModeToN1ModeSpareLen)
	}

	if len(spare) < s1ModeToN1ModeSpareLen {
		return nil, fmt.Errorf(
			"nas/fgs: S1 mode to N1 mode NAS transparent container: %d spare octets, want at least %d",
			len(spare), s1ModeToN1ModeSpareLen)
	}

	if c.CipheringAlgorithm > 0x0F || c.IntegrityAlgorithm > 0x0F {
		return nil, fmt.Errorf(
			"nas/fgs: S1 mode to N1 mode NAS transparent container: algorithm identifiers %d/%d do not fit four bits",
			c.CipheringAlgorithm, c.IntegrityAlgorithm)
	}

	if c.NCC > 0x07 {
		return nil, fmt.Errorf("nas/fgs: S1 mode to N1 mode NAS transparent container: NCC %d does not fit three bits", c.NCC)
	}

	b := make([]byte, 0, 2+len(spare))
	b = append(b, uint8(c.CipheringAlgorithm)&0x0F<<4|uint8(c.IntegrityAlgorithm)&0x0F) // octet 7
	b = append(b, (c.NCC&0x07)<<4|c.NgKSI.HalfOctet())                                  // octet 8

	return append(b, spare...), nil
}

// AppendBinary encodes the container's value part onto b.
func (c S1ModeToN1ModeNASTransparentContainer) AppendBinary(b []byte) ([]byte, error) {
	protected, err := c.MACProtected()
	if err != nil {
		return b, err
	}

	b = append(b, c.MessageAuthenticationCode[:]...)

	return append(b, protected...), nil
}

// MarshalBinary encodes the container's value part.
func (c S1ModeToN1ModeNASTransparentContainer) MarshalBinary() ([]byte, error) {
	return c.AppendBinary(nil)
}
