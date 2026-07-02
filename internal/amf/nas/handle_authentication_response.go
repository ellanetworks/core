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
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// TS 24.501
func handleAuthenticationResponse(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.AuthenticationResponse) error {
	if step := ue.RegStep(); step != amf.RegStepAuthenticating {
		return fmt.Errorf("state mismatch: receive Authentication Response message outside the authentication exchange (state %s)", ue.State())
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	conn.T3560.Stop()

	if conn.AuthenticationCtx == nil {
		return fmt.Errorf("ue amf.Authentication Context is nil")
	}

	if msg.AuthenticationResponseParameter == nil {
		// No RES* to verify: unsuccessful authentication (TS 24.501).
		ue.Log.Error("amf.Authentication Response missing RES* (amf.Authentication response parameter IE)")

		return failAuthentication(ctx, ue, ranUe)
	}

	resStar := msg.GetRES()

	// Calculate HRES* (TS 33.501)
	p0, err := hex.DecodeString(conn.AuthenticationCtx.Rand)
	if err != nil {
		return fmt.Errorf("failed to decode RAND: %s", err)
	}

	p1 := resStar[:]
	concat := append(p0, p1...)
	hResStarBytes := sha256.Sum256(concat)
	hResStar := hex.EncodeToString(hResStarBytes[16:])

	if subtle.ConstantTimeCompare([]byte(hResStar), []byte(conn.AuthenticationCtx.HxresStar)) != 1 {
		ue.Log.Error("HRES* Validation Failure")

		return failAuthentication(ctx, ue, ranUe)
	}

	supi, kseaf, err := amfInstance.Ausf.Confirm(ctx, hex.EncodeToString(resStar[:]), ue.Suci)
	if err != nil {
		logger.WithTrace(ctx, logger.AmfLog).Error("5G AKA Confirmation Request Procedure failed", zap.Error(err))

		return failAuthentication(ctx, ue, ranUe)
	}

	ue.SetSupi(supi)

	err = ue.DerivateKamf(kseaf)
	if err != nil {
		return fmt.Errorf("couldn't derive Kamf: %v", err)
	}

	return securityMode(ctx, amfInstance, ue)
}

// failAuthentication rejects an unacceptable authentication response (RES* absent,
// HRES* mismatch, or AUSF rejection) and deregisters the UE (TS 24.501). The AMF
// authenticates on the UE-provided SUCI (identify-first: an untrusted 5G-GUTI is
// re-identified via SUCI before authentication, a trusted one reuses its verified
// context and skips it), so a RES* failure is a genuine credential failure — there
// is no stale GUTI→identity mapping to recover by re-identifying. This mirrors the
// MME, which likewise rejects on authentication failure having identified first.
func failAuthentication(ctx context.Context, ue *amf.UeContext, ranUe *amf.RanUe) error {
	defer ue.Deregister(ctx)

	amf.SendAuthenticationReject(ctx, ranUe)

	return nil
}
