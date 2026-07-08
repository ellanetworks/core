// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func handleAuthenticationFailure(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.AuthenticationFailure) {
	if step := ue.RegStep(); step != amf.RegStepAuthenticating {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Authentication Failure message outside the authentication exchange", zap.String("state", string(ue.State())))
		return
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
		return
	}

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return
	}

	// An AUTHENTICATION FAILURE is only valid while a challenge is in flight; in the
	// identity sub-window of RegStepAuthenticating no challenge has been sent, so an
	// out-of-order one (admissible without integrity, TS 24.501 §4.4.4.3) must not
	// release the UE.
	if conn.AuthenticationCtx == nil {
		logger.From(ctx, logger.AmfLog).Warn("ignoring Authentication Failure with no authentication in progress")
		return
	}

	// A cause outside the AUTHENTICATION FAILURE enumeration is a semantically
	// incorrect message: ignore it and leave the authentication procedure and its
	// guard (T3560) running so it retransmits or times out (TS 24.501 §7.8). Stop the
	// guard only for an enumerated cause.
	switch msg.GetCauseValue() {
	case nasMessage.Cause5GMMMACFailure,
		nasMessage.Cause5GMMNon5GAuthenticationUnacceptable,
		nasMessage.Cause5GMMngKSIAlreadyInUse,
		nasMessage.Cause5GMMSynchFailure:
		conn.StopNASGuard()
	default:
		logger.From(ctx, logger.AmfLog).Warn("ignoring Authentication Failure with an out-of-enumeration cause",
			zap.Uint8("cause", msg.GetCauseValue()))

		return
	}

	switch msg.GetCauseValue() {
	case nasMessage.Cause5GMMMACFailure:
		logger.From(ctx, logger.AmfLog).Warn("amf.Authentication Failure Cause: Mac Failure")
		ue.Deregister(ctx)

		amf.SendAuthenticationReject(ctx, ueConn)

		return
	case nasMessage.Cause5GMMNon5GAuthenticationUnacceptable:
		logger.From(ctx, logger.AmfLog).Warn("amf.Authentication Failure Cause: Non-5G amf.Authentication Unacceptable")
		ue.Deregister(ctx)

		amf.SendAuthenticationReject(ctx, ueConn)

		return
	case nasMessage.Cause5GMMngKSIAlreadyInUse:
		logger.From(ctx, logger.AmfLog).Warn("amf.Authentication Failure Cause: NgKSI Already In Use")

		conn.SetResyncTried(false)

		logger.From(ctx, logger.AmfLog).Warn("Select new NgKsi")

		ngKsi := ue.NgKsi()
		ngKsi.Ksi = amf.NextNgKsi(ngKsi.Ksi)
		ue.SetNgKsi(ngKsi)

		amf.SendAuthenticationRequest(ctx, amfInstance, ueConn)

		logger.From(ctx, logger.AmfLog).Info("Sent authentication request")
	case nasMessage.Cause5GMMSynchFailure: // TS 24.501
		logger.From(ctx, logger.AmfLog).Warn("amf.Authentication Failure 5GMM Cause: Synch Failure")

		if conn.ResyncTried() {
			logger.From(ctx, logger.AmfLog).Warn("2 consecutive Synch Failure, terminate authentication procedure")
			ue.Deregister(ctx)

			amf.SendAuthenticationReject(ctx, ueConn)

			return
		}

		if msg.AuthenticationFailureParameter == nil {
			logger.From(ctx, logger.AmfLog).Warn("missing AuthenticationFailureParameter IE for SynchFailure")
			return
		}

		conn.SetResyncTried(true)

		auts := msg.GetAuthenticationFailureParameter()
		resynchronizationInfo := &ausf.ResyncInfo{
			Auts: hex.EncodeToString(auts[:]),
		}

		response, err := sendUEAuthenticationAuthenticateRequest(ctx, amfInstance, ue, resynchronizationInfo)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("send UE amf.Authentication Authenticate Request Error", zap.Error(err))
			return
		}

		conn.AuthenticationCtx = response

		ue.SetAbba([]uint8{0x00, 0x00})

		amf.SendAuthenticationRequest(ctx, amfInstance, ueConn)

		logger.From(ctx, logger.AmfLog).Info("Sent authentication request")
	}
}
