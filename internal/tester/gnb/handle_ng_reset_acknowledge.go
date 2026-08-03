// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handleNGResetAcknowledge(value []byte) error {
	ack, err := ngap.ParseNGResetAcknowledge(value)
	if err != nil {
		return fmt.Errorf("could not parse NGResetAcknowledge: %w", err)
	}

	logger.GnbLogger.Debug("Received NGResetAcknowledge",
		zap.Int("ConnectionsReset", len(ack.ConnectionList)))

	return nil
}
