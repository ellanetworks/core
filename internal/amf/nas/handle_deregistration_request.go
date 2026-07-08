// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// TS 23.502
func handleDeregistrationRequestUEOriginatingDeregistration(ctx context.Context, ue *amf.UeContext, msg *nasMessage.DeregistrationRequestUEOriginatingDeregistration, integrityVerified bool) {
	// No state precondition: TS 24.501 §5.5.2.2.2 has the network process the
	// UE-initiated de-registration and enter 5GMM-DEREGISTERED regardless of the
	// current state; the integrity guard below is the security control.

	// Reject unauthenticated Deregistration Requests while the AMF still holds a valid
	// security context (TS 24.501 defense in depth). A UE that lost its keys can
	// recover via Initial Registration.
	if !integrityVerified && ue.Secured() {
		logger.From(ctx, logger.AmfLog).Warn("rejecting unauthenticated Deregistration Request from UE with valid security context")
		return
	}

	defer ue.Deregister(ctx)

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.WithTrace(ctx, logger.AmfLog).Warn("amf.UeConn is nil, cannot send UE Context Release Command", logger.SUPI(ue.Supi().String()))
		return
	}

	if msg.GetSwitchOff() == 0 {
		amf.SendDeregistrationAccept(ctx, ueConn)

		logger.From(ctx, logger.AmfLog).Info("sent deregistration accept")
	}

	// TS 23.502
	targetDeregistrationAccessType := msg.GetAccessType()
	if targetDeregistrationAccessType != nasMessage.AccessType3GPP {
		return
	}

	ueConn.ReleaseAction = amf.UeContextReleaseUeContext

	err := ueConn.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("error sending ue context release command", zap.Error(err))
		return
	}
}
