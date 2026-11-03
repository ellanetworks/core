// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
)

func handleDeregistrationRequestUETerminated(ue *UE, _ []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Debug("Received Deregistration Request UE Terminated NAS message")

	accept, err := (&fgs.DeregistrationAcceptUETerminated{}).MarshalBinary()
	if err != nil {
		return fmt.Errorf("build Deregistration Accept: %w", err)
	}

	encoded, err := ue.EncodeNasPduWithSecurity(accept, uint8(fgs.SHTIntegrityProtectedCiphered))
	if err != nil {
		return fmt.Errorf("encode Deregistration Accept: %w", err)
	}

	if err := ue.Gnb.SendUplinkNAS(encoded, amfUENGAPID, ranUENGAPID); err != nil {
		return fmt.Errorf("send Deregistration Accept: %w", err)
	}

	logger.UeLogger.Debug("Sent Deregistration Accept NAS message")

	return nil
}
