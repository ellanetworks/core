// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func handleAuthenticationReject(ue *UE, plain []byte) error {
	if _, err := fgs.ParseAuthenticationReject(plain); err != nil {
		return fmt.Errorf("could not parse Authentication Reject: %v", err)
	}

	logger.UeLogger.Debug("Received Authentication Reject NAS message", zap.String("IMSI", ue.UeSecurity.Supi))

	return nil
}
