// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

func handleRegistrationComplete(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) error {
	if ue.GetState() != amf.ContextSetup {
		return fmt.Errorf("state mismatch: receive Registration Complete message in state %s", ue.GetState())
	}

	ue.TransitionTo(amf.Registered)

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	conn.T3550.Stop()

	// UE confirmed receipt of the new GUTI — free the old one (TS 24.501)
	amfInstance.FreeOldGuti(ue)

	// Configuration update command carries NITZ (network name + time zone) per TS 24.501.
	amf.SendConfigurationUpdateCommand(ctx, amfInstance, ue, false)

	forPending := conn.RegistrationRequest.GetFOR() == nasMessage.FollowOnRequestPending

	udsHasPending := conn.RegistrationRequest.UplinkDataStatus != nil

	hasActiveSessions := ue.HasActivePduSessions()

	shouldRelease := !forPending && !udsHasPending && !hasActiveSessions

	if shouldRelease {
		ranUe := ue.RanUe()
		if ranUe == nil {
			return fmt.Errorf("ue is not connected to RAN")
		}

		ranUe.ReleaseAction = amf.UeContextN2NormalRelease

		err := ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
	}

	ue.ClearRegistrationRequestData()

	return nil
}
