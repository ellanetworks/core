// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

// NAS message direction for the integrity/cipher algorithm input (TS 33.401).
const (
	DirectionUplink   uint8 = 0
	DirectionDownlink uint8 = 1
)

// NASCount builds the 32-bit algorithm input from the 24-bit NAS COUNT —
// a 16-bit overflow counter and an 8-bit sequence number, zero-padded in the
// 8 most significant bits (TS 24.301).
func NASCount(overflow uint16, sequence uint8) uint32 {
	return uint32(overflow)<<8 | uint32(sequence)
}

// Integrity computes the 4-octet NAS-MAC over a NAS message (TS 33.401).
// The implementation is chosen by the caller; the lib does not select algorithms.
// 128-EIA1 (SNOW3G) and 128-EIA2 (AES) live in snow3g.go and aes.go.
type Integrity interface {
	MAC(key [16]byte, count uint32, bearer, direction uint8, msg []byte) ([4]byte, error)
}

// Cipher enciphers or deciphers a NAS payload — a keystream XOR, so the same
// operation runs in both directions (TS 33.401). 128-EEA1 (SNOW3G) and
// 128-EEA2 (AES) live in snow3g.go and aes.go.
type Cipher interface {
	Apply(key [16]byte, count uint32, bearer, direction uint8, data []byte) ([]byte, error)
}

// NullIntegrity is 128-EIA0: no integrity, an all-zero MAC. Permitted only for
// unauthenticated emergency bearer services (TS 33.401).
type NullIntegrity struct{}

func (NullIntegrity) MAC(_ [16]byte, _ uint32, _, _ uint8, _ []byte) ([4]byte, error) {
	return [4]byte{}, nil
}

// NullCipher is 128-EEA0: no ciphering, data passes through unchanged.
type NullCipher struct{}

func (NullCipher) Apply(_ [16]byte, _ uint32, _, _ uint8, data []byte) ([]byte, error) {
	out := make([]byte, len(data))
	copy(out, data)

	return out, nil
}
