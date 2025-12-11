package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/consumer"
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/free5gc/nas/nasMessage"
)

func identityVerification(ue *context.AmfUe) bool {
	return ue.Supi != "" || len(ue.Suci) != 0
}

func AuthenticationProcedure(ctx ctxt.Context, ue *context.AmfUe) (bool, error) {
	ctx, span := tracer.Start(ctx, "AuthenticationProcedure")
	defer span.End()

	if !identityVerification(ue) {
		// Request UE's SUCI by sending identity request
		ue.GmmLog.Debug("UE has no SUCI / SUPI - send identity request to UE")

		err := message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeSuci)
		if err != nil {
			return false, fmt.Errorf("error sending identity request: %v", err)
		}

		ue.GmmLog.Info("sent identity request")
		return false, nil
	}

	// Check whether UE has SUCI and SUPI
	ue.GmmLog.Debug("UE has SUCI / SUPI")

	if ue.SecurityContextIsValid() {
		ue.GmmLog.Debug("UE has a valid security context - skip the authentication procedure")
		return true, nil
	}

	ue.GmmLog.Debug("UE has no valid security context - continue with the authentication procedure")

	response, err := consumer.SendUEAuthenticationAuthenticateRequest(ctx, ue, nil)
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
