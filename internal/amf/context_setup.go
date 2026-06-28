// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/free5gc/nas/nasMessage"
)

func contextSetup(ctx context.Context, amfInstance *AMF, ue *UeContext, msg *nasMessage.RegistrationRequest) error {
	ctx, span := gmmTracer.Start(ctx, "nas/context_setup")
	defer span.End()

	ue.TransitionTo(ContextSetup)

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	conn.RegistrationRequest = msg

	switch conn.RegistrationType5GS {
	case nasMessage.RegistrationType5GSInitialRegistration:
		if err := HandleInitialRegistration(ctx, amfInstance, ue); err != nil {
			return fmt.Errorf("error handling initial registration: %v", err)
		}
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		fallthrough
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue); err != nil {
			return fmt.Errorf("error handling mobility and periodic registration updating: %v", err)
		}
	}

	return nil
}
