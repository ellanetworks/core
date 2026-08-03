// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handleNGSetupResponse(value []byte) error {
	resp, err := ngap.ParseNGSetupResponse(value)
	if err != nil {
		return fmt.Errorf("could not parse NGSetupResponse: %w", err)
	}

	logger.GnbLogger.Debug(
		"Received NGSetupResponse",
		zap.String("AMFName", resp.AMFName),
		zap.Int("GUAMIListCount", len(resp.ServedGUAMIList)),
		zap.Int("RelativeAMFCapacity", int(derefUint8(resp.RelativeAMFCapacity))),
		zap.Int("PLMNSupportListCount", len(resp.PLMNSupportList)),
	)

	return nil
}

func derefUint8(p *uint8) uint8 {
	if p == nil {
		return 0
	}

	return *p
}
