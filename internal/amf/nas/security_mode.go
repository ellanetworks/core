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
	logger.WithTrace(ctx, logger.AmfLog).Debug("Security Mode Procedure", logger.SUPI(ue.Supi().String()))

	ctx, span := gmmTracer.Start(ctx, "nas/security_mode")
	defer span.End()

	ue.AdvanceRegStep(amf.RegStepSecurityMode)

	ue.Log = ue.Log.With(logger.SUPI(ue.Supi().String()))

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	if ue.SecurityContextIsValid() {
		ue.Log.Debug("UE has a valid security context - skip security mode control procedure")
		return contextSetup(ctx, amfInstance, ue, conn.RegistrationRequest)
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	// Claim the security mode procedure before deriving keys, so a conflicting
	// key-changing procedure (e.g. an in-flight N2 handover) blocks the re-key
	// before the security context is mutated (TS 33.501 §6.9.5.1).
	if _, beginErr := conn.Procedures.Begin(conn.Ctx(), procedure.Procedure{Type: procedure.SecurityMode}); beginErr != nil {
		return fmt.Errorf("security mode blocked by conflict: %w", beginErr)
	}

	// The claim is released on any failure before the SECURITY MODE COMMAND is sent,
	// so a later procedure is not blocked; on success it stays in flight until
	// SECURITY MODE COMPLETE, the T3560 abort callback, or UE context release
	// (TS 24.501).
	committed := false

	defer func() {
		if !committed {
			conn.Procedures.End(procedure.SecurityMode)
		}
	}()

	integrityOrder, cipheringOrder, err := amfInstance.SecurityAlgorithms(ctx)
	if err != nil {
		return fmt.Errorf("error getting security algorithms: %v", err)
	}

	if err := ue.SelectSecurityAlg(integrityOrder, cipheringOrder); err != nil {
		return fmt.Errorf("NAS security algorithm negotiation failed: %v", err)
	}

	if err := ue.DerivateAlgKey(); err != nil {
		return fmt.Errorf("error deriving algorithm key: %v", err)
	}

	if err := amf.SendSecurityModeCommand(ctx, amfInstance, ranUe); err != nil {
		return fmt.Errorf("send security mode command: %w", err)
	}

	committed = true

	return nil
}
