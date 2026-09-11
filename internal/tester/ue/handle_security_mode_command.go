// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func handleSecurityModeCommand(ue *UE, plain []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	if ue.Gnb == nil {
		return fmt.Errorf("GNB is not set for UE")
	}

	logger.UeLogger.Debug("Received Security Mode Command NAS message")

	smc, err := fgs.ParseSecurityModeCommand(plain)
	if err != nil {
		return fmt.Errorf("could not parse Security Mode Command: %v", err)
	}

	if !smc.ReplayedUESecurityCapability.Equal(ue.UeSecurity.UeSecurityCapability) {
		return fmt.Errorf("replayed UE security capability is %s, want the %s the UE sent: a real UE answers Security Mode Reject (TS 33.501 §6.7.2)",
			smc.ReplayedUESecurityCapability, ue.UeSecurity.UeSecurityCapability)
	}

	ksi := int32(smc.NgKSI.Value)

	tsc := models.ScTypeNative
	if smc.NgKSI.Mapped {
		tsc = models.ScTypeMapped
	}

	ue.UeSecurity.NgKsi.Ksi = ksi
	ue.UeSecurity.NgKsi.Tsc = tsc

	logger.UeLogger.Debug(
		"Updated UE security NG KSI",
		zap.Int32("KSI", ksi),
		zap.String("TSC", string(tsc)),
	)

	storeEPSNASAlgorithms(ue, smc)

	replay := ue.replayForSecurityModeComplete(smc)
	imeisvRequested := smc.IMEISVRequested != nil && *smc.IMEISVRequested == fgs.IMEISVRequested

	securityModeComplete, err := BuildSecurityModeComplete(&SecurityModeCompleteOpts{
		UESecurity:    ue.UeSecurity,
		IMEISV:        ue.IMEISV,
		Replay:        replay,
		OmitContainer: replay == nil,
		OmitIMEISV:    !imeisvRequested,
	})
	if err != nil {
		return fmt.Errorf("error sending Security Mode Complete: %w", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(securityModeComplete, uint8(fgs.SHTIntegrityProtectedCipheredNewContext))
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE  NAS Security Mode Complete message: %v", ue.UeSecurity.Supi, err)
	}

	err = ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Security Mode Complete NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}

// TS 33.501 §6.7.2, §8.3.2
func storeEPSNASAlgorithms(ue *UE, smc *fgs.SecurityModeCommand) {
	if smc.SelectedEPSNASSecurityAlgorithms == nil {
		return
	}

	ue.UeSecurity.EPSNASAlgorithms = smc.SelectedEPSNASSecurityAlgorithms

	logger.UeLogger.Debug("Stored the selected EPS NAS security algorithms",
		zap.Uint8("EEA", uint8(smc.SelectedEPSNASSecurityAlgorithms.Ciphering)),
		zap.Uint8("EIA", uint8(smc.SelectedEPSNASSecurityAlgorithms.Integrity)))
}

func (ue *UE) replayForSecurityModeComplete(smc *fgs.SecurityModeCommand) []byte {
	rinmr := smc.AdditionalSecurityInformation != nil && smc.AdditionalSecurityInformation.RINMR

	if !ue.replayPending && !rinmr {
		return nil
	}

	ue.replayPending = false

	return ue.replayRegistration
}
