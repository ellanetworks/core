// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handleNGSetupFailure(value []byte) error {
	fail, err := ngap.ParseNGSetupFailure(value)
	if err != nil {
		return fmt.Errorf("could not parse NGSetupFailure: %w", err)
	}

	cause := "absent"
	if fail.Cause != nil {
		cause = fail.Cause.String()
	}

	logger.GnbLogger.Debug("Received NGSetupFailure", zap.String("Cause", cause))

	return nil
}
