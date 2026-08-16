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

type IdleRegistrationOpts struct {
	RANUENGAPID            int64
	MappedGUTI             fgs.MobileIdentity
	EPSNASMessageContainer []byte
	PDUSessionStatus       *[16]bool
	UplinkDataStatus       *[16]bool
	Mapped                 MappedFromEPSIdle
}

type NativeIdleRegistrationOpts struct {
	RANUENGAPID            int64
	MappedGUTI             fgs.MobileIdentity
	EPSNASMessageContainer []byte
	PDUSessionStatus       *[16]bool
	UplinkDataStatus       *[16]bool
}

func (ue *UE) SendIdleMobilityRegistration(opts IdleRegistrationOpts) error {
	if ue.Gnb == nil {
		return fmt.Errorf("GNB is not set for UE")
	}

	ue.UeSecurity.Guti = &opts.MappedGUTI
	ue.UeSecurity.NgKsi = models.NgKsi{Ksi: int32(opts.Mapped.EKSI), Tsc: models.ScTypeMapped}

	cleartext := &RegistrationRequestOpts{
		RegistrationType:       uint8(fgs.RegistrationTypeMobilityUpdating),
		IncludeCapability:      true,
		UESecurity:             ue.UeSecurity,
		UEStatus:               &fgs.UEStatus{S1ModeReg: true},
		EPSNASMessageContainer: opts.EPSNASMessageContainer,
	}

	plain, err := BuildRegistrationRequest(cleartext)
	if err != nil {
		return fmt.Errorf("could not build the Registration Request of an inter-system change: %w", err)
	}

	replayMsg := &fgs.RegistrationRequest{
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
		replayMsg.PDUSessionStatus = &bitmap
	}

	if opts.UplinkDataStatus != nil {
		var bitmap fgs.PSIBitmap

		bitmap.PSI = *opts.UplinkDataStatus
		replayMsg.UplinkDataStatus = &bitmap
	}

	replayBytes, err := replayMsg.MarshalBinary()
	if err != nil {
		return fmt.Errorf("could not build the replayed Registration Request: %w", err)
	}

	ue.replayRegistration = replayBytes

	gutiIE, err := opts.MappedGUTI.MarshalBinary()
	if err != nil {
		return fmt.Errorf("could not encode the mapped 5G-GUTI: %w", err)
	}

	if err := ue.Gnb.SendInitialUEMessage(plain, opts.RANUENGAPID, gutiIE, ngap.RRCCauseMOSignalling); err != nil {
		return fmt.Errorf("could not send the Initial UE Message: %w", err)
	}

	return ue.InstallMappedSecurityContextForIdleMobility(opts.Mapped)
}

func (ue *UE) SendNativeIdleMobilityRegistration(opts NativeIdleRegistrationOpts) error {
	if ue.Gnb == nil {
		return fmt.Errorf("GNB is not set for UE")
	}

	if ue.UeSecurity.Guti == nil || ue.UeSecurity.Guti.GUTI == nil {
		return fmt.Errorf("the UE holds no native 5G-GUTI to arrive from EPS with")
	}

	if ue.UeSecurity.NgKsi.Ksi == ngKSINoKey {
		return fmt.Errorf("the UE holds no native 5G NAS security context to arrive from EPS on")
	}

	native := *ue.UeSecurity.Guti

	cleartext := &RegistrationRequestOpts{
		RegistrationType:       uint8(fgs.RegistrationTypeMobilityUpdating),
		IncludeCapability:      true,
		UESecurity:             ue.UeSecurity,
		UEStatus:               &fgs.UEStatus{S1ModeReg: true},
		EPSNASMessageContainer: opts.EPSNASMessageContainer,
		PDUSessionStatus:       opts.PDUSessionStatus,
		UplinkDataStatus:       opts.UplinkDataStatus,
		MobileIdentity:         &opts.MappedGUTI,
		AdditionalGUTI:         &native,
	}

	plain, err := BuildRegistrationRequest(cleartext)
	if err != nil {
		return fmt.Errorf("could not build the Registration Request of an inter-system change: %w", err)
	}

	replayMsg := &fgs.RegistrationRequest{
		RegistrationType:       fgs.RegistrationTypeMobilityUpdating,
		FOR:                    true,
		NgKSI:                  nas.KeySetIdentifier{Value: uint8(ue.UeSecurity.NgKsi.Ksi)},
		MobileIdentity:         opts.MappedGUTI,
		AdditionalGUTI:         &native,
		UEStatus:               &fgs.UEStatus{S1ModeReg: true},
		EPSNASMessageContainer: opts.EPSNASMessageContainer,
		GMMCapability:          &fgs.GMMCapability{RestrictEC: true, LPP: true, HOAttach: true, S1Mode: true},
		UESecurityCapability:   &ue.UeSecurity.UeSecurityCapability,
	}

	if opts.PDUSessionStatus != nil {
		var bitmap fgs.PSIBitmap

		bitmap.PSI = *opts.PDUSessionStatus
		replayMsg.PDUSessionStatus = &bitmap
	}

	if opts.UplinkDataStatus != nil {
		var bitmap fgs.PSIBitmap

		bitmap.PSI = *opts.UplinkDataStatus
		replayMsg.UplinkDataStatus = &bitmap
	}

	replayBytes, err := replayMsg.MarshalBinary()
	if err != nil {
		return fmt.Errorf("could not build the replayed Registration Request: %w", err)
	}

	ue.replayRegistration = replayBytes

	wire, err := ue.EncodeNasPduWithSecurity(plain, uint8(fgs.SHTIntegrityProtected))
	if err != nil {
		return fmt.Errorf("could not integrity-protect the Registration Request of an inter-system change: %w", err)
	}

	gutiIE, err := opts.MappedGUTI.MarshalBinary()
	if err != nil {
		return fmt.Errorf("could not encode the mapped 5G-GUTI: %w", err)
	}

	if err := ue.Gnb.SendInitialUEMessage(wire, opts.RANUENGAPID, gutiIE, ngap.RRCCauseMOSignalling); err != nil {
		return fmt.Errorf("could not send the Initial UE Message: %w", err)
	}

	return nil
}
