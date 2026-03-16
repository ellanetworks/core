// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
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

	"github.com/ellanetworks/core/etsi"
)

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
	ProfileBMacKeyLen = 32
	ProfileBEncKeyLen = 16
	ProfileBIcbLen    = 16
	ProfileBMacLen    = 8
	ProfileBHashLen   = 32
)

// suci-0(SUPI type)-mcc-mnc-routingIdentifier-protectionScheme-homeNetworkPublicKeyIdentifier-schemeOutput.
const (
	supiTypePlace = 1
	mccPlace      = 2
	mncPlace      = 3
	schemePlace   = 5
	keyIdPlace    = 6
)

const (
	typeIMSI       = "0"
	nullScheme     = "0"
	profileAScheme = "1"
	profileBScheme = "2"
)

func hmacSha256(input, macKey []byte, macLen int) ([]byte, error) {
	h := hmac.New(sha256.New, macKey)

	if _, err := h.Write(input); err != nil {
		return nil, err
	}

	macVal := h.Sum(nil)
	macTag := macVal[:macLen]

	return macTag, nil
}

func aes128ctr(input, encKey, icb []byte) ([]byte, error) {
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("error creating AES cipher: %w", err)
	}

	stream := cipher.NewCTR(block, icb)

	output := make([]byte, len(input))
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
	if supiType != typeIMSI {
		return hex.EncodeToString(decryptPlainText)
	}

	schemeResult := hex.EncodeToString(swapNibbles(decryptPlainText))
	if schemeResult[len(schemeResult)-1] == 'f' {
		schemeResult = schemeResult[:len(schemeResult)-1]
	}

	return schemeResult
}

// decryptWithKdf runs the shared KDF → MAC-verify → AES-CTR-decrypt pipeline.
func decryptWithKdf(sharedKey, kdfPubKey, cipherText, providedMac []byte,
	encKeyLen, macKeyLen, hashLen, icbLen, macLen int, //nolint:unparam
) ([]byte, error) {
	kdfKey := ansiX963KDF(sharedKey, kdfPubKey, encKeyLen, macKeyLen, hashLen)
	encKey := kdfKey[:encKeyLen]
	icb := kdfKey[encKeyLen : encKeyLen+icbLen]
	macKey := kdfKey[len(kdfKey)-macKeyLen:]

	computedMac, err := hmacSha256(cipherText, macKey, macLen)
	if err != nil {
		return nil, err
	}

	if !hmac.Equal(computedMac, providedMac) {
		return nil, fmt.Errorf("decryption MAC failed")
	}

	return aes128ctr(cipherText, encKey, icb) // #nosec G407
}

// ecdhX25519 performs X25519 ECDH using the standard library crypto/ecdh.
func ecdhX25519(privateKeyHex string, peerPubKey []byte) ([]byte, error) {
	privBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode X25519 private key: %w", err)
	}

	priv, err := ecdh.X25519().NewPrivateKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse X25519 private key: %w", err)
	}

	pub, err := ecdh.X25519().NewPublicKey(peerPubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse X25519 public key: %w", err)
	}

	return priv.ECDH(pub)
}

var errPublicKeyUnmarshalling = fmt.Errorf("failed to unmarshal P-256 public key")

// ecdhP256 performs P-256 ECDH, handling both compressed (33 bytes) and
// uncompressed (65 bytes) ephemeral public keys. It always returns the
// compressed form for KDF input as required by TS 33.501.
func ecdhP256(privateKeyHex string, transmittedPubKey []byte) (sharedKey, kdfPubKey []byte, err error) {
	privBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode P-256 private key: %w", err)
	}

	priv, err := ecdh.P256().NewPrivateKey(privBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse P-256 private key: %w", err)
	}

	var pubKeyForECDH []byte

	switch transmittedPubKey[0] {
	case 0x02, 0x03:
		// Compressed format (33 bytes) — decompress for ECDH.
		x, y := elliptic.UnmarshalCompressed(elliptic.P256(), transmittedPubKey)
		if x == nil || y == nil {
			return nil, nil, errPublicKeyUnmarshalling
		}
		// Manually construct uncompressed SEC1 (0x04 || x || y) to avoid deprecated elliptic.Marshal.
		pubKeyForECDH = make([]byte, 65)
		pubKeyForECDH[0] = 0x04
		x.FillBytes(pubKeyForECDH[1:33])
		y.FillBytes(pubKeyForECDH[33:65])
		// KDF uses the original compressed form.
		kdfPubKey = transmittedPubKey

	case 0x04:
		// Uncompressed format (65 bytes) — validate point before compressing,
		// since MarshalCompressed panics on invalid curve points.
		if _, err := ecdh.P256().NewPublicKey(transmittedPubKey); err != nil {
			return nil, nil, errPublicKeyUnmarshalling
		}

		pubKeyForECDH = transmittedPubKey
		// KDF needs the compressed form.
		x := new(big.Int).SetBytes(transmittedPubKey[1:33])
		y := new(big.Int).SetBytes(transmittedPubKey[33:65])
		kdfPubKey = elliptic.MarshalCompressed(elliptic.P256(), x, y)

	default:
		return nil, nil, fmt.Errorf("unknown public key format byte: 0x%02x", transmittedPubKey[0])
	}

	pub, err := ecdh.P256().NewPublicKey(pubKeyForECDH)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create P-256 public key: %w", err)
	}

	sharedKey, err = priv.ECDH(pub)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute ECDH: %w", err)
	}

	return sharedKey, kdfPubKey, nil
}

func profileA(input, supiType, privateKey string) (string, error) {
	s, err := hex.DecodeString(input)
	if err != nil {
		return "", fmt.Errorf("error decoding hex string: %w", err)
	}

	const ProfileAPubKeyLen = 32
	if len(s) < ProfileAPubKeyLen+ProfileAMacLen {
		return "", fmt.Errorf("suci input too short")
	}

	peerPubKey := s[:ProfileAPubKeyLen]
	cipherText := s[ProfileAPubKeyLen : len(s)-ProfileAMacLen]
	providedMac := s[len(s)-ProfileAMacLen:]

	sharedKey, err := ecdhX25519(privateKey, peerPubKey)
	if err != nil {
		return "", err
	}

	plainText, err := decryptWithKdf(sharedKey, peerPubKey, cipherText, providedMac,
		ProfileAEncKeyLen, ProfileAMacKeyLen, ProfileAHashLen, ProfileAIcbLen, ProfileAMacLen)
	if err != nil {
		return "", err
	}

	return calcSchemeResult(plainText, supiType), nil
}

func profileB(input, supiType, privateKey string) (string, error) {
	s, err := hex.DecodeString(input)
	if err != nil || len(s) < 1 {
		return "", fmt.Errorf("error decoding hex string: %w", err)
	}

	// Determine ephemeral public key length from format byte.
	var pubKeyLen int

	switch s[0] {
	case 0x02, 0x03:
		pubKeyLen = 33 // SEC1 compressed
	case 0x04:
		pubKeyLen = 65 // SEC1 uncompressed
	default:
		return "", fmt.Errorf("suci input error: unknown public key format byte 0x%02x", s[0])
	}

	if len(s) < pubKeyLen+ProfileBMacLen {
		return "", fmt.Errorf("suci input too short for Profile B")
	}

	transmittedPubKey := s[:pubKeyLen]
	cipherText := s[pubKeyLen : len(s)-ProfileBMacLen]
	providedMac := s[len(s)-ProfileBMacLen:]

	sharedKey, kdfPubKey, err := ecdhP256(privateKey, transmittedPubKey)
	if err != nil {
		return "", err
	}

	plainText, err := decryptWithKdf(sharedKey, kdfPubKey, cipherText, providedMac,
		ProfileBEncKeyLen, ProfileBMacKeyLen, ProfileBHashLen, ProfileBIcbLen, ProfileBMacLen)
	if err != nil {
		return "", err
	}

	return calcSchemeResult(plainText, supiType), nil
}

func schemeIDToLetter(schemeID string) (string, error) {
	switch schemeID {
	case profileAScheme: // "1"
		return "A", nil
	case profileBScheme: // "2"
		return "B", nil
	default:
		return "", fmt.Errorf("unsupported protection scheme: %s", schemeID)
	}
}

// KeyResolver resolves a home network private key by (scheme, keyIdentifier).
// scheme is "A" or "B". keyIdentifier is 0-255.
type KeyResolver func(scheme string, keyIdentifier int) (string, error)

func ToSupi(suci string, resolveKey KeyResolver) (etsi.SUPI, error) {
	suciPart := strings.Split(suci, "-")
	suciPrefix := suciPart[0]

	switch suciPrefix {
	case "imsi":
		return etsi.NewSUPIFromPrefixed(suci)
	case "nai":
		return etsi.InvalidSUPI, fmt.Errorf("NAI SUPI not yet supported")
	case "suci":
		if len(suciPart) < 8 {
			return etsi.InvalidSUPI, fmt.Errorf("suci with wrong format")
		}
	default:
		return etsi.InvalidSUPI, fmt.Errorf("unknown suciPrefix [%s]", suciPrefix)
	}

	scheme := suciPart[schemePlace]
	mccMnc := suciPart[mccPlace] + suciPart[mncPlace]

	if scheme == nullScheme {
		return etsi.NewSUPIFromIMSI(mccMnc + suciPart[len(suciPart)-1])
	}

	// Resolve the private key for this (scheme, keyId) pair.
	schemeLetter, err := schemeIDToLetter(scheme)
	if err != nil {
		return etsi.InvalidSUPI, err
	}

	keyIdStr := suciPart[keyIdPlace]

	keyId, err := strconv.Atoi(keyIdStr)
	if err != nil {
		return etsi.InvalidSUPI, fmt.Errorf("invalid key identifier: %s", keyIdStr)
	}

	privateKey, err := resolveKey(schemeLetter, keyId)
	if err != nil {
		return etsi.InvalidSUPI, fmt.Errorf("home network key not found (scheme=%s, keyId=%d): %w", schemeLetter, keyId, err)
	}

	schemeOutput := suciPart[len(suciPart)-1]
	supiType := suciPart[supiTypePlace]

	var result string

	switch scheme {
	case profileAScheme:
		result, err = profileA(schemeOutput, supiType, privateKey)
		if err != nil {
			return etsi.InvalidSUPI, fmt.Errorf("profile A error: %w", err)
		}
	case profileBScheme:
		result, err = profileB(schemeOutput, supiType, privateKey)
		if err != nil {
			return etsi.InvalidSUPI, fmt.Errorf("profile B error: %w", err)
		}
	}

	return etsi.NewSUPIFromIMSI(mccMnc + result)
}
