package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// TS 33.501 6.7.2
func handleSecurityModeComplete(ctx context.Context, ue *amfContext.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Security Mode Complete", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "AMF NAS HandleSecurityModeComplete")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	if ue.State.Current() != amfContext.SecurityMode {
		return fmt.Errorf("state mismatch: receive Security Mode Complete message in state %s", ue.State.Current())
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	if ue.SecurityContextIsValid() {
		err := ue.UpdateSecurityContext()
		if err != nil {
			return fmt.Errorf("error updating security context: %v", err)
		}
	}

	if msg.SecurityModeComplete.IMEISV != nil {
		ue.Pei = nasConvert.PeiToString(msg.SecurityModeComplete.IMEISV.Octet[:])
	}

	if msg.SecurityModeComplete.NASMessageContainer != nil {
		contents := msg.SecurityModeComplete.NASMessageContainer.GetNASMessageContainerContents()

		m := nas.NewMessage()
		if err := m.GmmMessageDecode(&contents); err != nil {
			return fmt.Errorf("failed to decode nas message container: %v", err)
		}

		messageType := m.GmmMessage.GmmHeader.GetMessageType()
		if messageType != nas.MsgTypeRegistrationRequest && messageType != nas.MsgTypeServiceRequest {
			return fmt.Errorf("nas message container Iei type error")
		}

		ue.State.Set(amfContext.ContextSetup)

		return contextSetup(ctx, ue, m.GmmMessage.RegistrationRequest)
	}

	ue.State.Set(amfContext.ContextSetup)

	err := contextSetup(ctx, ue, ue.RegistrationRequest)
	if err != nil {
		return fmt.Errorf("error in context setup: %v", err)
	}

	return nil
}
