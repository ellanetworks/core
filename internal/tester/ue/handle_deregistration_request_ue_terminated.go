// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"github.com/ellanetworks/core/internal/tester/logger"
)

func handleDeregistrationRequestUETerminated(ue *UE, _ []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Debug("Received Deregistration Request UE Terminated NAS message")
	return nil
}
