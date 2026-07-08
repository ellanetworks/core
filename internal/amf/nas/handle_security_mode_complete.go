// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// TS 33.501
func handleSecurityModeComplete(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.SecurityModeComplete, integrityVerified bool) {
	if step := ue.RegStep(); step != amf.RegStepSecurityMode {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Security Mode Complete message outside the security mode exchange", zap.String("state", string(ue.State())))
		return
	}

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return
	}

	conn.StopNASGuard()

	conn.Parent().EndKeyChainProc(procedure.SecurityMode)

	if ue.SecurityContextIsValid() && integrityVerified {
		err := ue.UpdateSecurityContext()
		if err != nil {
			abortRegistration(ctx, amfInstance, ue, "update security context", err)
			return
		}
	}

	if msg.IMEISV != nil {
		pei, err := etsi.NewIMEIFromPEI(nasConvert.PeiToString(msg.IMEISV.Octet[:]))
		if err != nil {
			// A malformed IMEISV yields no trusted equipment identity; reject and release
			// (the NAS guard has been stopped).
			amf.SendRegistrationReject(ctx, conn, nasMessage.Cause5GMMProtocolErrorUnspecified)
			ue.Deregister(ctx)

			return
		}

		ue.Imei = pei
	}

	if msg.NASMessageContainer != nil {
		contents := msg.GetNASMessageContainerContents()

		m := nas.NewMessage()
		if err := m.GmmMessageDecode(&contents); err != nil {
			abortRegistration(ctx, amfInstance, ue, "decode NAS message container", err)
			return
		}

		messageType := m.GmmHeader.GetMessageType()
		if messageType != nas.MsgTypeRegistrationRequest {
			abortRegistration(ctx, amfInstance, ue, "unexpected NAS container message type", nil)
			return
		}

		contextSetup(ctx, amfInstance, ue, m.RegistrationRequest)

		return
	}

	contextSetup(ctx, amfInstance, ue, conn.RegistrationRequest)
}
