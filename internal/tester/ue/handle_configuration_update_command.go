// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func handleConfigurationUpdateCommand(ue *UE, plain []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	cmd, err := fgs.ParseConfigurationUpdateCommand(plain)
	if err != nil {
		return fmt.Errorf("could not parse Configuration Update Command: %v", err)
	}

	if cmd.GUTI != nil {
		ue.Set5gGuti(cmd.GUTI)
	}

	logNITZ(ue, cmd)

	if cmd.ConfigurationUpdateIndication == nil || !cmd.ConfigurationUpdateIndication.ACK {
		logger.UeLogger.Debug(
			"Configuration Update Command without acknowledgement requested, not replying",
			zap.String("IMSI", ue.UeSecurity.Supi),
		)

		return nil
	}

	commandComplete, err := BuildConfigurationUpdateComplete()
	if err != nil {
		return fmt.Errorf("could not build Configuration Update Complete NAS PDU: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(commandComplete, uint8(fgs.SHTIntegrityProtectedCiphered))
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE NAS Configuration Update Complete", ue.UeSecurity.Supi)
	}

	err = ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Configuration Update Complete NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}

// logNITZ reports the network identity and time the command carried, which is
// what a lab needs to confirm the core is the UE's time source rather than the
// RAN (TS 24.501 §8.2.19.7 to §8.2.19.11).
func logNITZ(ue *UE, cmd *fgs.ConfigurationUpdateCommand) {
	if cmd.UniversalTime == nil && cmd.LocalTimeZone == nil && cmd.DaylightSavingTime == nil {
		return
	}

	fields := []zap.Field{zap.String("IMSI", ue.UeSecurity.Supi)}

	if cmd.UniversalTime != nil {
		fields = append(fields, zap.Stringer("universalTime", cmd.UniversalTime))
	}

	if cmd.LocalTimeZone != nil {
		fields = append(fields, zap.Stringer("localTimeZone", cmd.LocalTimeZone))
	}

	if cmd.DaylightSavingTime != nil {
		fields = append(fields, zap.Stringer("daylightSavingTime", cmd.DaylightSavingTime))
	}

	logger.UeLogger.Info("Received network time in Configuration Update Command", fields...)
}
