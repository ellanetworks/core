// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ue

import (
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/nas"
)

func handleDeregistrationRequestUETerminated(ue *UE, _ *nas.Message, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Debug("Received Deregistration Request UE Terminated NAS message")
	return nil
}
