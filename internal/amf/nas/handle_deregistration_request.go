// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// TS 23.502
func handleDeregistrationRequestUEOriginatingDeregistration(ctx context.Context, ue *amf.UeContext, plain []byte, integrityVerified bool) nasreply.Disposition {
	// No state precondition: TS 24.501 §5.5.2.2.2 has the network process the
	// UE-initiated de-registration and enter 5GMM-DEREGISTERED regardless of the
	// current state; the integrity guard below is the security control.

	// Reject unauthenticated Deregistration Requests while the AMF still holds a valid
	// security context (TS 24.501 defense in depth). A UE that lost its keys can
	// recover via Initial Registration.
	if !integrityVerified && ue.Secured() {
		logger.From(ctx, logger.AmfLog).Warn("rejecting unauthenticated Deregistration Request from UE with valid security context")
		return nasreply.Silent(nasreply.ReasonIntegrityFail)
	}

	defer ue.Deregister(ctx)

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.WithTrace(ctx, logger.AmfLog).Warn("amf.UeConn is nil, cannot send UE Context Release Command", logger.SUPI(ue.Supi().String()))
		return nasreply.Handled()
	}

	msg, err := fgs.ParseDeregistrationRequestUEOriginating(plain)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not decode Deregistration Request", zap.Error(err))
		return nasreply.Handled()
	}

	if !msg.SwitchOff {
		amf.SendDeregistrationAccept(ctx, ueConn)

		logger.From(ctx, logger.AmfLog).Info("sent deregistration accept")
	}

	// TS 23.502
	if msg.AccessType != fgs.AccessType3GPP {
		return nasreply.Handled()
	}

	ueConn.ReleaseAction = amf.UeContextReleaseUeContext

	ueConn.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)

	return nasreply.Handled()
}
