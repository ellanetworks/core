// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handleErrorIndication(value []byte) error {
	ind, err := ngap.ParseErrorIndication(value)
	if err != nil {
		return fmt.Errorf("could not parse ErrorIndication: %w", err)
	}

	cause := "(none)"
	if ind.Cause != nil {
		cause = ind.Cause.String()
	}

	logger.GnbLogger.Error("Received ErrorIndication", zap.String("Cause", cause))

	return nil
}
