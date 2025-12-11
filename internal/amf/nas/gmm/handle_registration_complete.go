package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleRegistrationComplete(ctx ctxt.Context, ue *context.AmfUe) error {
	logger.AmfLog.Debug("Handle Registration Complete", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "AMF NAS HandleRegistrationComplete")
	defer span.End()

	if ue.State.Current() != context.ContextSetup {
		return fmt.Errorf("state mismatch: receive Registration Complete message in state %s", ue.State.Current())
	}

	if ue.T3550 != nil {
		ue.T3550.Stop()
		ue.T3550 = nil // clear the timer
	}

	forPending := ue.RegistrationRequest.GetFOR() == nasMessage.FollowOnRequestPending

	uds := ue.RegistrationRequest.UplinkDataStatus

	udsHasPending := uds != nil

	hasActiveSessions := ue.HasActivePduSessions()

	shouldRelease := !(forPending || udsHasPending || hasActiveSessions)

	if shouldRelease {
		err := ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
	}

	ue.State.Set(context.Registered)
	ue.ClearRegistrationRequestData()

	return nil
}
