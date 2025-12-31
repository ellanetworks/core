package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
)

// TS 33.501 6.7.2
func handleSecurityModeComplete(ctx context.Context, amf *amfContext.AMF, ue *amfContext.AmfUe, msg *nas.GmmMessage) error {
	if ue.State != amfContext.SecurityMode {
		return fmt.Errorf("state mismatch: receive Security Mode Complete message in state %s", ue.State)
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

	if msg.IMEISV != nil {
		ue.Pei = nasConvert.PeiToString(msg.IMEISV.Octet[:])
	}

	if msg.SecurityModeComplete.NASMessageContainer != nil {
		contents := msg.SecurityModeComplete.GetNASMessageContainerContents()

		m := nas.NewMessage()
		if err := m.GmmMessageDecode(&contents); err != nil {
			return fmt.Errorf("failed to decode nas message container: %v", err)
		}

		messageType := m.GmmHeader.GetMessageType()
		if messageType != nas.MsgTypeRegistrationRequest {
			return fmt.Errorf("nas message container Iei type error")
		}

		return contextSetup(ctx, amf, ue, m.RegistrationRequest)
	}

	err := contextSetup(ctx, amf, ue, ue.RegistrationRequest)
	if err != nil {
		return fmt.Errorf("error in context setup: %v", err)
	}

	return nil
}
