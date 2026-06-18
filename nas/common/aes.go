// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
)

// AESCMACIntegrity is 128-EIA2: AES-CMAC over
// COUNT‖(BEARER<<3|DIRECTION<<2)‖0³‖message, truncated to the first 4 octets
// (TS 33.401 §B.2.2).
type AESCMACIntegrity struct{}

func (AESCMACIntegrity) MAC(key [16]byte, count uint32, bearer, direction uint8, msg []byte) ([4]byte, error) {
	m := make([]byte, len(msg)+8)
	binary.BigEndian.PutUint32(m, count)
	m[4] = bearer<<3 | direction<<2
	copy(m[8:], msg)

	mac, err := aesCMAC(key[:], m)
	if err != nil {
		return [4]byte{}, err
	}

	var out [4]byte

	copy(out[:], mac[:4])

	return out, nil
}

// AESCTRCipher is 128-EEA2: AES in counter mode with the 16-octet counter block
// COUNT‖(BEARER<<3|DIRECTION<<2)‖0¹¹ (TS 33.401 §B.1.2). The block is not a fixed
// nonce — it is derived from the per-message NAS COUNT, which is strictly
// monotonic per direction and never repeats within a security context
// (TS 24.301 §4.4.3), giving the uniqueness CTR requires.
type AESCTRCipher struct{}

func (AESCTRCipher) Apply(key [16]byte, count uint32, bearer, direction uint8, data []byte) ([]byte, error) {
	var iv [16]byte

	binary.BigEndian.PutUint32(iv[:], count)
	iv[4] = bearer<<3 | direction<<2

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	out := make([]byte, len(data))
	// #nosec G407 -- iv is the per-message NAS counter block (COUNT‖BEARER‖DIRECTION), not a hardcoded nonce; see the type doc.
	cipher.NewCTR(block, iv[:]).XORKeyStream(out, data)

	return out, nil
}

// aesCMAC computes AES-CMAC (RFC 4493 / NIST SP 800-38B) over msg. It is the
// building block of 128-EIA2; the NAS-MAC is its first four octets. Implemented
// on top of crypto/aes (no external dependency) and verified against the
// RFC 4493 test vectors.
func aesCMAC(key, msg []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	const bs = aes.BlockSize

	zero := make([]byte, bs)
	l := make([]byte, bs)
	block.Encrypt(l, zero)

	k1 := cmacSubkey(l)
	k2 := cmacSubkey(k1)

	n := (len(msg) + bs - 1) / bs
	complete := len(msg) > 0 && len(msg)%bs == 0

	if n == 0 {
		n = 1
	}

	last := make([]byte, bs)
	start := (n - 1) * bs

	if complete {
		xorBytes(last, msg[start:], k1)
	} else {
		rem := msg[start:]
		copy(last, rem)
		last[len(rem)] = 0x80
		xorBytes(last, last, k2)
	}

	x := make([]byte, bs)

	for i := 0; i < n-1; i++ {
		xorBytes(x, x, msg[i*bs:(i+1)*bs])
		block.Encrypt(x, x)
	}

	xorBytes(x, x, last)
	block.Encrypt(x, x)

	return x, nil
}

// cmacSubkey derives a CMAC subkey: left-shift by one bit, XOR the Rb constant
// (0x87 in the last octet for AES) when the input's MSB was set.
func cmacSubkey(in []byte) []byte {
	out := make([]byte, len(in))

	var carry byte

	for i := len(in) - 1; i >= 0; i-- {
		out[i] = in[i]<<1 | carry
		carry = in[i] >> 7
	}

	if in[0]&0x80 != 0 {
		out[len(out)-1] ^= 0x87
	}

	return out
}

func xorBytes(dst, a, b []byte) {
	for i := range dst {
		dst[i] = a[i] ^ b[i]
	}
}
