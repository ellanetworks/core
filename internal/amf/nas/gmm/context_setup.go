package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/free5gc/nas/nasMessage"
)

func contextSetup(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe, msg *nasMessage.RegistrationRequest) error {
	ctx, span := tracer.Start(ctx, "nas/context_setup")
	defer span.End()

	ue.TransitionTo(amf.ContextSetup)
	ue.RegistrationRequest = msg

	switch ue.RegistrationType5GS {
	case nasMessage.RegistrationType5GSInitialRegistration:
		if err := HandleInitialRegistration(ctx, amfInstance, ue); err != nil {
			return fmt.Errorf("error handling initial registration: %v", err)
		}
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		fallthrough
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue); err != nil {
			return fmt.Errorf("error handling mobility and periodic registration updating: %v", err)
		}
	}

	return nil
}
