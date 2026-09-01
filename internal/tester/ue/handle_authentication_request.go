// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"errors"
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
		if cause, ok := authenticationFailureCause(err); ok {
			return sendAuthenticationFailure(ue, cause, paramAutn, amfUENGAPID, ranUENGAPID)
		}

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

// authenticationFailureCause maps a failed AUTN check to the 5GMM cause the UE
// answers with (TS 24.501 §5.4.1.3.5).
func authenticationFailureCause(err error) (fgs.GMMCause, bool) {
	switch {
	case errors.Is(err, ErrSQNOutOfRange):
		return fgs.GMMCauseSynchFailure, true
	case errors.Is(err, ErrMACFailure):
		return fgs.GMMCauseMACFailure, true
	default:
		return 0, false
	}
}

// sendAuthenticationFailure answers an AUTHENTICATION REQUEST the UE could not
// accept. On a synch failure the network resynchronises from the AUTS and comes
// back with a fresh AUTHENTICATION REQUEST; on a MAC failure it rejects.
func sendAuthenticationFailure(ue *UE, cause fgs.GMMCause, auts []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	opts := &AuthenticationFailureOpts{Cause: cause}
	if cause == fgs.GMMCauseSynchFailure {
		opts.AUTS = auts
	}

	failure, err := BuildAuthenticationFailure(opts)
	if err != nil {
		return fmt.Errorf("could not build Authentication Failure: %v", err)
	}

	if err := ue.Gnb.SendUplinkNAS(failure, amfUENGAPID, ranUENGAPID); err != nil {
		return fmt.Errorf("could not send Authentication Failure: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Authentication Failure NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Stringer("cause", cause),
	)

	return nil
}
