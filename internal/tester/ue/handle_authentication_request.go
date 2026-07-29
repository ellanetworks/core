// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func handleAuthenticationRequest(ue *UE, plain []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Debug("Received Authentication Request NAS message")

	req, err := fgs.ParseAuthenticationRequest(plain)
	if err != nil {
		return fmt.Errorf("could not parse Authentication Request: %v", err)
	}

	if req.RAND == nil || req.AUTN == nil {
		return fmt.Errorf("missing RAND or AUTN in Authentication Request")
	}

	paramAutn, err := ue.DeriveRESstarAndSetKey(ue.UeSecurity.AuthenticationSubs, req.RAND[:], ue.UeSecurity.Snn, req.AUTN[:])
	if err != nil {
		return fmt.Errorf("could not derive RES* and set key: %v", err)
	}

	authResp, err := BuildAuthenticationResponse(&AuthenticationResponseOpts{
		AuthenticationResponseParam: paramAutn,
		EapMsg:                      "",
	})
	if err != nil {
		return fmt.Errorf("could not build authentication response: %v", err)
	}

	err = ue.Gnb.SendUplinkNAS(authResp, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send Authentication Response: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Authentication Response NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}
