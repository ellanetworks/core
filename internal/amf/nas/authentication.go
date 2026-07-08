// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
)

func sendUEAuthenticationAuthenticateRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, resyncInfo *ausf.ResyncInfo) (*ausf.AuthResult, error) {
	if ue.Tai.PlmnID == nil {
		return nil, fmt.Errorf("tai is not available in UE context")
	}

	ueAuthenticationCtx, err := amfInstance.Ausf.Authenticate(ctx, ue.Suci, *ue.Tai.PlmnID, resyncInfo)
	if err != nil {
		return nil, fmt.Errorf("ausf UE amf.Authentication Authenticate Request failed: %s", err.Error())
	}

	return ueAuthenticationCtx, nil
}

func identityVerification(ue *amf.UeContext) bool {
	return ue.Supi().IsValid() || len(ue.Suci) != 0
}

func authenticationProcedure(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) (bool, error) {
	ctx, span := gmmTracer.Start(ctx, "nas/authentication_procedure")
	defer span.End()

	ueConn := ue.Conn()
	if ueConn == nil {
		return false, fmt.Errorf("ue is not connected to RAN")
	}

	if !identityVerification(ue) {
		logger.From(ctx, logger.AmfLog).Debug("UE has no SUCI / SUPI - send identity request to UE")

		amf.SendIdentityRequest(ctx, amfInstance, ueConn, nasMessage.MobileIdentity5GSTypeSuci)

		logger.From(ctx, logger.AmfLog).Info("sent identity request")

		return false, nil
	}

	logger.From(ctx, logger.AmfLog).Debug("UE has SUCI / SUPI")

	if ue.SecurityContextIsValid() {
		logger.From(ctx, logger.AmfLog).Debug("UE has a valid security context - skip the authentication procedure")
		return true, nil
	}

	logger.From(ctx, logger.AmfLog).Debug("UE has no valid security context - continue with the authentication procedure")

	response, err := sendUEAuthenticationAuthenticateRequest(ctx, amfInstance, ue, nil)
	if err != nil {
		return false, fmt.Errorf("failed to send ue authentication request: %s", err)
	}

	conn := ue.Conn()
	if conn == nil {
		return false, fmt.Errorf("no active NAS connection")
	}

	conn.AuthenticationCtx = response

	ue.SetAbba([]uint8{0x00, 0x00}) // ABBA value per TS 33.501

	amf.SendAuthenticationRequest(ctx, amfInstance, ueConn)

	return false, nil
}
