package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleSecurityModeReject(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Security Mode Reject", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "AMF NAS HandleSecurityModeReject")
	defer span.End()

	if ue.State.Current() != context.SecurityMode {
		return fmt.Errorf("state mismatch: receive Security Mode Reject message in state %s", ue.State.Current())
	}

	ue.State.Set(context.Deregistered)

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	cause := msg.SecurityModeReject.Cause5GMM.GetCauseValue()

	ue.Log.Warn("Reject", zap.String("Cause", nasMessage.Cause5GMMToString(cause)))
	ue.Log.Error("UE reject the security mode command, abort the ongoing procedure")

	ue.SecurityContextAvailable = false

	err := ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
