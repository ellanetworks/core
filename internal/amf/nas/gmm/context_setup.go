package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas/nasMessage"
)

func contextSetup(ctx context.Context, ue *amfContext.AmfUe, msg *nasMessage.RegistrationRequest) error {
	ctx, span := tracer.Start(ctx, "contextSetup")
	defer span.End()

	ue.RegistrationRequest = msg

	switch ue.RegistrationType5GS {
	case nasMessage.RegistrationType5GSInitialRegistration:
		if err := HandleInitialRegistration(ctx, ue); err != nil {
			return fmt.Errorf("error handling initial registration: %v", err)
		}
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		fallthrough
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, ue); err != nil {
			return fmt.Errorf("error handling mobility and periodic registration updating: %v", err)
		}
	}

	return nil
}
