// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package interworking maps a UE's security context between EPS and 5GS for
// N26-based inter-system mobility (TS 33.501 §8).

package interworking

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/epskeys"
	"github.com/ellanetworks/core/internal/fivegskeys"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// TS 33.501 Annex A.14, A.15
const (
	fcKASMEPrimeHandover = "74" // A.14.2: K_AMF → K'ASME, P0 = downlink 5G NAS COUNT
	fcKAMFPrimeHandover  = "76" // A.15.2: K_ASME → K'AMF, P0 = NH
)

const keyLen = 32

type EPSNASAlgorithms struct {
	Ciphering nas.CipheringAlgorithm // EEA
	Integrity nas.IntegrityAlgorithm // EIA
}

type MappedEPSSecurityContext struct {
	KASME                [keyLen]byte
	EKSI                 nas.KeySetIdentifier
	ULNASCount           nas.Count
	DLNASCount           nas.Count
	Algorithms           EPSNASAlgorithms
	UESecurityCapability eps.UESecurityCapability
	NH                   [keyLen]byte
	NCC                  uint8
}

type FiveGToEPSHandover struct {
	Context   MappedEPSSecurityContext
	Container fgs.N1ModeToS1ModeNASTransparentContainer
}

// FiveGToEPSInput is the 5G security context a handover to EPS maps from.
type FiveGToEPSInput struct {
	KAMF                 []byte
	NgKSI                nas.KeySetIdentifier
	ULNASCount           nas.Count
	DLNASCount           nas.Count
	Algorithms           EPSNASAlgorithms
	UESecurityCapability eps.UESecurityCapability
}

// MapToEPSOnHandover derives the mapped EPS security context for a 5GS to EPS
// handover (TS 33.501 §8.3.2 step 2, §8.6.1, and TS 33.401 Annex A.3, A.4).
func MapToEPSOnHandover(in FiveGToEPSInput) (FiveGToEPSHandover, error) {
	if len(in.KAMF) != keyLen {
		return FiveGToEPSHandover{}, fmt.Errorf("interworking: K_AMF is %d octets, want %d", len(in.KAMF), keyLen)
	}

	// A.14.2: P0 is the downlink 5G NAS COUNT as a 32-bit value.
	kasme, err := deriveKey(in.KAMF, fcKASMEPrimeHandover, count32(in.DLNASCount.Value()))
	if err != nil {
		return FiveGToEPSHandover{}, fmt.Errorf("derive K'ASME: %w", err)
	}

	// TS 33.401 A.3 with the uplink NAS COUNT parameter set to 2³²−1. The value is
	// a placeholder for a COUNT, never a COUNT: neither side's counters move.
	kenb, err := epskeys.DeriveKeNB(kasme[:], nas.CountOutOfRange)
	if err != nil {
		return FiveGToEPSHandover{}, fmt.Errorf("derive initial K_eNB: %w", err)
	}

	// TS 33.401 A.4 twice: the first NH chains off the initial K_eNB, the second
	// off the first, giving the {NH, NCC=2} pair the MME forwards unchanged. The
	// intra-LTE handover path, which advances the chain by one from the stored
	// pair, is deliberately not used here — the UE derives these two the same way.
	nh1, err := epskeys.DeriveNH(kasme[:], kenb[:])
	if err != nil {
		return FiveGToEPSHandover{}, fmt.Errorf("derive NH: %w", err)
	}

	nh2, err := epskeys.DeriveNH(kasme[:], nh1[:])
	if err != nil {
		return FiveGToEPSHandover{}, fmt.Errorf("derive NH: %w", err)
	}

	return FiveGToEPSHandover{
		Context: MappedEPSSecurityContext{
			KASME:                kasme,
			EKSI:                 mappedKSI(in.NgKSI),
			ULNASCount:           in.ULNASCount,
			DLNASCount:           in.DLNASCount,
			Algorithms:           in.Algorithms,
			UESecurityCapability: in.UESecurityCapability,
			NH:                   nh2,
			NCC:                  2,
		},
		Container: fgs.NewN1ModeToS1ModeNASTransparentContainer(in.DLNASCount),
	}, nil
}

// EPSSecurityContext is the EPS security context the MME sends the AMF in the
// Forward Relocation Request (TS 33.501 §8.4.2 step 2). It is the MME's own
// context, not a mapped one, and the AMF reads it without ever sending a 5G
// parameter back the other way.
type EPSSecurityContext struct {
	KASME [keyLen]byte
	EKSI  nas.KeySetIdentifier

	ULNASCount nas.Count
	DLNASCount nas.Count

	// Algorithms are the EPS NAS algorithms in use, which the AMF stores in the
	// mapped 5G context rather than acting on (§8.4.2 step 3).
	Algorithms EPSNASAlgorithms

	// UESecurityCapability is the UE's EPS security capability.
	UESecurityCapability eps.UESecurityCapability

	// NH and NCC are the current EPS AS key chain. NH seeds K'AMF, and NCC is what
	// the UE is told so it can walk its own EPS chain to the same point
	// (TS 33.401 §7.2.8.4.4).
	NH  [keyLen]byte
	NCC uint8

	// UE5GSecurityCapability is the UE's 5G security capability where the MME
	// stored one. Nil means it did not, and DefaultUE5GSecurityCapability applies.
	UE5GSecurityCapability *fgs.UESecurityCapability
}

// Mapped5GSecurityContext is the 5G security context the AMF derives from an EPS
// one on a handover from EPS (TS 33.501 §8.4.2 step 3, §8.6.2).
type Mapped5GSecurityContext struct {
	// KAMF is K'AMF, taken as the K_AMF of the new context.
	KAMF [keyLen]byte
	// NgKSI names it: the value field of the eKSI it was mapped from, with the
	// type field set to indicate a mapped security context.
	NgKSI nas.KeySetIdentifier

	// The 5G NAS COUNTs. Both start at zero; the downlink one is already
	// incremented here, because building the NAS container consumes it (§8.4.2
	// step 3, TS 24.501 §4.4.3.1).
	ULNASCount nas.Count
	DLNASCount nas.Count

	// The selected 5G NAS algorithms and the keys derived for them.
	Ciphering nas.CipheringAlgorithm
	Integrity nas.IntegrityAlgorithm
	KNASEnc   nas.CipherKey
	KNASInt   nas.IntegrityKey

	// UESecurityCapability is what the AMF holds for the UE from here on: either
	// the capability the MME forwarded or DefaultUE5GSecurityCapability.
	UESecurityCapability fgs.UESecurityCapability

	// EPSAlgorithms are the EPS NAS algorithms received from the MME, stored so a
	// later move back to EPS names the pair the UE already holds (§8.4.2 step 3).
	EPSAlgorithms EPSNASAlgorithms

	// TemporaryKgNB is sent to the target gNB as the NH of a {NCC=0, NH} pair,
	// with the New Security Context Indicator. It is derived from K'AMF rather
	// than taken from the EPS chain, so the gNB gets a key the EPS side never had
	// (§8.4.2 step 4 and NOTE 3).
	TemporaryKgNB [keyLen]byte

	// NH and NCC are what the AMF stores after handing the gNB the pair above:
	// the chain advanced once, to {NCC=1, NH} (§8.4.2 step 4).
	NH  [keyLen]byte
	NCC uint8
}

// EPSTo5GSHandover is everything the AMF produces when it maps an EPS security
// context for a handover to 5GS.
type EPSTo5GSHandover struct {
	// Context is the mapped 5G NAS security context the AMF takes into use.
	Context Mapped5GSecurityContext

	// Container goes to the target gNB in the Handover Request and reaches the UE
	// inside the target-to-source container. TS 24.501 §4.4.3.1 has the UE read it
	// as it would an initial SECURITY MODE COMMAND, which is why it carries its
	// own message authentication code.
	Container fgs.S1ModeToN1ModeNASTransparentContainer
}

// DefaultUE5GSecurityCapability is what the AMF assumes when the MME forwards no
// UE 5G security capability: NEA0, 128-NEA1 and 128-NEA2 for ciphering, and
// 128-NIA1 and 128-NIA2 for integrity (TS 33.501 §8.4.2 step 3). NIA0 is
// deliberately absent — an assumed capability must not be the one that permits
// unprotected signalling.
//
// The set is stated as the 5GS capability element codes it: algorithm n occupies
// bit 8-n (TS 24.501 §9.11.3.54).
var DefaultUE5GSecurityCapability = fgs.UESecurityCapability{
	EA: nas.Algorithms(uint8(nas.CipheringNull), uint8(nas.CipheringSNOW3G), uint8(nas.CipheringAES)),
	IA: nas.Algorithms(uint8(nas.IntegritySNOW3G), uint8(nas.IntegrityAES)),
}

// MapTo5GSOnHandover derives the mapped 5G security context for an EPS to 5GS
// handover, along with the NAS container that tells the UE how to reproduce it
// (TS 33.501 §8.4.2 step 3, §8.6.2, Annex A.9, A.10, A.15.2).
//
// intOrder and encOrder are the operator's 5G NAS algorithm preference. Selection
// is against the UE's 5G security capability alone; the EPS algorithms travelling
// beside it say nothing about what the UE accepts in N1 mode, and are only stored.
func MapTo5GSOnHandover(in EPSSecurityContext, intOrder []nas.IntegrityAlgorithm, encOrder []nas.CipheringAlgorithm) (EPSTo5GSHandover, error) {
	// A.15.2: P0 is the NH of the EPS context, and the key is K_ASME.
	kamf, err := deriveKey(in.KASME[:], fcKAMFPrimeHandover, in.NH[:])
	if err != nil {
		return EPSTo5GSHandover{}, fmt.Errorf("derive K'AMF: %w", err)
	}

	capability := DefaultUE5GSecurityCapability
	if in.UE5GSecurityCapability != nil {
		capability = *in.UE5GSecurityCapability
	}

	nea, nia, ok := fgs.SelectNASAlgorithms(capability, intOrder, encOrder)
	if !ok {
		return EPSTo5GSHandover{}, fmt.Errorf("interworking: no 5G NAS algorithm common to the UE and the operator policy")
	}

	knasEnc, err := fivegskeys.DeriveKNASEnc(kamf[:], nea)
	if err != nil {
		return EPSTo5GSHandover{}, fmt.Errorf("derive K_NASenc: %w", err)
	}

	knasInt, err := fivegskeys.DeriveKNASInt(kamf[:], nia)
	if err != nil {
		return EPSTo5GSHandover{}, fmt.Errorf("derive K_NASint: %w", err)
	}

	// A.9 with the uplink NAS COUNT parameter set to 2³²−1, for the same reason as
	// the initial K_eNB in the other direction: the value can never be one a real
	// message used, so this K_gNB can never be derived twice (§8.4.2 NOTE 3).
	temporaryKgNB, err := fivegskeys.DeriveKgNB(kamf[:], nas.CountOutOfRange)
	if err != nil {
		return EPSTo5GSHandover{}, fmt.Errorf("derive temporary K_gNB: %w", err)
	}

	// A.10: the chain advances once past the pair handed to the gNB, and the AMF
	// keeps {NCC=1, NH}.
	nh, err := fivegskeys.DeriveNH(kamf[:], temporaryKgNB[:])
	if err != nil {
		return EPSTo5GSHandover{}, fmt.Errorf("derive NH: %w", err)
	}

	ngKSI := mappedKSI(in.EKSI)

	// The NCC in the container is neither of the two above: it names the NH that
	// K'AMF was derived from, so the UE knows how far along its *EPS* chain to
	// look (§8.4.2 step 3, TS 33.401 §7.2.8.4.4).
	container := fgs.S1ModeToN1ModeNASTransparentContainer{
		CipheringAlgorithm: nea,
		IntegrityAlgorithm: nia,
		NCC:                in.NCC,
		NgKSI:              ngKSI,
	}

	mac, err := macForContainer(container, knasInt, nia)
	if err != nil {
		return EPSTo5GSHandover{}, err
	}

	container.MessageAuthenticationCode = mac

	return EPSTo5GSHandover{
		Context: Mapped5GSecurityContext{
			KAMF:                 kamf,
			NgKSI:                ngKSI,
			ULNASCount:           0,
			DLNASCount:           nas.Count(0).Next(), // the container consumed COUNT 0
			Ciphering:            nea,
			Integrity:            nia,
			KNASEnc:              knasEnc,
			KNASInt:              knasInt,
			UESecurityCapability: capability,
			EPSAlgorithms:        in.Algorithms,
			TemporaryKgNB:        temporaryKgNB,
			NH:                   nh,
			NCC:                  1,
		},
		Container: container,
	}, nil
}

// macForContainer computes the container's message authentication code
// (TS 33.501 §6.9.2.3.3): the selected NIA over the container's protected
// octets, keyed with the mapped context's K_NASint, downlink, on the 3GPP NAS
// connection, at COUNT 2³²−1.
func macForContainer(c fgs.S1ModeToN1ModeNASTransparentContainer, knasInt nas.IntegrityKey, nia nas.IntegrityAlgorithm) ([4]byte, error) {
	protected, err := c.MACProtected()
	if err != nil {
		return [4]byte{}, fmt.Errorf("interworking: %w", err)
	}

	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:    nia,
		IntegrityKey: knasInt,
		// The container is not ciphered, so no ciphering key is needed; NEA0 stands
		// in for one rather than the code pretending to have selected it.
		Ciphering: nas.CipheringNull,
		// NIA0 authenticates nothing, but the container announces the very algorithms
		// the operator's policy selected, and every other NAS message in such a
		// deployment is equally unauthenticated. Refusing only here would invent a
		// second, stricter policy and turn an operator's explicit choice into a
		// handover that fails for no reason it could observe.
		AllowNullIntegrity: nia == nas.IntegrityNull,
	})
	if err != nil {
		return [4]byte{}, fmt.Errorf("interworking: NAS container security context: %w", err)
	}

	mac, err := sc.MACAtCountOutOfRange(protected, nas.Bearer3GPP, nas.DirectionDownlink)
	if err != nil {
		return [4]byte{}, fmt.Errorf("interworking: NAS container MAC: %w", err)
	}

	return mac, nil
}

// mappedKSI re-labels a key set identifier for the context mapped from the one it
// names: the value field carries over and the type field becomes "mapped"
// (TS 33.501 §8.6.1, §8.6.2).
func mappedKSI(k nas.KeySetIdentifier) nas.KeySetIdentifier {
	return nas.KeySetIdentifier{Value: k.Value, Mapped: true}
}

// deriveKey runs the TS 33.220 KDF for a single-parameter interworking
// derivation and checks that a full-length key came back.
func deriveKey(key []byte, fc string, p0 []byte) ([keyLen]byte, error) {
	var out [keyLen]byte

	derived, err := ueauth.GetKDFValue(key, fc, p0, ueauth.KDFLen(p0))
	if err != nil {
		return out, err
	}

	if len(derived) != keyLen {
		return out, fmt.Errorf("interworking: derived key is %d octets, want %d", len(derived), keyLen)
	}

	copy(out[:], derived)

	return out, nil
}

// count32 renders a NAS COUNT as the four-octet KDF parameter both A.14 and
// TS 33.401 A.3 take.
func count32(v uint32) []byte {
	p := make([]byte, 4)
	binary.BigEndian.PutUint32(p, v)

	return p
}
