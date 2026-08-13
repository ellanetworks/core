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

type IdleRegistrationOpts struct {
	RANUENGAPID            int64
	MappedGUTI             fgs.MobileIdentity
	EPSNASMessageContainer []byte
	PDUSessionStatus       *[16]bool
	Mapped                 MappedFromEPSIdle
}

func (ue *UE) SendIdleMobilityRegistration(opts IdleRegistrationOpts) error {
	if ue.Gnb == nil {
		return fmt.Errorf("GNB is not set for UE")
	}

	ue.UeSecurity.Guti = &opts.MappedGUTI
	ue.UeSecurity.NgKsi = models.NgKsi{Ksi: int32(opts.Mapped.EKSI), Tsc: models.ScTypeMapped}

	plain, err := BuildRegistrationRequest(&RegistrationRequestOpts{
		RegistrationType:       uint8(fgs.RegistrationTypeMobilityUpdating),
		IncludeCapability:      true,
		UESecurity:             ue.UeSecurity,
		UEStatus:               &fgs.UEStatus{S1ModeReg: true},
		EPSNASMessageContainer: opts.EPSNASMessageContainer,
		PDUSessionStatus:       opts.PDUSessionStatus,
	})
	if err != nil {
		return fmt.Errorf("could not build the Registration Request of an inter-system change: %w", err)
	}

	gutiIE, err := opts.MappedGUTI.MarshalBinary()
	if err != nil {
		return fmt.Errorf("could not encode the mapped 5G-GUTI: %w", err)
	}

	if err := ue.Gnb.SendInitialUEMessage(plain, opts.RANUENGAPID, gutiIE, ngap.RRCCauseMOSignalling); err != nil {
		return fmt.Errorf("could not send the Initial UE Message: %w", err)
	}

	return ue.InstallMappedSecurityContextForIdleMobility(opts.Mapped)
}
