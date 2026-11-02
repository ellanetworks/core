// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package usim

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des" // #nosec G502 -- TS 102 225 clause 5.1.3.1 defines two-key triple DES for SCP80
	"crypto/subtle"
	"errors"
	"fmt"
)

// The user data headers of the secured packets in a single short message: the UDHL, then the CPI, IEI '70', or the
// RPI, IEI '71', of a null length (TS 31.115 clauses 4.2 and 4.4).
var (
	commandUDH  = []byte{0x02, 0x70, 0x00}
	responseUDH = []byte{0x02, 0x71, 0x00}
)

const (
	counterLength = 5
	maxCounter    = 1<<(8*counterLength) - 1
)

// Algorithm is the algorithm of a key set, as the low nibble of KIc and KID codes it (TS 102 225 clauses 5.1.2 and
// 5.1.3.1).
type Algorithm byte

const (
	// TripleDES is triple DES in outer-CBC mode with two keys: CBC for ciphering, a CBC-MAC for the checksum.
	TripleDES Algorithm = 0x5
	// AES is AES in CBC mode for ciphering, in CMAC mode for the checksum.
	AES Algorithm = 0x2
)

// The fields of the SPI (TS 102 225 clause 5.1.1).
const (
	spiIntegrity      = 0x0300
	spiIntegrityCC    = 0x0200
	spiCiphering      = 0x0400
	spiCounter        = 0x1800
	spiCounterNone    = 0x0000
	spiCounterInfo    = 0x0800
	spiCounterHigher  = 0x1000
	spiCounterOneMore = 0x1800
	spiPoR            = 0x0003
	spiPoRAlways      = 0x0001
	spiPoROnError     = 0x0002
	spiPoRIntegrity   = 0x000c
	spiPoRCiphering   = 0x0010
	spiPoRSubmit      = 0x0020
)

// Status is the Response Status Code of a Response Packet (TS 102 225 table 5, TS 31.115 clause 7).
type Status byte

const (
	StatusPoROK            Status = 0x00
	StatusCNTRLow          Status = 0x02
	StatusCNTRHigh         Status = 0x03
	StatusCNTRBlocked      Status = 0x04
	StatusUnidentified     Status = 0x06
	StatusTARUnknown       Status = 0x09
	StatusInsufficient     Status = 0x0a
	StatusResponseInSubmit Status = 0x0b
)

// Keyset is a key set of the card's security domain for SCP80.
type Keyset struct {
	Version   uint8
	Algorithm Algorithm
	KIc, KID  []byte
	// CCLength is the length of the AES key set's cryptographic checksums, 4 or 8, as its key is configured for (TS
	// 102 225 clause 5.1.3.1); 0 is 8. A triple DES checksum is 8 octets.
	CCLength int
}

func (k Keyset) validate() error {
	if k.Version < 1 || k.Version > 15 {
		return fmt.Errorf("key version %d not in 1-15", k.Version)
	}

	switch k.Algorithm {
	case TripleDES:
		if len(k.KIc) != 16 || len(k.KID) != 16 {
			return errors.New("two-key triple DES keys are 16 octets")
		}

		if k.CCLength != 0 && k.CCLength != 8 {
			return errors.New("a triple DES checksum is 8 octets")
		}
	case AES:
		for _, key := range [][]byte{k.KIc, k.KID} {
			if n := len(key); n != 16 && n != 24 && n != 32 {
				return fmt.Errorf("an AES key of %d octets is not of 128, 192 or 256 bits", n)
			}
		}

		if k.CCLength != 0 && k.CCLength != 4 && k.CCLength != 8 {
			return fmt.Errorf("an AES checksum of %d octets is not of 32 or 64 bits", k.CCLength)
		}
	default:
		return fmt.Errorf("algorithm %#x", byte(k.Algorithm))
	}

	return nil
}

func (k Keyset) ccLength() int {
	if k.CCLength == 0 {
		return 8
	}

	return k.CCLength
}

func (k Keyset) block(key []byte) (cipher.Block, error) {
	if k.Algorithm == AES {
		return aes.NewCipher(key)
	}

	// Two-key triple DES is K1, K2, K1.
	return des.NewTripleDESCipher(append(append([]byte{}, key...), key[:8]...)) // #nosec G405 -- TS 102 225 clause 5.1.3.1
}

// crypt ciphers or deciphers data, of whole blocks, in CBC mode with a zero initial chaining value (TS 102 225 clauses
// 5.1.2 and 5.1.3.1).
func (k Keyset) crypt(data []byte, encrypt bool) ([]byte, error) {
	b, err := k.block(k.KIc)
	if err != nil {
		return nil, err
	}

	if len(data)%b.BlockSize() != 0 {
		return nil, fmt.Errorf("%d octets is not a multiple of the block size", len(data))
	}

	out := make([]byte, len(data))
	iv := make([]byte, b.BlockSize())

	if encrypt {
		cipher.NewCBCEncrypter(b, iv).CryptBlocks(out, data) // #nosec G407 -- TS 102 225 clauses 5.1.2 and 5.1.3.1 mandate a zero ICV
	} else {
		cipher.NewCBCDecrypter(b, iv).CryptBlocks(out, data)
	}

	return out, nil
}

// checksum is the cryptographic checksum of data: for triple DES, the last block of a CBC-MAC with a zero initial
// chaining value of data padded with '00' (TS 102 225 clause 5.1.3.1); for AES, the CMAC of NIST SP 800-38B, of its
// first CCLength octets.
func (k Keyset) checksum(data []byte) ([]byte, error) {
	b, err := k.block(k.KID)
	if err != nil {
		return nil, err
	}

	if k.Algorithm == AES {
		return cmac(b, data)[:k.ccLength()], nil
	}

	if r := len(data) % b.BlockSize(); r != 0 {
		data = append(append([]byte{}, data...), make([]byte, b.BlockSize()-r)...)
	}

	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(b, make([]byte, b.BlockSize())).CryptBlocks(out, data) // #nosec G407 -- TS 102 225 clause 5.1.3.1 mandates a zero ICV

	return out[len(out)-b.BlockSize():], nil
}

// cmac is the CMAC of data with an AES block cipher (NIST SP 800-38B).
func cmac(b cipher.Block, data []byte) []byte {
	const rb = 0x87

	size := b.BlockSize()
	l := make([]byte, size)
	b.Encrypt(l, l)

	subkey := func(in []byte) []byte {
		out := make([]byte, len(in))

		var carry byte
		for i := len(in) - 1; i >= 0; i-- {
			out[i] = in[i]<<1 | carry
			carry = in[i] >> 7
		}

		if in[0]&0x80 != 0 {
			out[len(out)-1] ^= rb
		}

		return out
	}

	k1 := subkey(l)
	k2 := subkey(k1)

	n := (len(data) + size - 1) / size
	complete := n > 0 && len(data)%size == 0

	if n == 0 {
		n = 1
	}

	last := make([]byte, size)
	if complete {
		copy(last, data[(n-1)*size:])
		subtle.XORBytes(last, last, k1)
	} else {
		rest := data[(n-1)*size:]
		copy(last, rest)
		last[len(rest)] = 0x80
		subtle.XORBytes(last, last, k2)
	}

	x := make([]byte, size)
	for i := range n - 1 {
		subtle.XORBytes(x, x, data[i*size:(i+1)*size])
		b.Encrypt(x, x)
	}

	subtle.XORBytes(x, x, last)
	b.Encrypt(x, x)

	return x
}

// command is a Command Packet, its secured data unpacked (TS 102 225 clause 5.1, TS 31.115 clause 4.2).
type command struct {
	spi      uint16
	kic, kid byte
	tar      [3]byte
	counter  uint64
	cipher   *Keyset
	verified *Keyset
	data     []byte
}

func (c command) authenticated() bool {
	return c.verified != nil
}

func (c command) keyVersion() uint8 {
	switch {
	case c.verified != nil:
		return c.verified.Version
	case c.cipher != nil:
		return c.cipher.Version
	default:
		return 0
	}
}

func (c command) keyVersionsDiffer() bool {
	return c.kic>>4 != 0 && c.kid>>4 != 0 && c.kic>>4 != c.kid>>4
}

func (c command) usesAES() bool {
	return c.cipher != nil && c.cipher.Algorithm == AES || c.verified != nil && c.verified.Algorithm == AES
}

func (c command) sufficient(msl byte) bool {
	spi1 := byte(c.spi >> 8)

	if spi1&0x03 < msl&0x03 || spi1&0x04 < msl&0x04 || spi1&0x18 < msl&0x18 {
		return false
	}

	return !c.usesAES() || c.spi&spiCounter >= spiCounterHigher
}

// errUnauthenticated is a Command Packet whose Sending Entity the card cannot authenticate: of a key set or algorithm
// it does not have, which does not decipher, or whose checksum does not match; or whose SPI asks for a Response Packet
// that the rules of TS 102 225 clause 5.1.1 do not allow, which the card treats so (TS 102 225 clause 4.1, item 4).
var errUnauthenticated = errors.New("the card cannot authenticate the Sending Entity")

// errUnrecognized is a Command Header that the card cannot interpret (TS 102 225 clause 4.1, item 5).
var errUnrecognized = errors.New("unrecognizable command header")

// unpack reads the Command Packet that starts the user data ud of an SMS-DELIVER, the CPI included (TS 31.115 clause
// 4.2), and authenticates and deciphers it with the card's key sets, by their versions.
func unpack(ud []byte, keysets map[uint8]Keyset) (command, error) {
	if !bytes.HasPrefix(ud, commandUDH) {
		return command{}, fmt.Errorf("%w: no command packet identifier", errUnrecognized)
	}

	p := ud[len(commandUDH):]

	// CPL 2, CHL 1, SPI 2, KIc 1, KID 1, TAR 3, of lengths not coded as BER-TLV (TS 31.115 table 2).
	if len(p) < 10 {
		return command{}, fmt.Errorf("%w: %d octets", errUnrecognized, len(p))
	}

	cpl, chl := int(p[0])<<8|int(p[1]), int(p[2])
	c := command{spi: uint16(p[3])<<8 | uint16(p[4]), kic: p[5], kid: p[6]}
	copy(c.tar[:], p[7:10])

	if cpl != len(p)-2 {
		return command{}, fmt.Errorf("%w: CPL %d of %d octets", errUnrecognized, cpl, len(p)-2)
	}

	ciphered := c.spi&spiCiphering != 0

	var cc *Keyset

	switch c.spi & spiIntegrity {
	case 0:
	case spiIntegrityCC:
		k, ok := keysets[c.kid>>4]
		if !ok || Algorithm(c.kid&0x0f) != k.Algorithm {
			return command{}, fmt.Errorf("%w: no key set for KID %#02x", errUnauthenticated, c.kid)
		}

		cc = &k
	default:
		// A redundancy check or a digital signature, which the card does not support.
		return command{}, fmt.Errorf("%w: SPI %04x", errUnauthenticated, c.spi)
	}

	ccLen := 0
	if cc != nil {
		ccLen = cc.ccLength()
	}

	// CHL counts SPI to the end of the RC/CC/DS: SPI 2, KIc 1, KID 1, TAR 3, CNTR 5, PCNTR 1.
	if chl != 13+ccLen {
		return command{}, fmt.Errorf("%w: CHL %d, want %d", errUnrecognized, chl, 13+ccLen)
	}

	if err := checkPoRSecurity(c.spi); err != nil {
		return command{}, err
	}

	header, secured := p[:10], p[10:]

	if ciphered {
		k, ok := keysets[c.kic>>4]
		if !ok || Algorithm(c.kic&0x0f) != k.Algorithm {
			return command{}, fmt.Errorf("%w: no key set for KIc %#02x", errUnauthenticated, c.kic)
		}

		plain, err := k.crypt(secured, false)
		if err != nil {
			return command{}, fmt.Errorf("%w: %w", errUnauthenticated, err)
		}

		secured, c.cipher = plain, &k
	}

	if len(secured) < counterLength+1+ccLen {
		return command{}, fmt.Errorf("%w: %d octets of secured data", errUnauthenticated, len(secured))
	}

	for _, b := range secured[:counterLength] {
		c.counter = c.counter<<8 | uint64(b)
	}

	padding := int(secured[counterLength])
	data := secured[counterLength+1+ccLen:]

	if cc != nil {
		// CPL and CHL are part of the checksum, to a length modulo 8 before it (TS 31.115 clause 4.2).
		want, err := cc.checksum(concat(header, secured[:counterLength+1], data))
		if err != nil {
			return command{}, fmt.Errorf("%w: %w", errUnauthenticated, err)
		}

		if subtle.ConstantTimeCompare(want, secured[counterLength+1:counterLength+1+ccLen]) != 1 {
			return command{}, fmt.Errorf("%w: cryptographic checksum mismatch", errUnauthenticated)
		}

		c.verified = cc
	}

	if padding > len(data) {
		return command{}, fmt.Errorf("%w: %d padding octets in %d octets of data", errUnauthenticated, padding, len(data))
	}

	c.data = data[:len(data)-padding]

	return c, nil
}

// checkPoRSecurity checks the rules of the security of a Response Packet that the SPI asks for (TS 102 225 clause
// 5.1.1): the same integrity as the command's, and ciphering only of a ciphered command with a cryptographic
// checksum. A violation is processed as a Sending Entity that the card cannot authenticate.
func checkPoRSecurity(spi uint16) error {
	if spi&spiPoR == 0x0003 {
		return fmt.Errorf("%w: SPI %04x asks for a reserved PoR", errUnrecognized, spi)
	}

	porIntegrity := spi & spiPoRIntegrity >> 2
	if porIntegrity != 0 && porIntegrity != spi&spiIntegrity>>8 {
		return fmt.Errorf("%w: SPI %04x asks for a PoR integrity of another kind than the command's", errUnauthenticated, spi)
	}

	if spi&spiPoRCiphering != 0 && (spi&spiCiphering == 0 || spi&spiIntegrity != spiIntegrityCC) {
		return fmt.Errorf("%w: SPI %04x asks for a ciphered PoR to an unauthenticated or unciphered command", errUnauthenticated, spi)
	}

	return nil
}

// checkCounter checks the counter of an authenticated command against last, the key set's, as the SPI asks (TS 102
// 225 clauses 5.1.1 and 5.1.4), and returns the key set's counter after it, and the status of a failure, if any.
func checkCounter(c command, last uint64) (uint64, Status, bool) {
	switch c.spi & spiCounter {
	case spiCounterNone, spiCounterInfo:
		return last, StatusPoROK, true
	}

	switch {
	case last == maxCounter:
		return last, StatusCNTRBlocked, false
	case c.counter <= last:
		return last, StatusCNTRLow, false
	case c.spi&spiCounter == spiCounterOneMore && c.counter != last+1:
		return last, StatusCNTRHigh, false
	}

	return c.counter, StatusPoROK, true
}

// response is a Response Packet (TS 102 225 clause 5.2).
type response struct {
	tar     [3]byte
	counter uint64
	status  Status
	// data is the additional response data, which is empty for a status other than PoR OK.
	data []byte
}

// pack codes r in the user data of a single short message, the RPI included, secured as the command's SPI asks of a
// Response Packet, with the key set that authenticated the command (TS 102 225 clause 5.2, TS 31.115 clause 4.4).
func (r response) pack(c command) ([]byte, error) {
	ciphered, checksummed := c.spi&spiPoRCiphering != 0, c.spi&spiPoRIntegrity>>2 == spiIntegrityCC>>8

	if ciphered && c.cipher == nil || checksummed && c.verified == nil {
		return nil, errors.New("usim: a secured PoR to a command not authenticated or not ciphered")
	}

	ccLen, blockSize := 0, 1
	if checksummed {
		ccLen = c.verified.ccLength()
	}

	if ciphered {
		b, err := c.cipher.block(c.cipher.KIc)
		if err != nil {
			return nil, err
		}

		blockSize = b.BlockSize()
	}

	data := r.data
	padding := 0

	// Ciphering covers CNTR, PCNTR, the status, the RC/CC/DS and the data, padded to the block.
	if rem := (counterLength + 2 + ccLen + len(data)) % blockSize; rem != 0 {
		padding = blockSize - rem
		data = append(append([]byte{}, data...), make([]byte, padding)...)
	}

	// RPL counts RHL to the end of the data, RHL TAR to the end of the RC/CC/DS, of lengths not coded as BER-TLV (TS
	// 31.115 table 3).
	rpl := 1 + 3 + counterLength + 2 + ccLen + len(data)
	header := []byte{byte(rpl >> 8), byte(rpl), byte(3 + counterLength + 2 + ccLen), r.tar[0], r.tar[1], r.tar[2]}

	counter := make([]byte, counterLength+2)
	for i := range counterLength {
		counter[i] = byte(r.counter >> (8 * (counterLength - 1 - i)))
	}

	counter[counterLength], counter[counterLength+1] = byte(padding), byte(r.status)

	var cc []byte

	if checksummed {
		var err error

		// The UDHL, the RPI and its length, RPL and RHL are part of the checksum (TS 31.115 clause 4.4).
		cc, err = c.verified.checksum(concat(responseUDH, header, counter, data))
		if err != nil {
			return nil, err
		}
	}

	secured := concat(counter, cc, data)

	if ciphered {
		var err error

		secured, err = c.cipher.crypt(secured, true)
		if err != nil {
			return nil, err
		}
	}

	return concat(responseUDH, header, secured), nil
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}

	return out
}
