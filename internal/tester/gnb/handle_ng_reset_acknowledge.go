// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package gnb

import (
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/ngap/ngapType"
)

func handleNGResetAcknowledge(_ *ngapType.NGResetAcknowledge) error {
	logger.GnbLogger.Debug("Received NGResetAcknowledge")

	return nil
}
