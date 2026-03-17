// Copyright 2026 Ella Networks
// Copyright 2019 free5gc.org

package ausf

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"testing"
)

// ------------- Profile A: low-level round-trip test -------------

// buildProfileASchemeOutput encrypts an MSIN using Profile A (X25519) so we
// can test decryption. This is the UE-side logic.
func buildProfileASchemeOutput(msin string, hnPubKeyHex string, _ string) (string, error) {
	hnPub, err := hex.DecodeString(hnPubKeyHex)
	if err != nil {
		return "", err
	}

	// Generate ephemeral key pair.
	ephPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}

	ephPub := ephPriv.PublicKey().Bytes()

	// Compute shared key.
	hnPubKey, err := ecdh.X25519().NewPublicKey(hnPub)
	if err != nil {
		return "", err
	}

	sharedKey, err := ephPriv.ECDH(hnPubKey)
	if err != nil {
		return "", err
	}

	// KDF
	kdfKey := ansiX963KDF(sharedKey, ephPub, ProfileAEncKeyLen, ProfileAMacKeyLen, ProfileAHashLen)
	encKey := kdfKey[:ProfileAEncKeyLen]
	icb := kdfKey[ProfileAEncKeyLen : ProfileAEncKeyLen+ProfileAIcbLen]
	macKey := kdfKey[len(kdfKey)-ProfileAMacKeyLen:]

	// Encode MSIN
	plainText := encodeMSIN(msin)

	// AES-128-CTR encrypt
	block, _ := aes.NewCipher(encKey)
	stream := cipher.NewCTR(block, icb)
	cipherText := make([]byte, len(plainText))
	stream.XORKeyStream(cipherText, plainText)

	// HMAC
	h := hmac.New(sha256.New, macKey)
	h.Write(cipherText)
	macTag := h.Sum(nil)[:ProfileAMacLen]

	// schemeOutput = ephPub || cipherText || macTag
	out := append(ephPub, cipherText...)
	out = append(out, macTag...)

	return hex.EncodeToString(out), nil
}

// encodeMSIN encodes an MSIN string to BCD-swapped bytes as per 3GPP.
func encodeMSIN(msin string) []byte {
	if len(msin)%2 != 0 {
		msin = msin + "f"
	}

	result := make([]byte, len(msin)/2)
	for i := 0; i < len(msin); i += 2 {
		high := msin[i] - '0'

		var low byte
		if msin[i+1] == 'f' {
			low = 0x0f
		} else {
			low = msin[i+1] - '0'
		}

		result[i/2] = (low << 4) | high
	}

	return result
}

func TestProfileA_RoundTrip(t *testing.T) {
	// Generate a fresh X25519 key pair for testing.
	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privHex := hex.EncodeToString(privKey.Bytes())
	pubHex := hex.EncodeToString(privKey.PublicKey().Bytes())

	msin := "0000000001"

	schemeOutput, err := buildProfileASchemeOutput(msin, pubHex, privHex)
	if err != nil {
		t.Fatalf("buildProfileASchemeOutput failed: %v", err)
	}

	result, err := profileA(schemeOutput, typeIMSI, privHex)
	if err != nil {
		t.Fatalf("profileA failed: %v", err)
	}

	if result != msin {
		t.Fatalf("expected MSIN %s, got %s", msin, result)
	}
}

func TestProfileA_ShortInput(t *testing.T) {
	_, err := profileA("0102030405060708", typeIMSI, "c53c22208b61860b06c62e5406a7b330c2b577aa5558981510d128247d38bd1d")
	if err == nil {
		t.Fatal("expected error for short input")
	}
}

// ------------- Profile B: low-level round-trip test -------------

// buildProfileBSchemeOutput encrypts an MSIN using Profile B (P-256).
func buildProfileBSchemeOutput(msin string, hnPubKeyCompressedHex string, compressed bool) (string, error) { //nolint:unparam
	hnPubCompressed, err := hex.DecodeString(hnPubKeyCompressedHex)
	if err != nil {
		return "", err
	}

	// Decompress the HN public key to get uncompressed form for ECDH.
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), hnPubCompressed)
	if x == nil {
		return "", fmt.Errorf("bad public key")
	}

	uncompressed := make([]byte, 65)
	uncompressed[0] = 0x04
	x.FillBytes(uncompressed[1:33])
	y.FillBytes(uncompressed[33:65])

	hnPub, err := ecdh.P256().NewPublicKey(uncompressed)
	if err != nil {
		return "", err
	}

	// Generate ephemeral key pair.
	ephPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}

	// Shared key
	sharedKey, err := ephPriv.ECDH(hnPub)
	if err != nil {
		return "", err
	}

	// Get ephemeral public key in the requested format.
	var (
		ephPubBytes []byte
		kdfPubBytes []byte
	)

	ephUncompressed := ephPriv.PublicKey().Bytes() // always 65 bytes (uncompressed)
	ex := new(big.Int).SetBytes(ephUncompressed[1:33])
	ey := new(big.Int).SetBytes(ephUncompressed[33:65])
	ephCompressed := elliptic.MarshalCompressed(elliptic.P256(), ex, ey)

	if compressed {
		ephPubBytes = ephCompressed
		kdfPubBytes = ephCompressed
	} else {
		ephPubBytes = ephUncompressed
		kdfPubBytes = ephCompressed // KDF always uses compressed form
	}

	// KDF
	kdfKey := ansiX963KDF(sharedKey, kdfPubBytes, ProfileBEncKeyLen, ProfileBMacKeyLen, ProfileBHashLen)
	encKey := kdfKey[:ProfileBEncKeyLen]
	icb := kdfKey[ProfileBEncKeyLen : ProfileBEncKeyLen+ProfileBIcbLen]
	macKey := kdfKey[len(kdfKey)-ProfileBMacKeyLen:]

	// Encode MSIN
	plainText := encodeMSIN(msin)

	// AES-128-CTR encrypt
	block, _ := aes.NewCipher(encKey)
	stream := cipher.NewCTR(block, icb)
	cipherText := make([]byte, len(plainText))
	stream.XORKeyStream(cipherText, plainText)

	// HMAC
	h := hmac.New(sha256.New, macKey)
	h.Write(cipherText)
	macTag := h.Sum(nil)[:ProfileBMacLen]

	// schemeOutput = ephPubBytes || cipherText || macTag
	out := append(ephPubBytes, cipherText...)
	out = append(out, macTag...)

	return hex.EncodeToString(out), nil
}

func TestProfileB_Compressed_RoundTrip(t *testing.T) {
	privKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privHex := hex.EncodeToString(privKey.Bytes())

	// Derive compressed public key.
	pubUncompressed := privKey.PublicKey().Bytes()
	x := new(big.Int).SetBytes(pubUncompressed[1:33])
	y := new(big.Int).SetBytes(pubUncompressed[33:65])
	pubCompressedHex := hex.EncodeToString(elliptic.MarshalCompressed(elliptic.P256(), x, y))

	msin := "0000000001"

	schemeOutput, err := buildProfileBSchemeOutput(msin, pubCompressedHex, true)
	if err != nil {
		t.Fatalf("buildProfileBSchemeOutput failed: %v", err)
	}

	result, err := profileB(schemeOutput, typeIMSI, privHex)
	if err != nil {
		t.Fatalf("profileB failed: %v", err)
	}

	if result != msin {
		t.Fatalf("expected MSIN %s, got %s", msin, result)
	}
}

func TestProfileB_Uncompressed_RoundTrip(t *testing.T) {
	privKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privHex := hex.EncodeToString(privKey.Bytes())

	// Derive compressed public key (needed by buildProfileBSchemeOutput).
	pubUncompressed := privKey.PublicKey().Bytes()
	x := new(big.Int).SetBytes(pubUncompressed[1:33])
	y := new(big.Int).SetBytes(pubUncompressed[33:65])
	pubCompressedHex := hex.EncodeToString(elliptic.MarshalCompressed(elliptic.P256(), x, y))

	msin := "0000000001"

	schemeOutput, err := buildProfileBSchemeOutput(msin, pubCompressedHex, false) // uncompressed ephemeral
	if err != nil {
		t.Fatalf("buildProfileBSchemeOutput failed: %v", err)
	}

	result, err := profileB(schemeOutput, typeIMSI, privHex)
	if err != nil {
		t.Fatalf("profileB failed: %v", err)
	}

	if result != msin {
		t.Fatalf("expected MSIN %s, got %s", msin, result)
	}
}

func TestProfileB_ShortInput(t *testing.T) {
	_, err := profileB("020102030405060708", typeIMSI, "f1ab1074477ebcce59b97460c83b4071db578ffab54ee4fbc76aeca38e4b7b01")
	if err == nil {
		t.Fatal("expected error for short input")
	}
}

func TestProfileB_UnknownFormatByte(t *testing.T) {
	// Build a hex string starting with 0x05 (invalid format).
	_, err := profileB("05"+hex.EncodeToString(make([]byte, 80)), typeIMSI, "f1ab1074477ebcce59b97460c83b4071db578ffab54ee4fbc76aeca38e4b7b01")
	if err == nil {
		t.Fatal("expected error for unknown format byte")
	}
}

func TestProfileB_BadMac(t *testing.T) {
	privKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privHex := hex.EncodeToString(privKey.Bytes())

	pubUncompressed := privKey.PublicKey().Bytes()
	x := new(big.Int).SetBytes(pubUncompressed[1:33])
	y := new(big.Int).SetBytes(pubUncompressed[33:65])
	pubCompressedHex := hex.EncodeToString(elliptic.MarshalCompressed(elliptic.P256(), x, y))

	msin := "0000000001"

	schemeOutput, err := buildProfileBSchemeOutput(msin, pubCompressedHex, true)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with the last byte of the scheme output (part of the MAC).
	raw, _ := hex.DecodeString(schemeOutput)
	raw[len(raw)-1] ^= 0xff
	tampered := hex.EncodeToString(raw)

	_, err = profileB(tampered, typeIMSI, privHex)
	if err == nil {
		t.Fatal("expected MAC failure")
	}
}

// ------------- ECDH helpers unit tests -------------

func TestEcdhX25519(t *testing.T) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privHex := hex.EncodeToString(priv.Bytes())
	pubBytes := priv.PublicKey().Bytes()

	// ecdhX25519 with own pub should give same result as raw ECDH.
	shared, err := ecdhX25519(privHex, pubBytes)
	if err != nil {
		t.Fatalf("ecdhX25519 failed: %v", err)
	}

	if len(shared) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(shared))
	}
}

func TestEcdhP256_Compressed(t *testing.T) {
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privHex := hex.EncodeToString(priv.Bytes())

	// Create a compressed public key.
	pubUncompressed := priv.PublicKey().Bytes()
	x := new(big.Int).SetBytes(pubUncompressed[1:33])
	y := new(big.Int).SetBytes(pubUncompressed[33:65])
	compressed := elliptic.MarshalCompressed(elliptic.P256(), x, y)

	shared, kdfPub, err := ecdhP256(privHex, compressed)
	if err != nil {
		t.Fatalf("ecdhP256 failed: %v", err)
	}

	if len(shared) != 32 {
		t.Fatalf("expected 32 bytes shared key, got %d", len(shared))
	}

	if len(kdfPub) != 33 {
		t.Fatalf("expected 33-byte compressed KDF pub, got %d", len(kdfPub))
	}
}

func TestEcdhP256_Uncompressed(t *testing.T) {
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privHex := hex.EncodeToString(priv.Bytes())
	pubUncompressed := priv.PublicKey().Bytes() // 65 bytes

	shared, kdfPub, err := ecdhP256(privHex, pubUncompressed)
	if err != nil {
		t.Fatalf("ecdhP256 failed: %v", err)
	}

	if len(shared) != 32 {
		t.Fatalf("expected 32 bytes shared key, got %d", len(shared))
	}
	// kdfPub should be compressed (33 bytes).
	if len(kdfPub) != 33 {
		t.Fatalf("expected 33-byte compressed KDF pub, got %d", len(kdfPub))
	}
}

// ------------- ToSupi integration tests -------------

func TestToSupi_NullScheme(t *testing.T) {
	suci := "suci-0-001-01-0000-0-0-0000000001"
	resolverCalled := false

	supi, err := ToSupi(suci, func(scheme string, keyID int) (string, error) {
		resolverCalled = true
		return "", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolverCalled {
		t.Fatal("resolver should not be called for null scheme")
	}

	if supi.IMSI() != "001010000000001" {
		t.Fatalf("expected IMSI 001010000000001, got %s", supi.IMSI())
	}
}

func TestToSupi_ImsiPrefix(t *testing.T) {
	supi, err := ToSupi("imsi-001010000000001", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if supi.IMSI() != "001010000000001" {
		t.Fatalf("expected IMSI 001010000000001, got %s", supi.IMSI())
	}
}

func TestToSupi_MalformedSuci(t *testing.T) {
	_, err := ToSupi("suci-0-001-01", nil)
	if err == nil {
		t.Fatal("expected error for malformed SUCI")
	}
}

func TestToSupi_UnsupportedScheme(t *testing.T) {
	suci := "suci-0-001-01-0000-3-0-0000000001"

	_, err := ToSupi(suci, func(scheme string, keyID int) (string, error) {
		return "deadbeef", nil
	})
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

func TestToSupi_KeyNotFound(t *testing.T) {
	suci := "suci-0-001-01-0000-1-0-0000000001"

	_, err := ToSupi(suci, func(scheme string, keyID int) (string, error) {
		return "", fmt.Errorf("key not found")
	})
	if err == nil {
		t.Fatal("expected error when resolver fails")
	}
}

func TestToSupi_NonZeroKeyId(t *testing.T) {
	// Build a valid Profile A SUCI with keyId=1.
	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privHex := hex.EncodeToString(privKey.Bytes())
	pubHex := hex.EncodeToString(privKey.PublicKey().Bytes())

	msin := "0000000001"

	schemeOutput, err := buildProfileASchemeOutput(msin, pubHex, privHex)
	if err != nil {
		t.Fatal(err)
	}

	suci := fmt.Sprintf("suci-0-001-01-0000-1-1-%s", schemeOutput)

	var receivedKeyID int

	supi, err := ToSupi(suci, func(scheme string, keyID int) (string, error) {
		receivedKeyID = keyID
		return privHex, nil
	})
	if err != nil {
		t.Fatalf("ToSupi failed: %v", err)
	}

	if receivedKeyID != 1 {
		t.Fatalf("expected keyID 1, got %d", receivedKeyID)
	}

	if supi.IMSI() != "001010000000001" {
		t.Fatalf("expected IMSI 001010000000001, got %s", supi.IMSI())
	}
}

func TestToSupi_ProfileA_FullRoundTrip(t *testing.T) {
	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privHex := hex.EncodeToString(privKey.Bytes())
	pubHex := hex.EncodeToString(privKey.PublicKey().Bytes())

	msin := "0000000001"

	schemeOutput, err := buildProfileASchemeOutput(msin, pubHex, privHex)
	if err != nil {
		t.Fatal(err)
	}

	suci := fmt.Sprintf("suci-0-001-01-0000-1-0-%s", schemeOutput)

	supi, err := ToSupi(suci, func(scheme string, keyID int) (string, error) {
		if scheme != "A" {
			t.Fatalf("expected scheme A, got %s", scheme)
		}

		return privHex, nil
	})
	if err != nil {
		t.Fatalf("ToSupi failed: %v", err)
	}

	if supi.IMSI() != "001010000000001" {
		t.Fatalf("expected IMSI 001010000000001, got %s", supi.IMSI())
	}
}

func TestToSupi_ProfileB_FullRoundTrip(t *testing.T) {
	privKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privHex := hex.EncodeToString(privKey.Bytes())

	pubUncompressed := privKey.PublicKey().Bytes()
	x := new(big.Int).SetBytes(pubUncompressed[1:33])
	y := new(big.Int).SetBytes(pubUncompressed[33:65])
	pubCompressedHex := hex.EncodeToString(elliptic.MarshalCompressed(elliptic.P256(), x, y))

	msin := "0000000001"

	schemeOutput, err := buildProfileBSchemeOutput(msin, pubCompressedHex, true)
	if err != nil {
		t.Fatal(err)
	}

	suci := fmt.Sprintf("suci-0-001-01-0000-2-0-%s", schemeOutput)

	supi, err := ToSupi(suci, func(scheme string, keyID int) (string, error) {
		if scheme != "B" {
			t.Fatalf("expected scheme B, got %s", scheme)
		}

		return privHex, nil
	})
	if err != nil {
		t.Fatalf("ToSupi failed: %v", err)
	}

	if supi.IMSI() != "001010000000001" {
		t.Fatalf("expected IMSI 001010000000001, got %s", supi.IMSI())
	}
}

// ------------- KDF and MAC helpers test -------------

func TestAnsiX963KDF(t *testing.T) {
	sharedKey := make([]byte, 32)
	pubKey := make([]byte, 32)
	result := ansiX963KDF(sharedKey, pubKey, 16, 32, 32)
	expectedLen := 16 + 32 // encKeyLen + macKeyLen, but actually ceil rounds
	kdfRounds := int(math.Ceil(float64(16+32) / float64(32)))

	actualLen := kdfRounds * 32
	if len(result) != actualLen {
		t.Fatalf("expected KDF output length %d, got %d", expectedLen, len(result))
	}
}

func TestDecryptWithKdf_BadMac(t *testing.T) {
	sharedKey := make([]byte, 32)
	kdfPubKey := make([]byte, 32)
	cipherText := []byte{0x01, 0x02, 0x03}
	badMac := make([]byte, 8) // zeros won't match

	_, err := decryptWithKdf(sharedKey, kdfPubKey, cipherText, badMac,
		ProfileAEncKeyLen, ProfileAMacKeyLen, ProfileAHashLen, ProfileAIcbLen, ProfileAMacLen)
	if err == nil {
		t.Fatal("expected MAC failure")
	}
}

func TestDecryptWithKdf_ValidMac(t *testing.T) {
	sharedKey := make([]byte, 32)
	kdfPubKey := make([]byte, 32)
	cipherText := []byte{0x01, 0x02, 0x03}

	// Compute the correct MAC manually.
	kdfKey := ansiX963KDF(sharedKey, kdfPubKey, ProfileAEncKeyLen, ProfileAMacKeyLen, ProfileAHashLen)
	macKey := kdfKey[len(kdfKey)-ProfileAMacKeyLen:]
	h := hmac.New(sha256.New, macKey)
	h.Write(cipherText)
	correctMac := h.Sum(nil)[:ProfileAMacLen]

	result, err := decryptWithKdf(sharedKey, kdfPubKey, cipherText, correctMac,
		ProfileAEncKeyLen, ProfileAMacKeyLen, ProfileAHashLen, ProfileAIcbLen, ProfileAMacLen)
	if err != nil {
		t.Fatalf("decryptWithKdf failed: %v", err)
	}

	if len(result) != len(cipherText) {
		t.Fatalf("expected %d bytes output, got %d", len(cipherText), len(result))
	}
}

// ------------- ansiX963KDF used in helper but declared here for KDF-only test above -------------
// (The production function is in suci.go; tests are in this same package.)

func TestSwapNibbles(t *testing.T) {
	input := []byte{0x12, 0x34}
	result := swapNibbles(input)

	expected := []byte{0x21, 0x43}
	if result[0] != expected[0] || result[1] != expected[1] {
		t.Fatalf("expected %x, got %x", expected, result)
	}
}

func TestCalcSchemeResult_IMSI(t *testing.T) {
	// Input: BCD-swapped 0000000001
	plain := encodeMSIN("0000000001")

	result := calcSchemeResult(plain, typeIMSI)
	if result != "0000000001" {
		t.Fatalf("expected 0000000001, got %s", result)
	}
}

func TestCalcSchemeResult_OddLength(t *testing.T) {
	// Input: odd-length MSIN (5 digits → 3 bytes BCD with trailing f)
	plain := encodeMSIN("12345")

	result := calcSchemeResult(plain, typeIMSI)
	if result != "12345" {
		t.Fatalf("expected 12345, got %s", result)
	}
}

func TestHmacSha256(t *testing.T) {
	// Just verify it returns the correct truncated length.
	key := make([]byte, 32)
	input := []byte("test data")

	result, err := hmacSha256(input, key, 8)
	if err != nil {
		t.Fatalf("hmacSha256 failed: %v", err)
	}

	if len(result) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(result))
	}
}

func TestAes128ctr(t *testing.T) {
	key := make([]byte, 16)
	icb := make([]byte, 16)
	plainText := []byte("hello world 1234") // 16 bytes

	encrypted, err := aes128ctr(plainText, key, icb)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// Decrypt back.
	decrypted, err := aes128ctr(encrypted, key, icb)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if string(decrypted) != string(plainText) {
		t.Fatalf("round trip failed: got %q", decrypted)
	}
}

// ------------- unused but ensure we have binary import for kdf test -----------
var _ = binary.BigEndian

// ------------- Known-good test vectors from free5gc/udm (Apache-2.0) -------------
// These validate our implementation against independently-computed test data.
// Source: https://github.com/free5gc/udm/blob/main/pkg/suci/suci_test.go

// TestFree5GCVectors_ProfileDecryption validates our profile-level decryption
// against known-good test vectors from the free5gc/udm project.
// This exercises the core cryptographic logic (ECDH, KDF, AES-CTR, HMAC)
// independently of the SUPI construction layer.
func TestFree5GCVectors_ProfileDecryption(t *testing.T) {
	const (
		profileAKey = "c53c22208b61860b06c62e5406a7b330c2b577aa5558981510d128247d38bd1d"
		profileBKey = "F1AB1074477EBCC7F554EA1C5FC368B1616730155E0041AC447D6301975FECDA"
	)

	tests := []struct {
		name         string
		profile      string // "A" or "B"
		schemeOutput string
		privateKey   string
		wantMSIN     string // expected decrypted MSIN (empty if error expected)
		wantErr      bool
	}{
		{
			name:    "Profile A",
			profile: "A",
			schemeOutput: "b2e92f836055a255837debf850b528997ce0201cb82a" +
				"dfe4be1f587d07d8457dcb02352410cddd9e730ef3fa87",
			privateKey: profileAKey,
			wantMSIN:   "001002086",
		},
		{
			name:    "Profile B, compressed eph key",
			profile: "B",
			schemeOutput: "039aab8376597021e855679a9778ea0b67396e68c66d" +
				"f32c0f41e9acca2da9b9d146a33fc2716ac7dae96aa30a4d",
			privateKey: profileBKey,
			wantMSIN:   "001002086",
		},
		{
			name:    "Profile B, bad uncompressed eph key",
			profile: "B",
			schemeOutput: "0434a66778799d52fedd9326db4b690d092e05c9ba0ace5b413da" +
				"fc0a40aa28ee00a79f790fa4da6a2ece892423adb130dc1b" +
				"30e270b7d0088bdd716b93894891d5221a74c810d6b9350cc067c76",
			privateKey: profileBKey,
			wantErr:    true,
		},
		{
			name:    "Profile B, compressed eph key, different MSIN",
			profile: "B",
			schemeOutput: "03a7b1db2a9db9d44112b59d03d8243dc6089fd91d2ecb" +
				"78f5d16298634682e94373888b22bdc9293d1681922e17",
			privateKey: profileBKey,
			wantMSIN:   "0123456789",
		},
		{
			name:    "Profile B, uncompressed eph key",
			profile: "B",
			schemeOutput: "049AAB8376597021E855679A9778EA0B67396E68C66DF32C0F41E9ACCA2D" +
				"A9B9D1D1F44EA1C87AA7478B954537BDE79951E748A43294A4F4CF86EAFF" +
				"1789C9C81F46A33FC2716AC7DAE96AA30A4D",
			privateKey: profileBKey,
			wantMSIN:   "001002086",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				result string
				err    error
			)

			switch tt.profile {
			case "A":
				result, err = profileA(tt.schemeOutput, typeIMSI, tt.privateKey)
			case "B":
				result, err = profileB(tt.schemeOutput, typeIMSI, tt.privateKey)
			}

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.wantMSIN {
				t.Fatalf("MSIN = %s, want %s", result, tt.wantMSIN)
			}
		})
	}
}

// TestToSupi_Free5GCVectors validates the full ToSupi pipeline (key resolution,
// scheme dispatch, SUPI construction) using vectors that produce valid 15-digit IMSIs.
func TestToSupi_Free5GCVectors(t *testing.T) {
	const profileBKey = "F1AB1074477EBCC7F554EA1C5FC368B1616730155E0041AC447D6301975FECDA"

	keys := map[string]string{
		"B-2": profileBKey,
		"B-3": profileBKey,
	}

	resolver := func(scheme string, keyID int) (string, error) {
		if priv, ok := keys[fmt.Sprintf("%s-%d", scheme, keyID)]; ok {
			return priv, nil
		}

		return "", fmt.Errorf("key not found: %s-%d", scheme, keyID)
	}

	tests := []struct {
		name    string
		suci    string
		want    string // expected IMSI
		wantErr bool
	}{
		{
			name: "Profile B, compressed eph, keyId=2",
			suci: "suci-0-001-01-0-2-2-" +
				"03a7b1db2a9db9d44112b59d03d8243dc6089fd91d2ecb" +
				"78f5d16298634682e94373888b22bdc9293d1681922e17",
			want: "001010123456789",
		},
		{
			name: "Profile B, compressed eph, keyId=3",
			suci: "suci-0-001-01-0-2-3-" +
				"03a7b1db2a9db9d44112b59d03d8243dc6089fd91d2ecb" +
				"78f5d16298634682e94373888b22bdc9293d1681922e17",
			want: "001010123456789",
		},
		{
			name: "Profile B, bad uncompressed eph, keyId=2",
			suci: "suci-0-208-93-0-2-2-" +
				"0434a66778799d52fedd9326db4b690d092e05c9ba0ace5b413da" +
				"fc0a40aa28ee00a79f790fa4da6a2ece892423adb130dc1b" +
				"30e270b7d0088bdd716b93894891d5221a74c810d6b9350cc067c76",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			supi, err := ToSupi(tt.suci, resolver)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if supi.IMSI() != tt.want {
				t.Fatalf("IMSI = %s, want %s", supi.IMSI(), tt.want)
			}
		})
	}
}
