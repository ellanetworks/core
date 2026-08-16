// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

const fcKAMFPrimeIdle = "75"

type MappedFromEPSIdle struct {
	KASME          [32]byte
	UplinkNASCount nas.Count
	EKSI           uint8
	Ciphering      uint8
	Integrity      uint8
}

func (ue *UE) InstallMappedSecurityContextForIdleMobility(in MappedFromEPSIdle) error {
	p0 := make([]byte, 4)
	binary.BigEndian.PutUint32(p0, in.UplinkNASCount.Value())

	kamf, err := ueauth.GetKDFValue(in.KASME[:], fcKAMFPrimeIdle, p0, ueauth.KDFLen(p0))
	if err != nil {
		return fmt.Errorf("derive K'AMF: %w", err)
	}

	if len(kamf) != len(in.KASME) {
		return fmt.Errorf("K'AMF is %d octets, want %d", len(kamf), len(in.KASME))
	}

	var (
		knasEnc [16]uint8
		knasInt [16]uint8
	)

	if err := AlgorithmKeyDerivation(in.Ciphering, kamf, &knasEnc, in.Integrity, &knasInt); err != nil {
		return fmt.Errorf("derive the mapped 5G NAS keys: %w", err)
	}

	ue.UeSecurity.Kamf = kamf
	ue.UeSecurity.CipheringAlg, ue.UeSecurity.IntegrityAlg = in.Ciphering, in.Integrity
	ue.UeSecurity.KnasEnc, ue.UeSecurity.KnasInt = knasEnc, knasInt
	ue.UeSecurity.NgKsi = models.NgKsi{Ksi: int32(in.EKSI), Tsc: models.ScTypeMapped}
	ue.UeSecurity.ULCount = 0
	ue.UeSecurity.DLCount = 0
	ue.UeSecurity.contextFromAuthentication = false

	return nil
}

func (ue *UE) TakeUplinkNASCountForInterSystemChange() nas.Count {
	spent := ue.UeSecurity.ULCount
	ue.UeSecurity.ULCount = spent.Next()

	return spent
}

// IdleRegistrationOpts drives an inter-system change from S1 mode to N1 mode in
// 5GMM-IDLE mode (TS 24.501 §4.4.2.5).
//
// Which of the spec's two cases it performs follows from the UE's own state, as
// it does for a real UE: one holding a valid native 5G NAS security context
// resumes on it (case a), one holding none arrives on the context the AMF maps
// from EPS (case b). Both present the 5G-GUTI mapped from the 4G-GUTI as their
// mobile identity, which is what lets the AMF claim the UE's PDN connections
// from the MME. Case a additionally carries the native 5G-GUTI in the Additional
// GUTI IE, cites the native ngKSI, integrity protects the request with that
// context, and continues its NAS COUNTs rather than restarting them
// (§5.5.1.3.2 case a).
type IdleRegistrationOpts struct {
	RANUENGAPID int64
	// MappedGUTI is the 5G-GUTI mapped from the 4G-GUTI the MME assigned.
	MappedGUTI             fgs.MobileIdentity
	EPSNASMessageContainer []byte
	// PDUSessionStatus names the PDU sessions the UE does not hold inactive.
	PDUSessionStatus *[16]bool
	// UplinkDataStatus asks the AMF to re-establish the user plane of the named
	// sessions as part of the arrival (TS 24.501 §9.11.3.57). Nil arrives without
	// one, leaving the UE in CM-IDLE with its sessions preserved — the 5GS
	// counterpart of a TRACKING AREA UPDATE REQUEST with the active flag clear.
	UplinkDataStatus *[16]bool
	// Mapped is the EPS security context to map onto 5GS, used only when the UE
	// holds no native 5G context of its own to resume on.
	Mapped MappedFromEPSIdle
}

// SendIdleMobilityRegistration sends the REGISTRATION REQUEST of an
// inter-system change from S1 mode in 5GMM-IDLE mode.
func (ue *UE) SendIdleMobilityRegistration(opts IdleRegistrationOpts) error {
	if ue.Gnb == nil {
		return fmt.Errorf("GNB is not set for UE")
	}

	native := ue.nativeContextForIdleArrival()

	cleartext := &RegistrationRequestOpts{
		RegistrationType:       uint8(fgs.RegistrationTypeMobilityUpdating),
		UESecurity:             ue.UeSecurity,
		UEStatus:               &fgs.UEStatus{S1ModeReg: true},
		EPSNASMessageContainer: opts.EPSNASMessageContainer,
		MobileIdentity:         &opts.MappedGUTI,
		AdditionalGUTI:         native,
	}

	if native != nil {
		// TS 24.501 §4.4.6 case b): the non-cleartext IEs travel in the ciphered
		// NAS message container of the request itself, the 5GMM capability among
		// them — §4.4.6 lists the cleartext IEs exhaustively and it is not one.
		cleartext.PDUSessionStatus = opts.PDUSessionStatus
		cleartext.UplinkDataStatus = opts.UplinkDataStatus
		cleartext.IncludeCapability = true
	} else {
		// §4.4.6 case a): with no context to cipher a container with, the request
		// carries the cleartext IEs alone and the AMF asks for the rest in the
		// SECURITY MODE COMPLETE.
		ue.UeSecurity.Guti = &opts.MappedGUTI
		ue.UeSecurity.NgKsi = models.NgKsi{Ksi: int32(opts.Mapped.EKSI), Tsc: models.ScTypeMapped}
	}

	plain, err := BuildRegistrationRequest(cleartext)
	if err != nil {
		return fmt.Errorf("could not build the Registration Request of an inter-system change: %w", err)
	}

	wire := plain

	if native != nil {
		// §4.4.2.5 case a): integrity protected on the native context, with the
		// uplink NAS COUNT carrying on from where NR left it.
		if wire, err = ue.EncodeNasPduWithSecurity(plain, uint8(fgs.SHTIntegrityProtected)); err != nil {
			return fmt.Errorf("could not integrity-protect the Registration Request of an inter-system change: %w", err)
		}
	} else if err := ue.armReplay(opts); err != nil {
		return err
	}

	// The RAN-level identity follows the 5GS mobile identity IE: the UE arrives
	// on a radio that knows it by the mapped 5G-S-TMSI.
	gutiIE, err := opts.MappedGUTI.MarshalBinary()
	if err != nil {
		return fmt.Errorf("could not encode the mapped 5G-GUTI: %w", err)
	}

	if err := ue.Gnb.SendInitialUEMessage(wire, opts.RANUENGAPID, gutiIE, ngap.RRCCauseMOSignalling); err != nil {
		return fmt.Errorf("could not send the Initial UE Message: %w", err)
	}

	if native != nil {
		return nil
	}

	return ue.InstallMappedSecurityContextForIdleMobility(opts.Mapped)
}

// nativeContextForIdleArrival returns the UE's own 5G-GUTI when it holds a
// native 5G NAS security context to resume on, and nil when it does not.
func (ue *UE) nativeContextForIdleArrival() *fgs.MobileIdentity {
	if ue.UeSecurity.NgKsi.Ksi == ngKSINoKey || ue.UeSecurity.NgKsi.Tsc != models.ScTypeNative {
		return nil
	}

	if ue.UeSecurity.Guti == nil || ue.UeSecurity.Guti.GUTI == nil {
		return nil
	}

	guti := *ue.UeSecurity.Guti

	return &guti
}

// armReplay stores the complete REGISTRATION REQUEST for the AMF to ask back in
// the SECURITY MODE COMPLETE (TS 24.501 §4.4.6 case a). Only the arrival that
// runs a security mode procedure needs it: a UE resuming on its native context
// expects no such command, so arming it there would let the UE answer one it
// never planned for and hide the defect from the scenario.
func (ue *UE) armReplay(opts IdleRegistrationOpts) error {
	replay := &fgs.RegistrationRequest{
		RegistrationType:       fgs.RegistrationTypeMobilityUpdating,
		FOR:                    true,
		NgKSI:                  nas.KeySetIdentifier{Value: opts.Mapped.EKSI, Mapped: true},
		MobileIdentity:         opts.MappedGUTI,
		UEStatus:               &fgs.UEStatus{S1ModeReg: true},
		EPSNASMessageContainer: opts.EPSNASMessageContainer,
		GMMCapability:          &fgs.GMMCapability{RestrictEC: true, LPP: true, HOAttach: true, S1Mode: true},
		UESecurityCapability:   &ue.UeSecurity.UeSecurityCapability,
	}

	if opts.PDUSessionStatus != nil {
		var bitmap fgs.PSIBitmap

		bitmap.PSI = *opts.PDUSessionStatus
		replay.PDUSessionStatus = &bitmap
	}

	if opts.UplinkDataStatus != nil {
		var bitmap fgs.PSIBitmap

		bitmap.PSI = *opts.UplinkDataStatus
		replay.UplinkDataStatus = &bitmap
	}

	replayBytes, err := replay.MarshalBinary()
	if err != nil {
		return fmt.Errorf("could not build the replayed Registration Request: %w", err)
	}

	ue.replayRegistration = replayBytes

	return nil
}
