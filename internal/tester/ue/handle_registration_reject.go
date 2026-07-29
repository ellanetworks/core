// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func handleRegistrationReject(ue *UE, plain []byte) error {
	rej, err := fgs.ParseRegistrationReject(plain)
	if err != nil {
		return fmt.Errorf("could not parse Registration Reject: %v", err)
	}

	logger.UeLogger.Debug(
		"Received Registration Reject NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.String("Cause", cause5GMMToString(rej.Cause)),
	)

	return nil
}
