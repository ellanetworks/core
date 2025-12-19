package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// TS 23.502 4.2.2.3
func handleDeregistrationRequestUEOriginatingDeregistration(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Deregistration Request", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "AMF NAS HandleDeregistrationRequestUEOriginatingDeregistration")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	if ue.State.Current() != context.Registered {
		return fmt.Errorf("state mismatch: receive Deregistration Request (UE Originating Deregistration) message in state %s", ue.State.Current())
	}

	if msg == nil {
		return fmt.Errorf("gmm message is nil")
	}

	ue.State.Set(context.Deregistered)

	targetDeregistrationAccessType := msg.DeregistrationRequestUEOriginatingDeregistration.GetAccessType()

	for _, smContext := range ue.SmContextList {
		err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
		if err != nil {
			ue.Log.Error("Release SmContext Error", zap.Error(err))
		}
	}

	// if Deregistration type is not switch-off, send Deregistration Accept
	if msg.DeregistrationRequestUEOriginatingDeregistration.GetSwitchOff() == 0 && ue.RanUe != nil {
		err := message.SendDeregistrationAccept(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("error sending deregistration accept: %v", err)
		}
		ue.Log.Info("sent deregistration accept")
	}

	// TS 23.502 4.2.6, 4.12.3
	if targetDeregistrationAccessType != nasMessage.AccessType3GPP {
		return fmt.Errorf("only 3gpp access type is supported")
	}

	if ue.RanUe != nil {
		err := ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
	}

	return nil
}
