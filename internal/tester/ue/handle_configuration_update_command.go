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
