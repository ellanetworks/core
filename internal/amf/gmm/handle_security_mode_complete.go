package gmm

import (
	ctxt "context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// TS 33.501 6.7.2
func HandleSecurityModeComplete(ctx ctxt.Context, ue *context.AmfUe, securityModeComplete *nasMessage.SecurityModeComplete) error {
	logger.AmfLog.Debug("Handle Security Mode Complete", zap.String("supi", ue.Supi))

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	if ue.SecurityContextIsValid() {
		ue.UpdateSecurityContext()
	}

	if securityModeComplete.IMEISV != nil {
		ue.GmmLog.Debug("receieve IMEISV")
		ue.Pei = nasConvert.PeiToString(securityModeComplete.IMEISV.Octet[:])
	}

	if securityModeComplete.NASMessageContainer != nil {
		contents := securityModeComplete.NASMessageContainer.GetNASMessageContainerContents()
		m := nas.NewMessage()
		if err := m.GmmMessageDecode(&contents); err != nil {
			return err
		}

		messageType := m.GmmMessage.GmmHeader.GetMessageType()
		if messageType != nas.MsgTypeRegistrationRequest && messageType != nas.MsgTypeServiceRequest {
			ue.GmmLog.Error("nas message container Iei type error")
			return errors.New("nas message container Iei type error")
		} else {
			ue.State.Set(context.ContextSetup)
			return contextSetup(ctx, ue, m.GmmMessage.RegistrationRequest)
		}
	}
	ue.State.Set(context.ContextSetup)

	return contextSetup(ctx, ue, ue.RegistrationRequest)
}

func handleSecurityModeComplete(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.SecurityMode:
		err := HandleSecurityModeComplete(ctx, ue, msg.SecurityModeComplete)
		if err != nil {
			return fmt.Errorf("error handling security mode complete: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Security Mode Complete message in state %s", ue.State.Current())
	}

	return nil
}
