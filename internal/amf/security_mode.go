// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
)

func securityMode(ctx context.Context, amfInstance *AMF, ue *UeContext) error {
	logger.WithTrace(ctx, logger.AmfLog).Debug("Security Mode Procedure", logger.SUPI(ue.supi.String()))

	ctx, span := gmmTracer.Start(ctx, "nas/security_mode")
	defer span.End()

	ue.TransitionTo(SecurityMode)

	ue.Log = ue.Log.With(logger.SUPI(ue.supi.String()))

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	if ue.SecurityContextIsValid() {
		ue.Log.Debug("UE has a valid security context - skip security mode control procedure")
		return contextSetup(ctx, amfInstance, ue, conn.RegistrationRequest)
	}

	integrityOrder, cipheringOrder, err := amfInstance.GetSecurityAlgorithms(ctx)
	if err != nil {
		return fmt.Errorf("error getting security algorithms: %v", err)
	}

	err = ue.SelectSecurityAlg(integrityOrder, cipheringOrder)
	if err != nil {
		return fmt.Errorf("NAS security algorithm negotiation failed: %v", err)
	}

	err = ue.DerivateAlgKey()
	if err != nil {
		return fmt.Errorf("error deriving algorithm key: %v", err)
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	if _, beginErr := conn.Procedures.Begin(conn.Ctx(), procedure.Procedure{Type: procedure.SecurityMode}); beginErr != nil {
		return fmt.Errorf("security mode blocked by conflict: %w", beginErr)
	}

	err = SendSecurityModeCommand(ctx, amfInstance, ranUe)
	if err != nil {
		conn.Procedures.End(procedure.SecurityMode)

		return fmt.Errorf("error sending security mode command: %v", err)
	}

	return nil
}
