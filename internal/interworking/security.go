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
	fcKASMEPrimeIdle     = "73" // A.14.1: K_AMF → K'ASME, P0 = uplink 5G NAS COUNT
	fcKASMEPrimeHandover = "74" // A.14.2: K_AMF → K'ASME, P0 = downlink 5G NAS COUNT
	fcKAMFPrimeIdle      = "75" // A.15.1: K_ASME → K'AMF, P0 = uplink EPS NAS COUNT of the TAU
	fcKAMFPrimeHandover  = "76" // A.15.2: K_ASME → K'AMF, P0 = NH
)

const keyLen = 32

type EPSNASAlgorithms struct {
	Ciphering nas.CipheringAlgorithm // EEA
	Integrity nas.IntegrityAlgorithm // EIA
}

type FiveGToEPSHandover struct {
	Context   EPSSecurityContext
	Container fgs.N1ModeToS1ModeNASTransparentContainer
}

type FiveGToEPSInput struct {
	KAMF                   []byte
	NgKSI                  nas.KeySetIdentifier
	ULNASCount             nas.Count
	DLNASCount             nas.Count
	Algorithms             EPSNASAlgorithms
	UESecurityCapability   eps.UESecurityCapability
	UE5GSecurityCapability *fgs.UESecurityCapability
}

func MapToEPSOnHandover(in FiveGToEPSInput) (FiveGToEPSHandover, error) {
	if len(in.KAMF) != keyLen {
		return FiveGToEPSHandover{}, fmt.Errorf("interworking: K_AMF is %d octets, want %d", len(in.KAMF), keyLen)
	}

	kasme, err := deriveKey(in.KAMF, fcKASMEPrimeHandover, count32(in.DLNASCount.Value()))
	if err != nil {
		return FiveGToEPSHandover{}, fmt.Errorf("derive K'ASME: %w", err)
	}

	kenb, err := epskeys.DeriveKeNB(kasme[:], nas.MaxCount)
	if err != nil {
		return FiveGToEPSHandover{}, fmt.Errorf("derive initial K_eNB: %w", err)
	}

	nh1, err := epskeys.DeriveNH(kasme[:], kenb[:])
	if err != nil {
		return FiveGToEPSHandover{}, fmt.Errorf("derive NH: %w", err)
	}

	nh2, err := epskeys.DeriveNH(kasme[:], nh1[:])
	if err != nil {
		return FiveGToEPSHandover{}, fmt.Errorf("derive NH: %w", err)
	}

	return FiveGToEPSHandover{
		Context: EPSSecurityContext{
			KASME:                  kasme,
			EKSI:                   mappedKSI(in.NgKSI),
			ULNASCount:             in.ULNASCount,
			DLNASCount:             in.DLNASCount.Next(),
			Algorithms:             in.Algorithms,
			UESecurityCapability:   in.UESecurityCapability,
			UE5GSecurityCapability: in.UE5GSecurityCapability,
			NH:                     nh2,
			NCC:                    2,
		},
		Container: fgs.NewN1ModeToS1ModeNASTransparentContainer(in.DLNASCount),
	}, nil
}

type EPSSecurityContext struct {
	KASME                  [keyLen]byte
	EKSI                   nas.KeySetIdentifier
	ULNASCount             nas.Count
	DLNASCount             nas.Count
	Algorithms             EPSNASAlgorithms
	UESecurityCapability   eps.UESecurityCapability
	NH                     [keyLen]byte
	NCC                    uint8
	UE5GSecurityCapability *fgs.UESecurityCapability
}

type Mapped5GSecurityContext struct {
	KAMF                  [keyLen]byte
	NgKSI                 nas.KeySetIdentifier
	DLNASCount            nas.Count
	Ciphering             nas.CipheringAlgorithm
	Integrity             nas.IntegrityAlgorithm
	KNASEnc               nas.CipherKey
	KNASInt               nas.IntegrityKey
	UESecurityCapability  fgs.UESecurityCapability
	EPSAlgorithms         EPSNASAlgorithms
	EPSSecurityCapability eps.UESecurityCapability
	TemporaryKgNB         [keyLen]byte
	NH                    [keyLen]byte
	NCC                   uint8
}

type EPSTo5GSHandover struct {
	Context   Mapped5GSecurityContext
	Container fgs.S1ModeToN1ModeNASTransparentContainer
}

var DefaultUE5GSecurityCapability = fgs.UESecurityCapability{
	EA: nas.Algorithms(uint8(nas.CipheringNull), uint8(nas.CipheringSNOW3G), uint8(nas.CipheringAES)),
	IA: nas.Algorithms(uint8(nas.IntegritySNOW3G), uint8(nas.IntegrityAES)),
}

func MapTo5GSOnHandover(in EPSSecurityContext, intOrder []nas.IntegrityAlgorithm, encOrder []nas.CipheringAlgorithm) (EPSTo5GSHandover, error) {
	kamf, err := deriveKey(in.KASME[:], fcKAMFPrimeHandover, in.NH[:])
	if err != nil {
		return EPSTo5GSHandover{}, fmt.Errorf("derive K'AMF: %w", err)
	}

	capability := mapped5GSecurityCapability(in)

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

	temporaryKgNB, err := fivegskeys.DeriveKgNB(kamf[:], nas.MaxCount)
	if err != nil {
		return EPSTo5GSHandover{}, fmt.Errorf("derive temporary K_gNB: %w", err)
	}

	nh, err := fivegskeys.DeriveNH(kamf[:], temporaryKgNB[:])
	if err != nil {
		return EPSTo5GSHandover{}, fmt.Errorf("derive NH: %w", err)
	}

	ngKSI := mappedKSI(in.EKSI)

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
			KAMF:                  kamf,
			NgKSI:                 ngKSI,
			DLNASCount:            nas.Count(0).Next(), // the container consumed COUNT 0
			Ciphering:             nea,
			Integrity:             nia,
			KNASEnc:               knasEnc,
			KNASInt:               knasInt,
			UESecurityCapability:  capability,
			EPSAlgorithms:         in.Algorithms,
			EPSSecurityCapability: in.UESecurityCapability,
			TemporaryKgNB:         temporaryKgNB,
			NH:                    nh,
			NCC:                   1,
		},
		Container: container,
	}, nil
}

func macForContainer(c fgs.S1ModeToN1ModeNASTransparentContainer, knasInt nas.IntegrityKey, nia nas.IntegrityAlgorithm) ([4]byte, error) {
	protected, err := c.MACProtected()
	if err != nil {
		return [4]byte{}, fmt.Errorf("interworking: %w", err)
	}

	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:          nia,
		IntegrityKey:       knasInt,
		Ciphering:          nas.CipheringNull,
		AllowNullIntegrity: nia == nas.IntegrityNull,
	})
	if err != nil {
		return [4]byte{}, fmt.Errorf("interworking: NAS container security context: %w", err)
	}

	mac, err := sc.MACAtMaxCount(protected, nas.Bearer3GPP, nas.DirectionDownlink)
	if err != nil {
		return [4]byte{}, fmt.Errorf("interworking: NAS container MAC: %w", err)
	}

	return mac, nil
}

func mapped5GSecurityCapability(in EPSSecurityContext) fgs.UESecurityCapability {
	if in.UE5GSecurityCapability != nil {
		return *in.UE5GSecurityCapability
	}

	capability := DefaultUE5GSecurityCapability
	capability.HasEPS = true
	capability.EEA = in.UESecurityCapability.EEA
	capability.EIA = in.UESecurityCapability.EIA &^ 1

	return capability
}

func mappedKSI(k nas.KeySetIdentifier) nas.KeySetIdentifier {
	return nas.KeySetIdentifier{Value: k.Value, Mapped: true}
}

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

func count32(v uint32) []byte {
	p := make([]byte, 4)
	binary.BigEndian.PutUint32(p, v)

	return p
}
