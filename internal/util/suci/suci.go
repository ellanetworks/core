// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package suci

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
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

const (
	ProtectionScheme  = "1"
	ProfileAMacKeyLen = 32 // octets
	ProfileAEncKeyLen = 16 // octets
	ProfileAIcbLen    = 16 // octets
	ProfileAMacLen    = 8  // octets
	ProfileAHashLen   = 32 // octets
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

func aes128ctr(input, encKey []byte) ([]byte, error) {
	output := make([]byte, len(input))
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("AES128 CTR cipher creation error: %v", err)
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("failed to generate random IV: %v", err)
	}

	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(output, input)

	return output, nil
}

func ansiX963KDF(sharedKey, publicKey []byte, profileEncKeyLen, profileMacKeyLen, profileHashLen int) []byte {
	var counter uint32 = 0x00000001
	var kdfKey []byte
	kdfRounds := int(math.Ceil(float64(profileEncKeyLen+profileMacKeyLen) / float64(profileHashLen)))
	for i := 1; i <= kdfRounds; i++ {
		counterBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(counterBytes, counter)
		tmpK := sha256.Sum256(append(append(sharedKey, counterBytes...), publicKey...))
		sliceK := tmpK[:]
		kdfKey = append(kdfKey, sliceK...)
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
		return "", fmt.Errorf("suci input too short: got %d, want %d", len(s), ProfileAPubKeyLen+ProfileAMacLen)
	}

	decryptMac := s[len(s)-ProfileAMacLen:]
	decryptPublicKey := s[:ProfileAPubKeyLen]
	decryptCipherText := s[ProfileAPubKeyLen : len(s)-ProfileAMacLen]

	// test data from TS33.501 Annex C.4
	var aHNPriv []byte
	if aHNPrivTmp, err := hex.DecodeString(privateKey); err != nil {
		logger.UtilLog.Errorf("decode error: %+v", err)
	} else {
		aHNPriv = aHNPrivTmp
	}
	var decryptSharedKey []byte
	if decryptSharedKeyTmp, err := curve25519.X25519(aHNPriv, decryptPublicKey); err != nil {
		logger.UtilLog.Errorf("X25519 error: %+v", err)
	} else {
		decryptSharedKey = decryptSharedKeyTmp
	}

	kdfKey := ansiX963KDF(decryptSharedKey, decryptPublicKey, ProfileAEncKeyLen, ProfileAMacKeyLen, ProfileAHashLen)
	decryptEncKey := kdfKey[:ProfileAEncKeyLen]
	decryptMacKey := kdfKey[len(kdfKey)-ProfileAMacKeyLen:]

	decryptMacTag := hmacSha256(decryptCipherText, decryptMacKey, ProfileAMacLen)
	if bytes.Equal(decryptMacTag, decryptMac) {
		logger.UtilLog.Infoln("decryption MAC match")
	} else {
		return "", fmt.Errorf("decryption MAC failed")
	}

	decryptPlainText, err := aes128ctr(decryptCipherText, decryptEncKey)
	if err != nil {
		return "", fmt.Errorf("AES decryption error: %v", err)
	}

	return calcSchemeResult(decryptPlainText, supiType), nil
}

// suci-0(SUPI type)-mcc-mnc-routingIndentifier-protectionScheme-homeNetworkPublicKeyIdentifier-schemeOutput.
const (
	supiTypePlace = 1
	mccPlace      = 2
	mncPlace      = 3
	schemePlace   = 5
)

const (
	typeIMSI = "0"
)

func ToSupi(suci string, privateKey string) (string, error) {
	suciPart := strings.Split(suci, "-")
	suciPrefix := suciPart[0]
	if suciPrefix == "imsi" || suciPrefix == "nai" {
		logger.UtilLog.Infoln("got supi")
		return suci, nil
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

	if scheme != ProtectionScheme {
		return "", fmt.Errorf("protect Scheme mismatch [%s:%s]", scheme, ProtectionScheme)
	}

	if profileAResult, err := profileA(suciPart[len(suciPart)-1], suciPart[supiTypePlace], privateKey); err != nil {
		return "", err
	} else {
		return mccMnc + profileAResult, nil
	}
}
