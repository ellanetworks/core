// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package udm

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
	"math/big"
	"math/bits"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"golang.org/x/crypto/curve25519"
)

type SuciProfile struct {
	ProtectionScheme string
	PrivateKey       string
	PublicKey        string
}

// profile A.
const (
	ProfileAMacKeyLen = 32 // octets
	ProfileAEncKeyLen = 16 // octets
	ProfileAIcbLen    = 16 // octets
	ProfileAMacLen    = 8  // octets
	ProfileAHashLen   = 32 // octets
)

// profile B.
const (
	ProfileBMacKeyLen = 32 // octets
	ProfileBEncKeyLen = 16 // octets
	ProfileBIcbLen    = 16 // octets
	ProfileBMacLen    = 8  // octets
	ProfileBHashLen   = 32 // octets
)

func CompressKey(uncompressed []byte, y *big.Int) []byte {
	compressed := uncompressed[0:33]
	if y.Bit(0) == 1 { // 0x03
		compressed[0] = 0x03
	} else { // 0x02
		compressed[0] = 0x02
	}
	// logger.UtilLog.Debugf("compressed: %x", compressed)
	return compressed
}

func HmacSha256(input, macKey []byte, macLen int) []byte {
	h := hmac.New(sha256.New, macKey)
	if _, err := h.Write(input); err != nil {
		logger.UtilLog.Errorf("HMAC SHA256 error %+v", err)
	}
	macVal := h.Sum(nil)
	macTag := macVal[:macLen]
	// logger.UtilLog.Debugf("macVal: %x\nmacTag: %x", macVal, macTag)
	return macTag
}

func Aes128ctr(input, encKey, icb []byte) []byte {
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

func AnsiX963KDF(sharedKey, publicKey []byte, profileEncKeyLen, profileMacKeyLen, profileHashLen int) []byte {
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
	logger.UtilLog.Infoln("suciToSupi Profile A")
	s, hexDecodeErr := hex.DecodeString(input)
	if hexDecodeErr != nil {
		logger.UtilLog.Errorln("hex DecodeString error")
		return "", hexDecodeErr
	}

	// for X25519(profile A), q (The number of elements in the field Fq) = 2^255 - 19
	// len(pubkey) is therefore ceil((log2q)/8+1) = 32octets
	ProfileAPubKeyLen := 32
	if len(s) < ProfileAPubKeyLen+ProfileAMacLen {
		logger.UtilLog.Errorln("len of input data is too short")
		return "", fmt.Errorf("suci input too short")
	}

	decryptMac := s[len(s)-ProfileAMacLen:]
	decryptPublicKey := s[:ProfileAPubKeyLen]
	decryptCipherText := s[ProfileAPubKeyLen : len(s)-ProfileAMacLen]
	// logger.UtilLog.Debugf("dePub: %x deCiph: %x deMac: %x", decryptPublicKey, decryptCipherText, decryptMac)

	// test data from TS33.501 Annex C.4
	// aHNPriv, _ := hex.DecodeString("c53c2208b61860b06c62e5406a7b330c2b577aa5558981510d128247d38bd1d")
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
	// logger.UtilLog.Debugf("deShared: %x", decryptSharedKey)

	kdfKey := AnsiX963KDF(decryptSharedKey, decryptPublicKey, ProfileAEncKeyLen, ProfileAMacKeyLen, ProfileAHashLen)
	decryptEncKey := kdfKey[:ProfileAEncKeyLen]
	decryptIcb := kdfKey[ProfileAEncKeyLen : ProfileAEncKeyLen+ProfileAIcbLen]
	decryptMacKey := kdfKey[len(kdfKey)-ProfileAMacKeyLen:]
	// logger.UtilLog.Debugf("deEncKey(size%d): %x deMacKey: %x deIcb: %x", len(decryptEncKey), decryptEncKey, decryptMacKey,
	// decryptIcb)

	decryptMacTag := HmacSha256(decryptCipherText, decryptMacKey, ProfileAMacLen)
	if bytes.Equal(decryptMacTag, decryptMac) {
		logger.UtilLog.Infoln("decryption MAC match")
	} else {
		logger.UtilLog.Errorln("decryption MAC failed")
		return "", fmt.Errorf("decryption MAC failed")
	}

	decryptPlainText := Aes128ctr(decryptCipherText, decryptEncKey, decryptIcb)

	return calcSchemeResult(decryptPlainText, supiType), nil
}

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
)

func ToSupi(suci string, suciProfiles []SuciProfile) (string, error) {
	suciPart := strings.Split(suci, "-")
	logger.UtilLog.Infof("suciPart: %+v", suciPart)

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

	logger.UtilLog.Infof("scheme %s", suciPart[schemePlace])
	scheme := suciPart[schemePlace]
	mccMnc := suciPart[mccPlace] + suciPart[mncPlace]

	if suciPrefix == "suci" && suciPart[supiTypePlace] == typeIMSI {
		logger.UtilLog.Infoln("supi type is IMSI")
	}

	if scheme == nullScheme { // NULL scheme
		return mccMnc + suciPart[len(suciPart)-1], nil
	}

	// (HNPublicKeyID-1) is the index of "suciProfiles" slices
	keyIndex, err := strconv.Atoi(suciPart[HNPublicKeyIDPlace])
	if err != nil {
		return "", fmt.Errorf("parse HNPublicKeyID error: %+v", err)
	}
	if keyIndex > len(suciProfiles) {
		return "", fmt.Errorf("keyIndex(%d) out of range(%d)", keyIndex, len(suciProfiles))
	}

	protectScheme := suciProfiles[keyIndex-1].ProtectionScheme
	privateKey := suciProfiles[keyIndex-1].PrivateKey

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
