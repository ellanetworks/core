// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// TS 24.501
func handleAuthenticationResponse(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, resp *fgs.AuthenticationResponse) nasreply.Disposition {
	if step := ue.RegStep(); step != amf.RegStepAuthenticating {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Authentication Response message outside the authentication exchange", zap.String("state", string(ue.State())))
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

	conn.StopNASGuard(ctx)

	if conn.AuthenticationCtx == nil {
		logger.From(ctx, logger.AmfLog).Warn("ue amf.Authentication Context is nil")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	if resp.RES == nil {
		// No RES* to verify: unsuccessful authentication (TS 24.501).
		logger.From(ctx, logger.AmfLog).Error("amf.Authentication Response missing RES* (amf.Authentication response parameter IE)")

		amfInstance.AuthenticationFailed(ctx, ue)

		failAuthentication(ctx, ue, ueConn)

		return nasreply.Handled()
	}

	resStar := resp.RES

	// HRES* derivation (TS 33.501)
	p0, err := hex.DecodeString(conn.AuthenticationCtx.Rand)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to decode RAND", zap.Error(err))
		return nasreply.Handled()
	}

	p1 := resStar[:]
	concat := append(p0, p1...)
	hResStarBytes := sha256.Sum256(concat)
	hResStar := hex.EncodeToString(hResStarBytes[16:])

	if subtle.ConstantTimeCompare([]byte(hResStar), []byte(conn.AuthenticationCtx.HxresStar)) != 1 {
		logger.From(ctx, logger.AmfLog).Error("HRES* Validation Failure")

		amfInstance.AuthenticationFailed(ctx, ue)

		failAuthentication(ctx, ue, ueConn)

		return nasreply.Handled()
	}

	amfInstance.RequestConfirmation(ctx, ue, ueConn, [16]byte(resStar[:]), func(ctx context.Context, supi etsi.SUPI, kseaf []byte, err error) {
		completeAuthentication(ctx, amfInstance, ue, ueConn, supi, kseaf, err)
	})

	return nasreply.Handled()
}

func completeAuthentication(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, ueConn *amf.UeConn, supi etsi.SUPI, kseaf []byte, err error) {
	if ue.RegStep() != amf.RegStepAuthenticating {
		logger.From(ctx, logger.AmfLog).Info("registration moved on while waiting for the authentication confirmation; dropping it")
		return
	}

	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("5G AKA Confirmation Request Procedure failed", zap.Error(err))

		if ue.IsVisitor() {
			if r, ok := amf.ConfirmationRejection(err); ok {
				rejectRoaming(ctx, amfInstance, ue, r)
				return
			}

			abortRegistration(ctx, amfInstance, ue, "home authentication confirmation", err)

			return
		}

		failAuthentication(ctx, ue, ueConn)

		return
	}

	ue.SetSupi(supi)

	if err := ue.DeriveKamf(kseaf, ueConn.AuthNgKsi); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("couldn't derive Kamf", zap.Error(err))

		failAuthentication(ctx, ue, ueConn)

		return
	}

	if isRegistrationUpdate(ueConn.RegistrationType5GS) {
		amfInstance.CarrySubscriberSessions(ue)
	} else {
		ueConn.RegisteredBeforeRegistration = false
	}

	if err := amfInstance.CommitUEIdentity(ctx, ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		logger.From(ctx, logger.AmfLog).Error("could not adopt the authenticated SUPI", zap.Error(err))

		failAuthentication(ctx, ue, ueConn)

		return
	}

	securityMode(ctx, amfInstance, ue)
}

// failAuthentication rejects the authentication response and deregisters the UE
// (TS 24.501). Authentication runs on the UE-provided SUCI, so a RES* failure is a
// genuine credential failure with no stale GUTI→identity mapping to recover by
// re-identifying.
func failAuthentication(ctx context.Context, ue *amf.UeContext, ueConn *amf.UeConn) {
	defer ue.Deregister(ctx, models.ReleaseFor(models.ReleaseRegistrationFailure))

	amf.SendAuthenticationReject(ctx, ueConn)
}
