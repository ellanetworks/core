/*
*
// Modified by Ella Networks Inc.
  - SPDX-License-Identifier: BUSL-1.1
  - SPDX-FileCopyrightText: Ella Networks Inc.
  - © Copyright 2019 The Free5GC Authors
  - © Copyright 2025 Free Mobile SAS
    *
  - Modified by Ella Networks.
*/
package sidf

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
)

// suci-0(SUPI type: IMSI)-mcc-mnc-routingIndicator-protectionScheme-homeNetworkPublicKeyID-schemeOutput.
const (
	PrefixIMSI     = "imsi-"
	PrefixSUCI     = "suci"
	SupiTypeIMSI   = "0"
	NullScheme     = "0"
	ProfileAScheme = "1"
	ProfileBScheme = "2"
)

var (
	mccRegex = `(?P<mcc>\d{3})`
	mncRegex = `(?P<mnc>\d{2,3})`

	imsiTypeRegex = fmt.Sprintf("(?P<imsiType>0-%s-%s)", mccRegex, mncRegex)

	// NAI type: variable-length domain name (RFC 7542 §2.2).
	naiTypeRegex = "(?P<naiType>1-.*)"

	// SUPI type: 0 = IMSI, 1 = NAI.
	supiTypeRegex = fmt.Sprintf("(?P<supi_type>%s|%s)",
		imsiTypeRegex,
		naiTypeRegex)

	routingIndicatorRegex = `(?P<routing_indicator>\d{1,4})`
	// Protection scheme: 0 = NULL (unencrypted), 1 = Profile A, 2 = Profile B.
	protectionSchemeRegex = `(?P<protection_scheme_id>(?:[0-2]))`
	publicKeyIDRegex      = `(?P<public_key_id>(?:\d{1,2}|1\d{2}|2[0-4]\d|25[0-5]))`
	// Unbounded hex is safe from ReDoS because the overall SUCI length is bounded.
	schemeOutputRegex = `(?P<scheme_output>[A-Fa-f0-9]+)`
	// SUCI grammar per 3GPP TS 29.503 Annex C.
	suciRegex = regexp.MustCompile(fmt.Sprintf("^suci-%s-%s-%s-%s-%s$",
		supiTypeRegex,
		routingIndicatorRegex,
		protectionSchemeRegex,
		publicKeyIDRegex,
		schemeOutputRegex,
	))
)

type Suci struct {
	SupiType         string
	Mcc              string
	Mnc              string
	HomeNetworkId    string
	RoutingIndicator string
	ProtectionScheme string
	PublicKeyID      string
	SchemeOutput     string

	Raw string
}

func ParseSuci(input string) *Suci {
	matches := suciRegex.FindStringSubmatch(input)
	if matches == nil {
		return nil
	}

	// Index gaps skip the nested imsiType subgroups in the SUCI pattern.
	return &Suci{
		SupiType:         matches[1],
		Mcc:              matches[3],
		Mnc:              matches[4],
		HomeNetworkId:    matches[5],
		RoutingIndicator: matches[6],
		ProtectionScheme: matches[7],
		PublicKeyID:      matches[8],
		SchemeOutput:     matches[9],

		Raw: input,
	}
}

type HomeNetworkPrivateKey struct {
	ProtectionScheme string           `yaml:"ProtectionScheme,omitempty"`
	PrivateKey       *ecdh.PrivateKey `yaml:"PrivateKey,omitempty"`
	PublicKey        *ecdh.PublicKey  `yaml:"PublicKey,omitempty"`
}

const (
	ProfileAMacKeyLen = 32 // octets
	ProfileAEncKeyLen = 16 // octets
	ProfileAIcbLen    = 16 // octets
	ProfileAMacLen    = 8  // octets
	ProfileAHashLen   = 32 // octets
)

const (
	ProfileBMacKeyLen = 32 // octets
	ProfileBEncKeyLen = 16 // octets
	ProfileBIcbLen    = 16 // octets
	ProfileBMacLen    = 8  // octets
	ProfileBHashLen   = 32 // octets
)

func HmacSha256(input, macKey []byte, macLen int) ([]byte, error) {
	h := hmac.New(sha256.New, macKey)

	if _, err := h.Write(input); err != nil {
		return nil, fmt.Errorf("HMAC SHA256 error: %w", err)
	}

	macVal := h.Sum(nil)

	return macVal[:macLen], nil
}

func Aes128ctr(input, encKey, icb []byte) ([]byte, error) {
	output := make([]byte, len(input))

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("AES128 CTR error: %w", err)
	}

	stream := cipher.NewCTR(block, icb)
	stream.XORKeyStream(output, input)

	return output, nil
}

func AnsiX963KDF(sharedKey, publicKey []byte, encKeyLen, macKeyLen, hashLen int) []byte {
	var counter uint32 = 1

	var kdfKey []byte

	kdfRounds := int(math.Ceil(float64(encKeyLen+macKeyLen) / float64(hashLen)))

	for i := 0; i < kdfRounds; i++ {
		counterBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(counterBytes, counter)
		tmpK := sha256.Sum256(append(append(sharedKey, counterBytes...), publicKey...))
		kdfKey = append(kdfKey, tmpK[:]...)
		counter++
	}

	return kdfKey
}

var ErrorPublicKeyUnmarshalling = fmt.Errorf("failed to unmarshal uncompressed public key")

func Tbcd(value string) string {
	valueBytes := []byte(value)
	for (len(valueBytes) % 2) != 0 {
		valueBytes = append(valueBytes, 'F')
	}

	for i := 1; i < len(valueBytes); i += 2 {
		valueBytes[i-1], valueBytes[i] = valueBytes[i], valueBytes[i-1]
	}

	i := len(valueBytes) - 1
	if valueBytes[i] == 'F' || valueBytes[i] == 'f' {
		valueBytes = valueBytes[:i]
	}

	return string(valueBytes)
}
