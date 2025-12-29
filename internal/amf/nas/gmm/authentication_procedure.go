package gmm

import (
	"context"
	"fmt"
	"strconv"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
)

func sendUEAuthenticationAuthenticateRequest(ctx context.Context, ue *amfContext.AmfUe, resynchronizationInfo *models.ResynchronizationInfo) (*models.Av5gAka, error) {
	if ue.Tai.PlmnID == nil {
		return nil, fmt.Errorf("tai is not available in UE context")
	}

	mnc, err := strconv.Atoi(ue.Tai.PlmnID.Mnc)
	if err != nil {
		return nil, fmt.Errorf("could not convert mnc to int: %v", err)
	}

	authInfo := models.AuthenticationInfo{
		Suci:                  ue.Suci,
		ServingNetworkName:    fmt.Sprintf("5G:mnc%03d.mcc%s.3gppnetwork.org", mnc, ue.Tai.PlmnID.Mcc),
		ResynchronizationInfo: resynchronizationInfo,
	}

	ueAuthenticationCtx, err := ausf.UeAuthPostRequestProcedure(ctx, authInfo)
	if err != nil {
		return nil, fmt.Errorf("ausf UE Authentication Authenticate Request failed: %s", err.Error())
	}

	return ueAuthenticationCtx, nil
}

func identityVerification(ue *amfContext.AmfUe) bool {
	return ue.Supi != "" || len(ue.Suci) != 0
}

func AuthenticationProcedure(ctx context.Context, ue *amfContext.AmfUe) (bool, error) {
	ctx, span := tracer.Start(ctx, "AuthenticationProcedure")
	defer span.End()

	if !identityVerification(ue) {
		// Request UE's SUCI by sending identity request
		ue.Log.Debug("UE has no SUCI / SUPI - send identity request to UE")

		err := message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeSuci)
		if err != nil {
			return false, fmt.Errorf("error sending identity request: %v", err)
		}

		ue.Log.Info("sent identity request")
		return false, nil
	}

	// Check whether UE has SUCI and SUPI
	ue.Log.Debug("UE has SUCI / SUPI")

	if ue.SecurityContextIsValid() {
		ue.Log.Debug("UE has a valid security context - skip the authentication procedure")
		return true, nil
	}

	ue.Log.Debug("UE has no valid security context - continue with the authentication procedure")

	response, err := sendUEAuthenticationAuthenticateRequest(ctx, ue, nil)
	if err != nil {
		return false, fmt.Errorf("failed to send ue authentication request: %s", err)
	}

	ue.AuthenticationCtx = response

	ue.ABBA = []uint8{0x00, 0x00} // set ABBA value as described at TS 33.501 Annex A.7.1

	err = message.SendAuthenticationRequest(ctx, ue.RanUe)
	if err != nil {
		return false, fmt.Errorf("error sending authentication request: %v", err)
	}

	return false, nil
}
