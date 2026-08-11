// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func kdf(t *testing.T, key []byte, fc byte, params ...[]byte) []byte {
	t.Helper()

	s := []byte{fc}

	for _, p := range params {
		s = append(s, p...)
		s = append(s, byte(len(p)>>8), byte(len(p)))
	}

	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write(s); err != nil {
		t.Fatalf("KDF write: %v", err)
	}

	return mac.Sum(nil)
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}

	return b
}

const (
	testKAMF  = "6c38fea1e0a2ff9f8ba6a1e4f4de8b8e1b3b7f2e9d5c0a4738261f5e0d9c8b7a"
	testKASME = "1f2e3d4c5b6a798807162534435261708f9eadbccbdae9f80716253443526170"
)

func u32(v uint32) []byte { return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)} }

func TestMapToEPSDerivesKASMEFromTheDownlinkCount(t *testing.T) {
	kamf := mustHex(t, testKAMF)
	dl := nas.MakeCount(0x0102, 0x03)

	got, err := interworking.MapToEPSOnHandover(interworking.FiveGToEPSInput{
		KAMF:       kamf,
		NgKSI:      nas.KeySetIdentifier{Value: 5},
		ULNASCount: nas.MakeCount(0, 9),
		DLNASCount: dl,
		Algorithms: interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES},
	})
	if err != nil {
		t.Fatalf("MapToEPSOnHandover: %v", err)
	}

	want := kdf(t, kamf, 0x74, u32(dl.Value()))
	if !bytes.Equal(got.Context.KASME[:], want) {
		t.Fatalf("K'ASME = %x, want %x", got.Context.KASME, want)
	}

	if got.Context.EKSI != (nas.KeySetIdentifier{Value: 5, Mapped: true}) {
		t.Fatalf("eKSI = %+v, want value 5 of a mapped context", got.Context.EKSI)
	}

	// The EPS NAS COUNTs are the 5G ones, not a reset pair (§8.6.1).
	if got.Context.ULNASCount != nas.MakeCount(0, 9) || got.Context.DLNASCount != dl {
		t.Fatalf("EPS NAS COUNTs = (%d, %d), want the 5G context's", got.Context.ULNASCount, got.Context.DLNASCount)
	}
}

// TS 33.501 §8.3.2
func TestMapToEPSContainerNamesTheDerivationCount(t *testing.T) {
	dl := nas.MakeCount(0x00ff, 0xfe)

	got, err := interworking.MapToEPSOnHandover(interworking.FiveGToEPSInput{
		KAMF:       mustHex(t, testKAMF),
		DLNASCount: dl,
	})
	if err != nil {
		t.Fatalf("MapToEPSOnHandover: %v", err)
	}

	if got.Container.SequenceNumber != dl.SQN() {
		t.Fatalf("container sequence number = %#x, want the %#x the derivation used", got.Container.SequenceNumber, dl.SQN())
	}
}

// TS 33.501 §8.3.2 step 2, TS 33.401 A.3, A.4
func TestMapToEPSDerivesTheNHChainTwice(t *testing.T) {
	kamf := mustHex(t, testKAMF)
	dl := nas.MakeCount(1, 1)

	got, err := interworking.MapToEPSOnHandover(interworking.FiveGToEPSInput{KAMF: kamf, DLNASCount: dl})
	if err != nil {
		t.Fatalf("MapToEPSOnHandover: %v", err)
	}

	kasme := kdf(t, kamf, 0x74, u32(dl.Value()))
	kenb := kdf(t, kasme, 0x11, u32(0xFFFFFFFF))
	nh1 := kdf(t, kasme, 0x12, kenb)
	want := kdf(t, kasme, 0x12, nh1)

	if !bytes.Equal(got.Context.NH[:], want) {
		t.Fatalf("NH = %x, want the second hop %x", got.Context.NH, want)
	}

	if got.Context.NCC != 2 {
		t.Fatalf("NCC = %d, want 2", got.Context.NCC)
	}

	// The first hop must not be what is sent: a UE deriving twice would not match.
	if bytes.Equal(got.Context.NH[:], nh1) {
		t.Fatal("NH is the first hop, so the chain was walked only once")
	}
}

// TS 33.501 §8.3.2 NOTE 3
func TestMapToEPSInitialKeNBUsesTheOutOfRangeCount(t *testing.T) {
	kamf := mustHex(t, testKAMF)
	dl := nas.MakeCount(0, 0)

	got, err := interworking.MapToEPSOnHandover(interworking.FiveGToEPSInput{KAMF: kamf, DLNASCount: dl})
	if err != nil {
		t.Fatalf("MapToEPSOnHandover: %v", err)
	}

	kasme := kdf(t, kamf, 0x74, u32(dl.Value()))

	// The same chain seeded from the 24-bit truncation of 2³²−1 must differ.
	truncated := kdf(t, kasme, 0x11, u32(0x00FFFFFF))
	nh1 := kdf(t, kasme, 0x12, truncated)

	if bytes.Equal(got.Context.NH[:], kdf(t, kasme, 0x12, nh1)) {
		t.Fatal("the initial K_eNB was derived from a truncated COUNT")
	}
}

func TestMapToEPSRejectsAShortKAMF(t *testing.T) {
	if _, err := interworking.MapToEPSOnHandover(interworking.FiveGToEPSInput{KAMF: make([]byte, 16)}); err == nil {
		t.Fatal("a 128-bit K_AMF must be refused")
	}
}

func TestMapToEPSCarriesTheEPSParameters(t *testing.T) {
	algorithms := interworking.EPSNASAlgorithms{Ciphering: nas.CipheringSNOW3G, Integrity: nas.IntegrityAES}
	capability := eps.UESecurityCapability{EEA: 0xe0, EIA: 0x60}

	got, err := interworking.MapToEPSOnHandover(interworking.FiveGToEPSInput{
		KAMF:                 mustHex(t, testKAMF),
		Algorithms:           algorithms,
		UESecurityCapability: capability,
	})
	if err != nil {
		t.Fatalf("MapToEPSOnHandover: %v", err)
	}

	if got.Context.Algorithms != algorithms {
		t.Fatalf("EPS NAS algorithms = %+v, want %+v", got.Context.Algorithms, algorithms)
	}

	if got.Context.UESecurityCapability != capability {
		t.Fatalf("UE security capability = %+v, want %+v", got.Context.UESecurityCapability, capability)
	}
}

// aesOnly is an operator policy of 128-NEA2 / 128-NIA2.
var (
	aesInt = []nas.IntegrityAlgorithm{nas.IntegrityAES}
	aesEnc = []nas.CipheringAlgorithm{nas.CipheringAES}
)

func epsContext(t *testing.T) interworking.EPSSecurityContext {
	t.Helper()

	in := interworking.EPSSecurityContext{
		EKSI:       nas.KeySetIdentifier{Value: 3},
		ULNASCount: nas.MakeCount(0, 7),
		DLNASCount: nas.MakeCount(0, 4),
		Algorithms: interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES},
		NCC:        3,
	}
	copy(in.KASME[:], mustHex(t, testKASME))

	for i := range in.NH {
		in.NH[i] = byte(i + 1)
	}

	return in
}

// K'AMF is KDF(K_ASME, FC=0x76, P0 = the NH of the EPS context) — the NH, not the
// EPS NAS COUNTs (TS 33.501 Annex A.15.2, §8.6.2).
func TestMapTo5GSDerivesKAMFFromTheNH(t *testing.T) {
	in := epsContext(t)

	got, err := interworking.MapTo5GSOnHandover(in, aesInt, aesEnc)
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	want := kdf(t, in.KASME[:], 0x76, in.NH[:])
	if !bytes.Equal(got.Context.KAMF[:], want) {
		t.Fatalf("K'AMF = %x, want %x", got.Context.KAMF, want)
	}

	if got.Context.NgKSI != (nas.KeySetIdentifier{Value: 3, Mapped: true}) {
		t.Fatalf("ngKSI = %+v, want value 3 of a mapped context", got.Context.NgKSI)
	}
}

// The 5G NAS COUNTs start at zero, and the downlink one is already past zero
// because building the container consumed it (TS 33.501 §8.6.2, §8.4.2 step 3).
func TestMapTo5GSResetsTheNASCounts(t *testing.T) {
	got, err := interworking.MapTo5GSOnHandover(epsContext(t), aesInt, aesEnc)
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	if got.Context.ULNASCount != 0 {
		t.Fatalf("uplink 5G NAS COUNT = %d, want 0", got.Context.ULNASCount)
	}

	if got.Context.DLNASCount != 1 {
		t.Fatalf("downlink 5G NAS COUNT = %d, want 1 after the NAS container", got.Context.DLNASCount)
	}
}

// The gNB is handed {NCC=0, NH=temporary K_gNB} derived from K'AMF at 2³²−1, and
// the AMF keeps the chain advanced once to {NCC=1, NH} (§8.4.2 steps 3 and 4).
func TestMapTo5GSDerivesTheASKeyChain(t *testing.T) {
	in := epsContext(t)

	got, err := interworking.MapTo5GSOnHandover(in, aesInt, aesEnc)
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	kamf := kdf(t, in.KASME[:], 0x76, in.NH[:])
	wantKgNB := kdf(t, kamf, 0x6E, u32(0xFFFFFFFF), []byte{0x01})

	if !bytes.Equal(got.Context.TemporaryKgNB[:], wantKgNB) {
		t.Fatalf("temporary K_gNB = %x, want %x", got.Context.TemporaryKgNB, wantKgNB)
	}

	if want := kdf(t, kamf, 0x6F, wantKgNB); !bytes.Equal(got.Context.NH[:], want) {
		t.Fatalf("stored NH = %x, want %x", got.Context.NH, want)
	}

	if got.Context.NCC != 1 {
		t.Fatalf("stored NCC = %d, want 1", got.Context.NCC)
	}

	if bytes.Equal(got.Context.TemporaryKgNB[:], in.NH[:]) {
		t.Fatal("the EPS NH was handed to the gNB instead of a K_gNB derived from K'AMF")
	}
}

// TS 33.401 §7.2.8.4.4
func TestMapTo5GSContainerCarriesTheEPSNCC(t *testing.T) {
	in := epsContext(t)

	got, err := interworking.MapTo5GSOnHandover(in, aesInt, aesEnc)
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	if got.Container.NCC != in.NCC {
		t.Fatalf("container NCC = %d, want the EPS NCC %d", got.Container.NCC, in.NCC)
	}

	if got.Container.NCC == got.Context.NCC {
		t.Fatal("the container carries the AMF's stored NCC rather than the EPS one")
	}

	if got.Container.NgKSI != got.Context.NgKSI ||
		got.Container.CipheringAlgorithm != got.Context.Ciphering ||
		got.Container.IntegrityAlgorithm != got.Context.Integrity {
		t.Fatal("the container must name the mapped context it announces")
	}
}

// TS 33.501 §6.9.2.3.3
func TestMapTo5GSContainerMAC(t *testing.T) {
	got, err := interworking.MapTo5GSOnHandover(epsContext(t), aesInt, aesEnc)
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:    got.Context.Integrity,
		Ciphering:    nas.CipheringNull,
		IntegrityKey: got.Context.KNASInt,
	})
	if err != nil {
		t.Fatalf("NewSecurityContext: %v", err)
	}

	protected, err := got.Container.MACProtected()
	if err != nil {
		t.Fatalf("MACProtected: %v", err)
	}

	if err := sc.VerifyMACAtMaxCount(protected, got.Container.MessageAuthenticationCode, nas.Bearer3GPP, nas.DirectionDownlink); err != nil {
		t.Fatalf("the container does not verify against the mapped context: %v", err)
	}

	if err := sc.VerifyMAC(protected, got.Container.MessageAuthenticationCode, 0, nas.Bearer3GPP, nas.DirectionDownlink); err == nil {
		t.Fatal("the container MAC was computed at COUNT 0")
	}

	if raw, err := got.Container.MarshalBinary(); err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	} else if !bytes.Equal(raw[4:], protected) {
		t.Fatal("the encoded container does not carry the octets the MAC covers")
	}
}

// TS 33.501
func TestMapTo5GSDefaultCapability(t *testing.T) {
	got, err := interworking.MapTo5GSOnHandover(epsContext(t), aesInt, aesEnc)
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	if !got.Context.UESecurityCapability.Equal(interworking.DefaultUE5GSecurityCapability) {
		t.Fatalf("UE security capability = %s, want the default set", got.Context.UESecurityCapability)
	}

	for _, alg := range []uint8{uint8(nas.CipheringNull), uint8(nas.CipheringSNOW3G), uint8(nas.CipheringAES)} {
		if !got.Context.UESecurityCapability.SupportsEA(alg) {
			t.Errorf("the default set must include NEA%d", alg)
		}
	}

	if got.Context.UESecurityCapability.SupportsIA(uint8(nas.IntegrityNull)) {
		t.Error("the default set must not include NIA0")
	}

	if !got.Context.UESecurityCapability.SupportsIA(uint8(nas.IntegritySNOW3G)) ||
		!got.Context.UESecurityCapability.SupportsIA(uint8(nas.IntegrityAES)) {
		t.Error("the default set must include 128-NIA1 and 128-NIA2")
	}
}

func TestMapTo5GSUsesTheForwardedCapability(t *testing.T) {
	in := epsContext(t)

	in.UE5GSecurityCapability = &fgs.UESecurityCapability{EA: 0x40, IA: 0x40}

	got, err := interworking.MapTo5GSOnHandover(in,
		[]nas.IntegrityAlgorithm{nas.IntegrityAES, nas.IntegritySNOW3G},
		[]nas.CipheringAlgorithm{nas.CipheringAES, nas.CipheringSNOW3G})
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	if got.Context.Ciphering != nas.CipheringSNOW3G || got.Context.Integrity != nas.IntegritySNOW3G {
		t.Fatalf("selected (%s, %s), want the SNOW 3G pair the UE supports in N1 mode",
			got.Context.Ciphering, got.Context.Integrity)
	}

	if got.Context.EPSAlgorithms != in.Algorithms {
		t.Fatalf("stored EPS algorithms = %+v, want the MME's %+v", got.Context.EPSAlgorithms, in.Algorithms)
	}
}

func TestMapTo5GSRejectsWhenNoAlgorithmIsCommon(t *testing.T) {
	in := epsContext(t)
	in.UE5GSecurityCapability = &fgs.UESecurityCapability{EA: 0x40, IA: 0x40} // SNOW 3G only

	if _, err := interworking.MapTo5GSOnHandover(in, aesInt, aesEnc); err == nil {
		t.Fatal("a UE sharing no 5G NAS algorithm with the operator must be refused")
	}
}

// TS 33.501 A.8
func TestMapTo5GSDerivesNASKeys(t *testing.T) {
	in := epsContext(t)

	got, err := interworking.MapTo5GSOnHandover(in, aesInt, aesEnc)
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	kamf := kdf(t, in.KASME[:], 0x76, in.NH[:])

	wantEnc := kdf(t, kamf, 0x69, []byte{0x01}, []byte{uint8(nas.CipheringAES)})
	if !bytes.Equal(got.Context.KNASEnc[:], wantEnc[16:32]) {
		t.Fatalf("K_NASenc = %x, want %x", got.Context.KNASEnc, wantEnc[16:32])
	}

	wantInt := kdf(t, kamf, 0x69, []byte{0x02}, []byte{uint8(nas.IntegrityAES)})
	if !bytes.Equal(got.Context.KNASInt[:], wantInt[16:32]) {
		t.Fatalf("K_NASint = %x, want %x", got.Context.KNASInt, wantInt[16:32])
	}
}

func TestMapTo5GSFollowsTheOperatorIntegrityPolicy(t *testing.T) {
	in := epsContext(t)
	in.UE5GSecurityCapability = &fgs.UESecurityCapability{EA: 0x80, IA: 0x80} // NEA0/NIA0 only

	got, err := interworking.MapTo5GSOnHandover(in,
		[]nas.IntegrityAlgorithm{nas.IntegrityNull},
		[]nas.CipheringAlgorithm{nas.CipheringNull})
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	if got.Context.Integrity != nas.IntegrityNull {
		t.Fatalf("integrity algorithm = %s, want the configured NIA0", got.Context.Integrity)
	}

	if got.Container.MessageAuthenticationCode != [4]byte{} {
		t.Fatal("NIA0 produces an all-zero message authentication code")
	}
}
