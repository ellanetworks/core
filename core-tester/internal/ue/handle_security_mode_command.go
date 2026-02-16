package ue

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/openapi/models"
	"go.uber.org/zap"
)

func handleSecurityModeCommand(ue *UE, msg *nas.Message, amfUENGAPID int64, ranUENGAPID int64) error {
	if ue.Gnb == nil {
		return fmt.Errorf("GNB is not set for UE")
	}

	logger.UeLogger.Debug("Received Security Mode Command NAS message")

	ksi := int32(msg.SecurityModeCommand.GetNasKeySetIdentifiler())

	var tsc models.ScType

	switch msg.SecurityModeCommand.GetTSC() {
	case nasMessage.TypeOfSecurityContextFlagNative:
		tsc = models.ScType_NATIVE
	case nasMessage.TypeOfSecurityContextFlagMapped:
		tsc = models.ScType_MAPPED
	}

	ue.UeSecurity.NgKsi.Ksi = ksi
	ue.UeSecurity.NgKsi.Tsc = tsc

	logger.UeLogger.Debug(
		"Updated UE security NG KSI",
		zap.Int32("KSI", ksi),
		zap.String("TSC", string(tsc)),
	)

	securityModeComplete, err := BuildSecurityModeComplete(&SecurityModeCompleteOpts{
		UESecurity: ue.UeSecurity,
		IMEISV:     ue.IMEISV,
	})
	if err != nil {
		return fmt.Errorf("error sending Security Mode Complete: %w", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(securityModeComplete, nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext)
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
