// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// abortSecurityMode rejects the in-flight registration and releases the (pre-pool) UE
// after a technical failure during the security mode procedure, so it does not leak an
// open RAN connection under no supervision (the NAS guard is not yet armed, and the
// key-chain claim is released by the caller's !committed defer). Returns nil so the
// dispatcher does not also emit a 5GMM STATUS.
func abortSecurityMode(ctx context.Context, ue *amf.UeContext, ueConn *amf.UeConn, reason string, err error) {
	logger.From(ctx, logger.AmfLog).Error("security mode aborted, releasing UE", zap.String("reason", reason), zap.Error(err))
	metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(ueConn.RegistrationType5GS), metrics.ResultReject)
	amf.SendRegistrationReject(ctx, ueConn, fgs.GMMCauseProtocolErrorUnspecified)
	ue.Deregister(ctx)
}

func securityMode(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) {
	logger.WithTrace(ctx, logger.AmfLog).Debug("Security Mode Procedure", logger.SUPI(ue.Supi().String()))

	ctx, span := gmmTracer.Start(ctx, "nas/security_mode")
	defer span.End()

	ue.AdvanceRegStep(amf.RegStepSecurityMode)

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return
	}

	if ue.SecurityContextIsValid() && !conn.MappedContextFromEPS {
		if ue.NeedsEPSNASAlgorithms() && provideEPSNASAlgorithms(ctx, amfInstance, ue, conn) {
			return
		}

		logger.From(ctx, logger.AmfLog).Debug("UE has a valid security context - skip security mode control procedure")
		contextSetup(ctx, amfInstance, ue, conn.RegistrationRequest, conn.RegistrationRequestPlain)

		return
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
		return
	}

	// Claim the security mode procedure before deriving keys, so a conflicting
	// key-changing procedure (e.g. an in-flight N2 handover) blocks the re-key
	// before the security context is mutated (TS 33.501 §6.9.5.1).
	if !ue.BeginKeyChainProc(procedure.SecurityMode) {
		logger.From(ctx, logger.AmfLog).Warn("security mode blocked by a conflicting key-changing procedure")
		return
	}

	// Release the claim on any failure before SECURITY MODE COMMAND is sent so a later
	// procedure is not blocked; on success it stays in flight until SECURITY MODE
	// COMPLETE, T3560 abort, or UE context release (TS 24.501).
	committed := false

	defer func() {
		if !committed {
			ue.EndKeyChainProc(procedure.SecurityMode)
		}
	}()

	integrityOrder, cipheringOrder, err := amfInstance.SecurityAlgorithms(ctx)
	if err != nil {
		abortSecurityMode(ctx, ue, ueConn, "get security algorithms", err)
		return
	}

	nea, nia, ok := ue.SelectSecurityAlg(integrityOrder, cipheringOrder)
	if !ok {
		// The UE and operator policy share no NAS algorithm; reject the registration
		// and release the UE to avoid a half-registered UE with an open RAN connection
		// (5GMM cause #23).
		logger.From(ctx, logger.AmfLog).Warn("NAS security algorithm negotiation failed, rejecting registration")

		metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ueConn, fgs.GMMCauseUESecurityCapabilitiesMismatch)
		ue.Deregister(ctx)

		return
	}

	if err := ue.InstallNASSecurityContext(nea, nia, amf.MintAuthProofForSecurityMode()); err != nil {
		abortSecurityMode(ctx, ue, ueConn, "install NAS security context", err)
		return
	}

	selectEPSNASAlgorithms(ctx, ue, integrityOrder, cipheringOrder)

	if err := amf.SendSecurityModeCommand(ctx, amfInstance, ueConn); err != nil {
		abortSecurityMode(ctx, ue, ueConn, "send security mode command", err)
		return
	}

	committed = true
}

func selectEPSNASAlgorithms(ctx context.Context, ue *amf.UeContext, intOrder []nas.IntegrityAlgorithm, encOrder []nas.CipheringAlgorithm) bool {
	if !ue.NeedsEPSNASAlgorithms() {
		return false
	}

	if _, ok := ue.EPSNetworkCapability(); !ok {
		logger.From(ctx, logger.AmfLog).Debug("UE has sent no usable S1 UE network capability, not signalling EPS NAS algorithms")
		return false
	}

	selected, ok := ue.SelectEPSNASAlgorithms(intOrder, encOrder)
	if !ok {
		logger.From(ctx, logger.AmfLog).Warn("no EPS NAS security algorithm common to the UE and the operator policy, not signalling EPS NAS algorithms")
		return false
	}

	logger.From(ctx, logger.AmfLog).Debug("selected EPS NAS security algorithms for mobility to EPS",
		zap.Stringer("eea", selected.Ciphering), zap.Stringer("eia", selected.Integrity))

	return true
}

func provideEPSNASAlgorithms(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, ueConn *amf.UeConn) bool {
	intOrder, encOrder, err := amfInstance.SecurityAlgorithms(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not read the operator security policy, not signalling EPS NAS algorithms", zap.Error(err))
		return false
	}

	if !selectEPSNASAlgorithms(ctx, ue, intOrder, encOrder) {
		return false
	}

	if !ue.BeginKeyChainProc(procedure.SecurityMode) {
		logger.From(ctx, logger.AmfLog).Warn("EPS NAS algorithm delivery blocked by a conflicting key-changing procedure")
		return false
	}

	committed := false

	defer func() {
		if !committed {
			ue.EndKeyChainProc(procedure.SecurityMode)
		}
	}()

	if err := amf.SendEPSNASAlgorithmsSecurityModeCommand(ctx, amfInstance, ueConn); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not send the security mode command carrying the EPS NAS algorithms", zap.Error(err))
		return false
	}

	committed = true

	return true
}
