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

	if ue.SecurityContextIsValid() {
		// TS 24.501 §5.4.2.2: a UE that supports S1 mode and has not yet been given its
		// EPS NAS algorithms gets a security mode command carrying them, even though the
		// 5G NAS security context needs no change. Registration continues from the
		// SECURITY MODE COMPLETE as it does for any other security mode procedure.
		if amfInstance.N26Enabled && ue.NeedsEPSNASAlgorithms() && provideEPSNASAlgorithms(ctx, amfInstance, ue, conn) {
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

	selectEPSNASAlgorithms(ctx, amfInstance, ue, integrityOrder, cipheringOrder)

	if err := amf.SendSecurityModeCommand(ctx, amfInstance, ueConn); err != nil {
		abortSecurityMode(ctx, ue, ueConn, "send security mode command", err)
		return
	}

	committed = true
}

// selectEPSNASAlgorithms offers the EPS NAS algorithms this UE is to use after
// mobility to EPS, so the SECURITY MODE COMMAND about to be sent carries them
// (TS 24.501 §5.4.2.2, TS 33.501 §6.7.2). The orders are the operator's, the same
// ones the MME applies; only the capability they are matched against is
// EPS-specific. It reports whether anything is on offer.
//
// A UE registering for the first time has not yet disclosed its EPS capability —
// it is not a cleartext IE — so nothing is offered here, and the algorithms are
// delivered by their own security mode procedure at its next registration. Failing
// to select costs that UE N26 mobility and nothing else, so it never fails the
// registration.
func selectEPSNASAlgorithms(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, intOrder []nas.IntegrityAlgorithm, encOrder []nas.CipheringAlgorithm) bool {
	if !amfInstance.N26Enabled || !ue.NeedsEPSNASAlgorithms() {
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

// provideEPSNASAlgorithms runs the security mode control procedure whose only
// purpose is to deliver the EPS NAS algorithms to a UE whose 5G NAS security
// context is already current (TS 24.501 §5.4.2.2). It reports whether the procedure
// is in flight; when it is not, the caller carries on with the registration as
// though nothing had been owed.
func provideEPSNASAlgorithms(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, ueConn *amf.UeConn) bool {
	// Settle what there is to say before claiming anything, so a UE with nothing to
	// be told does not hold the key chain for the length of a database read.
	intOrder, encOrder, err := amfInstance.SecurityAlgorithms(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not read the operator security policy, not signalling EPS NAS algorithms", zap.Error(err))
		return false
	}

	if !selectEPSNASAlgorithms(ctx, amfInstance, ue, intOrder, encOrder) {
		return false
	}

	// The message re-signals the current NAS algorithms, so it must not race a
	// procedure that is changing them (TS 33.501 §6.9.5.1).
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
