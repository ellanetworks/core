package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/free5gc/nas/nasMessage"
)

func sendUEAuthenticationAuthenticateRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe, resyncInfo *ausf.ResyncInfo) (*ausf.AuthResult, error) {
	if ue.Tai.PlmnID == nil {
		return nil, fmt.Errorf("tai is not available in UE context")
	}

	ueAuthenticationCtx, err := amfInstance.Ausf.Authenticate(ctx, ue.Suci, *ue.Tai.PlmnID, resyncInfo)
	if err != nil {
		return nil, fmt.Errorf("ausf UE Authentication Authenticate Request failed: %s", err.Error())
	}

	return ueAuthenticationCtx, nil
}

func identityVerification(ue *amf.AmfUe) bool {
	return ue.Supi.IsValid() || len(ue.Suci) != 0
}

func authenticationProcedure(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe) (bool, error) {
	ctx, span := tracer.Start(ctx, "nas/authentication_procedure")
	defer span.End()

	ranUe := ue.RanUe()
	if ranUe == nil {
		return false, fmt.Errorf("ue is not connected to RAN")
	}

	if !identityVerification(ue) {
		// Request UE's SUCI by sending identity request
		ue.Log.Debug("UE has no SUCI / SUPI - send identity request to UE")

		err := message.SendIdentityRequest(ctx, ranUe, nasMessage.MobileIdentity5GSTypeSuci)
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

	response, err := sendUEAuthenticationAuthenticateRequest(ctx, amfInstance, ue, nil)
	if err != nil {
		return false, fmt.Errorf("failed to send ue authentication request: %s", err)
	}

	ue.AuthenticationCtx = response

	ue.ABBA = []uint8{0x00, 0x00} // set ABBA value as described at TS 33.501 Annex A.7.1

	err = message.SendAuthenticationRequest(ctx, amfInstance, ranUe)
	if err != nil {
		return false, fmt.Errorf("error sending authentication request: %v", err)
	}

	return false, nil
}
