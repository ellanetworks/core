package suci

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/elliptic"
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

	"github.com/yeastengine/ella/internal/logger"
	"golang.org/x/crypto/curve25519"
)

type SuciProfile struct {
	ProtectionScheme string `yaml:"ProtectionScheme,omitempty"`
	PrivateKey       string `yaml:"PrivateKey,omitempty"`
	PublicKey        string `yaml:"PublicKey,omitempty"`
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

// modified from https://stackoverflow.com/questions/46283760/
// how-to-uncompress-a-single-x9-62-compressed-point-on-an-ecdh-p256-curve-in-go.
func uncompressKey(compressedBytes []byte) (*big.Int, *big.Int) {
	// Split the sign byte from the rest
	signByte := uint(compressedBytes[0])
	xBytes := compressedBytes[1:]

	x := new(big.Int).SetBytes(xBytes)
	three := big.NewInt(3)

	// The params for P256
	c := elliptic.P256().Params()

	// The equation is y^2 = x^3 - 3x + b
	// x^3, mod P
	xCubed := new(big.Int).Exp(x, three, c.P)

	// 3x, mod P
	threeX := new(big.Int).Mul(x, three)
	threeX.Mod(threeX, c.P)

	// x^3 - 3x + b mod P
	ySquared := new(big.Int).Sub(xCubed, threeX)
	ySquared.Add(ySquared, c.B)
	ySquared.Mod(ySquared, c.P)

	// find the square root mod P
	y := new(big.Int).ModSqrt(ySquared, c.P)
	if y == nil {
		// If this happens then you're dealing with an invalid point.
		logger.UtilLog.Errorln("uncompressed key with invalid point")
		return nil, nil
	}

	// Finally, check if you have the correct root. If not you want -y mod P
	if y.Bit(0) != signByte&1 {
		y.Neg(y)
		y.Mod(y, c.P)
	}
	// logger.UtilLog.Debugf("xUncom: %x\nyUncon: %x", x, y)
	return x, y
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

func profileB(input, supiType, privateKey string) (string, error) {
	logger.UtilLog.Infoln("suciToSupi Profile B")
	s, hexDecodeErr := hex.DecodeString(input)
	if hexDecodeErr != nil {
		logger.UtilLog.Errorln("hex DecodeString error")
		return "", hexDecodeErr
	}

	var ProfileBPubKeyLen int // p256, module q = 2^256 - 2^224 + 2^192 + 2^96 - 1
	var uncompressed bool
	if s[0] == 0x02 || s[0] == 0x03 {
		ProfileBPubKeyLen = 33 // ceil(log(2, q)/8) + 1 = 33
		uncompressed = false
	} else if s[0] == 0x04 {
		ProfileBPubKeyLen = 65 // 2*ceil(log(2, q)/8) + 1 = 65
		uncompressed = true
	} else {
		logger.UtilLog.Errorln("input error")
		return "", fmt.Errorf("suci input error")
	}

	// logger.UtilLog.Debugf("len:%d %d", len(s), ProfileBPubKeyLen + ProfileBMacLen)
	if len(s) < ProfileBPubKeyLen+ProfileBMacLen {
		logger.UtilLog.Errorln("len of input data is too short")
		return "", fmt.Errorf("suci input too short")
	}
	decryptPublicKey := s[:ProfileBPubKeyLen]
	decryptMac := s[len(s)-ProfileBMacLen:]
	decryptCipherText := s[ProfileBPubKeyLen : len(s)-ProfileBMacLen]
	// logger.UtilLog.Debugf("dePub: %x deCiph: %x deMac: %x", decryptPublicKey, decryptCipherText, decryptMac)

	// test data from TS33.501 Annex C.4
	// bHNPriv, _ := hex.DecodeString("F1AB1074477EBCC7F554EA1C5FC368B1616730155E0041AC447D6301975FECDA")
	var bHNPriv []byte
	if bHNPrivTmp, err := hex.DecodeString(privateKey); err != nil {
		logger.UtilLog.Errorf("decode error: %+v", err)
	} else {
		bHNPriv = bHNPrivTmp
	}

	var xUncompressed, yUncompressed *big.Int
	if uncompressed {
		xUncompressed = new(big.Int).SetBytes(decryptPublicKey[1:(ProfileBPubKeyLen/2 + 1)])
		yUncompressed = new(big.Int).SetBytes(decryptPublicKey[(ProfileBPubKeyLen/2 + 1):])
	} else {
		xUncompressed, yUncompressed = uncompressKey(decryptPublicKey)
		if xUncompressed == nil || yUncompressed == nil {
			logger.UtilLog.Errorln("uncompressed key has invalid point")
			return "", fmt.Errorf("key uncompression error")
		}
	}
	// logger.UtilLog.Debugf("xUncom: %x yUncom: %x", xUncompressed, yUncompressed)

	// x-coordinate is the shared key
	decryptSharedKey, _ := elliptic.P256().ScalarMult(xUncompressed, yUncompressed, bHNPriv)
	// logger.UtilLog.Debugf("deShared: %x", decryptSharedKey.Bytes())

	decryptPublicKeyForKDF := decryptPublicKey
	if uncompressed {
		decryptPublicKeyForKDF = CompressKey(decryptPublicKey, yUncompressed)
	}

	kdfKey := AnsiX963KDF(decryptSharedKey.Bytes(), decryptPublicKeyForKDF, ProfileBEncKeyLen, ProfileBMacKeyLen,
		ProfileBHashLen)
	// logger.UtilLog.Debugf("kdfKey: %x", kdfKey)
	decryptEncKey := kdfKey[:ProfileBEncKeyLen]
	decryptIcb := kdfKey[ProfileBEncKeyLen : ProfileBEncKeyLen+ProfileBIcbLen]
	decryptMacKey := kdfKey[len(kdfKey)-ProfileBMacKeyLen:]
	// logger.UtilLog.Debugf("deEncKey(size%d): %x deMacKey: %x deIcb: %x", len(decryptEncKey), decryptEncKey, decryptMacKey,
	// decryptIcb)

	decryptMacTag := HmacSha256(decryptCipherText, decryptMacKey, ProfileBMacLen)
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
	imsiPrefix     = "imsi-"
	nullScheme     = "0"
	profileAScheme = "1"
	profileBScheme = "2"
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

	supiPrefix := imsiPrefix
	if suciPrefix == "suci" && suciPart[supiTypePlace] == typeIMSI {
		supiPrefix = imsiPrefix
		logger.UtilLog.Infoln("supi type is IMSI")
	}

	if scheme == nullScheme { // NULL scheme
		return supiPrefix + mccMnc + suciPart[len(suciPart)-1], nil
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
			return supiPrefix + mccMnc + profileAResult, nil
		}
	} else if scheme == profileBScheme {
		if profileBResult, err := profileB(suciPart[len(suciPart)-1], suciPart[supiTypePlace], privateKey); err != nil {
			return "", err
		} else {
			return supiPrefix + mccMnc + profileBResult, nil
		}
	} else {
		return "", fmt.Errorf("protect Scheme (%s) is not supported", scheme)
	}
}
