package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

func handleRegistrationComplete(ctx context.Context, ue *amfContext.AmfUe) error {
	if ue.GetState() != amfContext.ContextSetup {
		return fmt.Errorf("state mismatch: receive Registration Complete message in state %s", ue.GetState())
	}

	ue.SetState(amfContext.Registered)

	if ue.T3550 != nil {
		ue.T3550.Stop()
		ue.T3550 = nil // clear the timer
	}

	forPending := ue.RegistrationRequest.GetFOR() == nasMessage.FollowOnRequestPending

	udsHasPending := ue.RegistrationRequest.UplinkDataStatus != nil

	hasActiveSessions := ue.HasActivePduSessions()

	shouldRelease := !forPending && !udsHasPending && !hasActiveSessions

	if shouldRelease {
		ue.RanUe.ReleaseAction = amfContext.UeContextN2NormalRelease

		err := ue.RanUe.Radio.NGAPSender.SendUEContextReleaseCommand(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
	}

	ue.ClearRegistrationRequestData()

	return nil
}
