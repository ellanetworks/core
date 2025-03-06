// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package suci

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/bits"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"golang.org/x/crypto/curve25519"
)

// profile A.
const (
	ProfileAMacKeyLen = 32 // octets
	ProfileAEncKeyLen = 16 // octets
	ProfileAIcbLen    = 16 // octets
	ProfileAMacLen    = 8  // octets
	ProfileAHashLen   = 32 // octets
)

// suci-0(SUPI type)-mcc-mnc-routingIndentifier-protectionScheme-homeNetworkPublicKeyIdentifier-schemeOutput.
const (
	supiTypePlace      = 1
	mccPlace           = 2
	mncPlace           = 3
	schemePlace        = 5
	HNPublicKeyIDPlace = 6
)

const (
	typeIMSI       = "0"
	nullScheme     = "0"
	profileAScheme = "1"
	profileBScheme = "2"
)

func hmacSha256(input, macKey []byte, macLen int) []byte {
	h := hmac.New(sha256.New, macKey)
	if _, err := h.Write(input); err != nil {
		logger.UtilLog.Errorf("HMAC SHA256 error %+v", err)
	}
	macVal := h.Sum(nil)
	macTag := macVal[:macLen]
	return macTag
}

func aes128ctr(input, encKey, icb []byte) []byte {
	output := make([]byte, len(input))
	block, err := aes.NewCipher(encKey)
	if err != nil {
		logger.UtilLog.Errorf("AES128 CTR error %+v", err)
	}
	stream := cipher.NewCTR(block, icb)
	stream.XORKeyStream(output, input)
	// logger.UtilLog.Debugf("aes input: %x %x %x\naes output: %x", input, encKey, icb, output)
	return output
}

func ansiX963KDF(sharedKey, publicKey []byte, profileEncKeyLen, profileMacKeyLen, profileHashLen int) []byte {
	var counter uint32 = 0x00000001
	var kdfKey []byte
	kdfRounds := int(math.Ceil(float64(profileEncKeyLen+profileMacKeyLen) / float64(profileHashLen)))
	for i := 1; i <= kdfRounds; i++ {
		counterBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(counterBytes, counter)
		// logger.UtilLog.Debugf("counterBytes: %x", counterBytes)
		tmpK := sha256.Sum256(append(append(sharedKey, counterBytes...), publicKey...))
		sliceK := tmpK[:]
		kdfKey = append(kdfKey, sliceK...)
		// logger.UtilLog.Debugf("kdfKey in round %d: %x", i, kdfKey)
		counter++
	}
	return kdfKey
}

func swapNibbles(input []byte) []byte {
	output := make([]byte, len(input))
	for i, b := range input {
		output[i] = bits.RotateLeft8(b, 4)
	}
	return output
}

func calcSchemeResult(decryptPlainText []byte, supiType string) string {
	var schemeResult string
	if supiType == typeIMSI {
		schemeResult = hex.EncodeToString(swapNibbles(decryptPlainText))
		if schemeResult[len(schemeResult)-1] == 'f' {
			schemeResult = schemeResult[:len(schemeResult)-1]
		}
	} else {
		schemeResult = hex.EncodeToString(decryptPlainText)
	}
	return schemeResult
}

func profileA(input, supiType, privateKey string) (string, error) {
	s, hexDecodeErr := hex.DecodeString(input)
	if hexDecodeErr != nil {
		logger.UtilLog.Errorln("hex DecodeString error")
		return "", hexDecodeErr
	}

	// for X25519(profile A), q (The number of elements in the field Fq) = 2^255 - 19
	// len(pubkey) is therefore ceil((log2q)/8+1) = 32octets
	ProfileAPubKeyLen := 32
	if len(s) < ProfileAPubKeyLen+ProfileAMacLen {
		return "", fmt.Errorf("suci input too short")
	}

	decryptMac := s[len(s)-ProfileAMacLen:]
	decryptPublicKey := s[:ProfileAPubKeyLen]
	decryptCipherText := s[ProfileAPubKeyLen : len(s)-ProfileAMacLen]

	// test data from TS33.501 Annex C.4
	var aHNPriv []byte
	aHNPrivTmp, err := hex.DecodeString(privateKey)
	if err != nil {
		return "", fmt.Errorf("decode error: %w", err)
	}
	aHNPriv = aHNPrivTmp
	decryptSharedKeyTmp, err := curve25519.X25519(aHNPriv, decryptPublicKey)
	if err != nil {
		return "", fmt.Errorf("could not calculate shared key: %w", err)
	}
	decryptSharedKey := decryptSharedKeyTmp
	kdfKey := ansiX963KDF(decryptSharedKey, decryptPublicKey, ProfileAEncKeyLen, ProfileAMacKeyLen, ProfileAHashLen)
	decryptEncKey := kdfKey[:ProfileAEncKeyLen]
	decryptIcb := kdfKey[ProfileAEncKeyLen : ProfileAEncKeyLen+ProfileAIcbLen]
	decryptMacKey := kdfKey[len(kdfKey)-ProfileAMacKeyLen:]
	decryptMacTag := hmacSha256(decryptCipherText, decryptMacKey, ProfileAMacLen)
	if !bytes.Equal(decryptMacTag, decryptMac) {
		return "", fmt.Errorf("decryption MAC failed")
	}
	logger.UtilLog.Infoln("decryption MAC match")
	decryptPlainText := aes128ctr(decryptCipherText, decryptEncKey, decryptIcb) // #nosec G407
	return calcSchemeResult(decryptPlainText, supiType), nil
}

func ToSupi(suci string, privateKey string) (string, error) {
	suciPart := strings.Split(suci, "-")
	suciPrefix := suciPart[0]
	if suciPrefix == "imsi" || suciPrefix == "nai" {
		return "", fmt.Errorf("suci with wrong format")
	} else if suciPrefix == "suci" {
		if len(suciPart) < 6 {
			return "", fmt.Errorf("suci with wrong format")
		}
	} else {
		return "", fmt.Errorf("unknown suciPrefix [%s]", suciPrefix)
	}

	scheme := suciPart[schemePlace]
	mccMnc := suciPart[mccPlace] + suciPart[mncPlace]

	if suciPrefix == "suci" && suciPart[supiTypePlace] == typeIMSI {
		logger.UtilLog.Infoln("supi type is IMSI")
	}

	if scheme == nullScheme {
		return mccMnc + suciPart[len(suciPart)-1], nil
	}

	protectScheme := profileAScheme

	if scheme != protectScheme {
		return "", fmt.Errorf("protect Scheme mismatch [%s:%s]", scheme, protectScheme)
	}

	if scheme == profileAScheme {
		if profileAResult, err := profileA(suciPart[len(suciPart)-1], suciPart[supiTypePlace], privateKey); err != nil {
			return "", err
		} else {
			return mccMnc + profileAResult, nil
		}
	} else {
		return "", fmt.Errorf("protect Scheme (%s) is not supported", scheme)
	}
}
