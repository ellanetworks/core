// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
)

func securityMode(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) error {
	logger.WithTrace(ctx, logger.AmfLog).Debug("Security Mode Procedure", logger.SUPI(ue.SupiValue().String()))

	ctx, span := gmmTracer.Start(ctx, "nas/security_mode")
	defer span.End()

	ue.TransitionTo(amf.SecurityMode)

	ue.Log = ue.Log.With(logger.SUPI(ue.SupiValue().String()))

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

	// The security mode control procedure stays in flight until SECURITY MODE
	// COMPLETE, the T3560 abort callback, or UE context release — not a single
	// transport send (TS 24.501).
	amf.SendSecurityModeCommand(ctx, amfInstance, ranUe)

	return nil
}
