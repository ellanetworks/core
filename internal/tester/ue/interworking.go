// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

const fcKAMFPrimeHandover = "76"

type MappedFromEPS struct {
	KASME     [32]byte
	NH        [32]byte
	Container fgs.S1ModeToN1ModeNASTransparentContainer
}

func (ue *UE) InstallMappedSecurityContextFromEPS(in MappedFromEPS) error {
	kamf, err := ueauth.GetKDFValue(in.KASME[:], fcKAMFPrimeHandover, in.NH[:], ueauth.KDFLen(in.NH[:]))
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

	nea, nia := uint8(in.Container.CipheringAlgorithm), uint8(in.Container.IntegrityAlgorithm)

	if err := AlgorithmKeyDerivation(nea, kamf, &knasEnc, nia, &knasInt); err != nil {
		return fmt.Errorf("derive the mapped 5G NAS keys: %w", err)
	}

	if err := verifyMappedContainer(in.Container, nia, knasInt); err != nil {
		return err
	}

	ue.UeSecurity.Kamf = kamf
	ue.UeSecurity.CipheringAlg, ue.UeSecurity.IntegrityAlg = nea, nia
	ue.UeSecurity.KnasEnc, ue.UeSecurity.KnasInt = knasEnc, knasInt
	ue.UeSecurity.NgKsi = models.NgKsi{Ksi: int32(in.Container.NgKSI.Value), Tsc: models.ScTypeMapped}
	ue.UeSecurity.ULCount = 0
	ue.UeSecurity.DLRecv.Reset()
	ue.UeSecurity.contextFromAuthentication = false

	logger.UeLogger.Debug("Installed the mapped 5G NAS security context",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Int32("ngKSI", ue.UeSecurity.NgKsi.Ksi),
		zap.Uint8("NEA", nea),
		zap.Uint8("NIA", nia),
		zap.Uint8("NCC", in.Container.NCC),
	)

	return nil
}

func verifyMappedContainer(c fgs.S1ModeToN1ModeNASTransparentContainer, nia uint8, knasInt [16]uint8) error {
	protected, err := c.MACProtected()
	if err != nil {
		return fmt.Errorf("encode the protected part of the NAS container: %w", err)
	}

	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:          nas.IntegrityAlgorithm(nia),
		IntegrityKey:       knasInt,
		Ciphering:          nas.CipheringNull,
		AllowNullIntegrity: nas.IntegrityAlgorithm(nia) == nas.IntegrityNull,
	})
	if err != nil {
		return fmt.Errorf("build the NAS container security context: %w", err)
	}

	if err := sc.VerifyMACAtMaxCount(protected, c.MessageAuthenticationCode, nas.Bearer3GPP, nas.DirectionDownlink); err != nil {
		return fmt.Errorf("the NAS container's MAC does not verify, so the handover is aborted: %w", err)
	}

	return nil
}

func (ue *UE) SendMobilityRegistrationUpdate(amfUENGAPID, ranUENGAPID int64) error {
	if ue.Gnb == nil {
		return fmt.Errorf("GNB is not set for UE")
	}

	plain, err := BuildRegistrationRequest(&RegistrationRequestOpts{
		RegistrationType:  uint8(fgs.RegistrationTypeMobilityUpdating),
		IncludeCapability: true,
		UESecurity:        ue.UeSecurity,
		UEStatus:          &fgs.UEStatus{S1ModeReg: true},
	})
	if err != nil {
		return fmt.Errorf("could not build Registration Request NAS PDU: %w", err)
	}

	wire, err := ue.EncodeNasPduWithSecurity(plain, uint8(fgs.SHTIntegrityProtectedCiphered))
	if err != nil {
		return fmt.Errorf("could not protect Registration Request NAS PDU: %w", err)
	}

	if err := ue.Gnb.SendUplinkNAS(wire, amfUENGAPID, ranUENGAPID); err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %w", err)
	}

	logger.UeLogger.Debug("Sent the mobility Registration Request after a handover from EPS",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}
