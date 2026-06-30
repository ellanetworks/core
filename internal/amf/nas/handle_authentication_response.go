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

// TS 24.501 5.4.1
func handleAuthenticationResponse(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.AuthenticationResponse) error {
	if state := ue.GetState(); state != amf.Authentication {
		return fmt.Errorf("state mismatch: receive amf.Authentication Response message in state %s", state)
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
		// No RES* to verify: treat as an unsuccessful authentication
		// (TS 24.501 §5.4.1.3.5).
		ue.Log.Error("amf.Authentication Response missing RES* (amf.Authentication response parameter IE)")

		return failAuthentication(ctx, ue, ranUe, conn)
	}

	resStar := msg.GetRES()

	// Calculate HRES* (TS 33.501 Annex A.5)
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

		return failAuthentication(ctx, ue, ranUe, conn)
	}

	supi, kseaf, err := amfInstance.Ausf.Confirm(ctx, hex.EncodeToString(resStar[:]), ue.Suci)
	if err != nil {
		logger.WithTrace(ctx, logger.AmfLog).Error("5G AKA Confirmation Request Procedure failed", zap.Error(err))

		return failAuthentication(ctx, ue, ranUe, conn)
	}

	ue.SetSupi(supi)

	err = ue.DerivateKamf(kseaf)
	if err != nil {
		return fmt.Errorf("couldn't derive Kamf: %v", err)
	}

	return securityMode(ctx, amfInstance, ue)
}

// failAuthentication applies TS 24.501 §5.4.1.3.5 when an authentication
// response cannot be accepted (RES* absent, HRES* mismatch, or AUSF rejection):
// if the UE was identified by 5G-GUTI, the network retrieves the SUCI via an
// identification procedure and restarts authentication; otherwise it rejects
// authentication and deregisters the UE.
func failAuthentication(ctx context.Context, ue *amf.UeContext, ranUe *amf.RanUe, conn *amf.ActiveNasConnection) error {
	if conn.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
		amf.SendIdentityRequest(ctx, ranUe, nasMessage.MobileIdentity5GSTypeSuci)

		ue.Log.Info("sent identity request")

		return nil
	}

	defer ue.Deregister(ctx)

	amf.SendAuthenticationReject(ctx, ranUe)

	return nil
}
