// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

// TS 23.502 4.2.2.3
func handleDeregistrationRequestUEOriginatingDeregistration(ctx context.Context, ue *amf.UeContext, msg *nasMessage.DeregistrationRequestUEOriginatingDeregistration, integrityVerified bool) error {
	if state := ue.GetState(); state != amf.Registered {
		return fmt.Errorf("state mismatch: receive Deregistration Request (UE Originating Deregistration) message in state %s", state)
	}

	// Reject unauthenticated Deregistration Requests while the amf.AMF still
	// holds a valid security context (TS 24.501 §4.4.4.3 defense in depth).
	// A UE that lost its keys can recover via Initial Registration.
	if !integrityVerified && ue.HasSecurityContext() {
		return fmt.Errorf("rejecting unauthenticated Deregistration Request from UE with valid security context")
	}

	defer ue.Deregister(ctx)

	ranUe := ue.RanUe()
	if ranUe == nil {
		logger.WithTrace(ctx, logger.AmfLog).Warn("amf.RanUe is nil, cannot send UE Context Release Command", logger.SUPI(ue.SupiValue().String()))
		return nil
	}

	if msg.GetSwitchOff() == 0 {
		amf.SendDeregistrationAccept(ctx, ranUe)

		ue.Log.Info("sent deregistration accept")
	}

	// TS 23.502 4.2.6, 4.12.3
	targetDeregistrationAccessType := msg.GetAccessType()
	if targetDeregistrationAccessType != nasMessage.AccessType3GPP {
		return nil
	}

	ranUe.ReleaseAction = amf.UeContextReleaseUeContext

	err := ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
