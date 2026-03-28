package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

func handleRegistrationComplete(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe) error {
	if ue.GetState() != amf.ContextSetup {
		return fmt.Errorf("state mismatch: receive Registration Complete message in state %s", ue.GetState())
	}

	ue.TransitionTo(amf.Registered)

	if ue.T3550 != nil {
		ue.T3550.Stop()
		ue.T3550 = nil // clear the timer
	}

	// UE confirmed receipt of the new GUTI — free the old one (TS 24.501 5.5.1.2.4 step 20)
	amfInstance.FreeOldGuti(ue)

	// Send NITZ (network name + timezone) to UE per TS 24.501
	message.SendConfigurationUpdateCommand(ctx, amfInstance, ue, false)

	forPending := ue.RegistrationRequest.GetFOR() == nasMessage.FollowOnRequestPending

	udsHasPending := ue.RegistrationRequest.UplinkDataStatus != nil

	hasActiveSessions := ue.HasActivePduSessions()

	shouldRelease := !forPending && !udsHasPending && !hasActiveSessions

	if shouldRelease {
		ue.RanUe().ReleaseAction = amf.UeContextN2NormalRelease

		err := ue.RanUe().SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
	}

	ue.ClearRegistrationRequestData()

	return nil
}
