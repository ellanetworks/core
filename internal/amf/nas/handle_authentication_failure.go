// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func handleAuthenticationFailure(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, plain []byte) nasreply.Disposition {
	if step := ue.RegStep(); step != amf.RegStepAuthenticating {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Authentication Failure message outside the authentication exchange", zap.String("state", string(ue.State())))
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
		return nasreply.Handled()
	}

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return nasreply.Handled()
	}

	// An AUTHENTICATION FAILURE is only valid while a challenge is in flight; in the
	// identity sub-window of RegStepAuthenticating no challenge has been sent, so an
	// out-of-order one (admissible without integrity, TS 24.501 §4.4.4.3) must not
	// release the UE.
	if conn.AuthenticationCtx == nil {
		logger.From(ctx, logger.AmfLog).Warn("ignoring Authentication Failure with no authentication in progress")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	msg, err := fgs.ParseAuthenticationFailure(plain)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not decode Authentication Failure", zap.Error(err))
		return nasreply.Handled()
	}

	// A cause outside the AUTHENTICATION FAILURE enumeration is a semantically
	// incorrect message: ignore it and leave the authentication procedure and its
	// guard (T3560) running so it retransmits or times out (TS 24.501 §7.8). Stop the
	// guard only for an enumerated cause.
	switch msg.Cause {
	case amf.GmmCauseMACFailure,
		amf.GmmCauseNon5GAuthUnacceptable,
		amf.GmmCauseNgKSIAlreadyInUse,
		amf.GmmCauseSynchFailure:
		conn.StopNASGuard()
	default:
		logger.From(ctx, logger.AmfLog).Warn("ignoring Authentication Failure with an out-of-enumeration cause",
			zap.Uint8("cause", msg.Cause))

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	switch msg.Cause {
	case amf.GmmCauseMACFailure:
		logger.From(ctx, logger.AmfLog).Warn("amf.Authentication Failure Cause: Mac Failure")
		ue.Deregister(ctx)

		amf.SendAuthenticationReject(ctx, ueConn)

		return nasreply.Handled()
	case amf.GmmCauseNon5GAuthUnacceptable:
		logger.From(ctx, logger.AmfLog).Warn("amf.Authentication Failure Cause: Non-5G amf.Authentication Unacceptable")
		ue.Deregister(ctx)

		amf.SendAuthenticationReject(ctx, ueConn)

		return nasreply.Handled()
	case amf.GmmCauseNgKSIAlreadyInUse:
		logger.From(ctx, logger.AmfLog).Warn("amf.Authentication Failure Cause: NgKSI Already In Use")

		conn.SetResyncTried(false)

		logger.From(ctx, logger.AmfLog).Warn("Select new NgKsi")

		ngKsi := ue.NgKsi()
		ngKsi.Ksi = amf.NextNgKsi(ngKsi.Ksi)
		ue.SetNgKsi(ngKsi)

		amf.SendAuthenticationRequest(ctx, amfInstance, ueConn)

		logger.From(ctx, logger.AmfLog).Info("Sent authentication request")
	case amf.GmmCauseSynchFailure: // TS 24.501
		logger.From(ctx, logger.AmfLog).Warn("amf.Authentication Failure 5GMM Cause: Synch Failure")

		if conn.ResyncTried() {
			logger.From(ctx, logger.AmfLog).Warn("2 consecutive Synch Failure, terminate authentication procedure")
			ue.Deregister(ctx)

			amf.SendAuthenticationReject(ctx, ueConn)

			return nasreply.Handled()
		}

		if msg.AUTS == nil {
			logger.From(ctx, logger.AmfLog).Warn("missing AuthenticationFailureParameter IE for SynchFailure")
			return nasreply.Handled()
		}

		conn.SetResyncTried(true)

		resynchronizationInfo := &ausf.ResyncInfo{
			Auts: hex.EncodeToString(msg.AUTS),
		}

		response, err := sendUEAuthenticationAuthenticateRequest(ctx, amfInstance, ue, resynchronizationInfo)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("send UE amf.Authentication Authenticate Request Error", zap.Error(err))
			return nasreply.Handled()
		}

		conn.AuthenticationCtx = response

		ue.SetAbba([]uint8{0x00, 0x00})

		amf.SendAuthenticationRequest(ctx, amfInstance, ueConn)

		logger.From(ctx, logger.AmfLog).Info("Sent authentication request")
	}

	return nasreply.Handled()
}
