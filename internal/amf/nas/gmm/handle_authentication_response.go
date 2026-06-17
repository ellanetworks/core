// Copyright 2026 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package gmm

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// TS 24.501 5.4.1
func handleAuthenticationResponse(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe, msg *nasMessage.AuthenticationResponse) error {
	if state := ue.GetState(); state != amf.Authentication {
		return fmt.Errorf("state mismatch: receive Authentication Response message in state %s", state)
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	if conn.T3560 != nil {
		conn.T3560.Stop()
		conn.T3560 = nil
	}

	if conn.AuthenticationCtx == nil {
		return fmt.Errorf("ue Authentication Context is nil")
	}

	if msg.AuthenticationResponseParameter == nil {
		// No RES* to verify: treat as an unsuccessful authentication
		// (TS 24.501 §5.4.1.3.5).
		ue.Log.Error("Authentication Response missing RES* (Authentication response parameter IE)")

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

	ue.Supi = supi

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
func failAuthentication(ctx context.Context, ue *amf.AmfUe, ranUe *amf.RanUe, conn *amf.ActiveNasConnection) error {
	if conn.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
		if err := message.SendIdentityRequest(ctx, ranUe, nasMessage.MobileIdentity5GSTypeSuci); err != nil {
			return fmt.Errorf("send identity request error: %s", err)
		}

		ue.Log.Info("sent identity request")

		return nil
	}

	defer ue.Deregister(ctx)

	if err := message.SendAuthenticationReject(ctx, ranUe); err != nil {
		return fmt.Errorf("error sending GMM authentication reject: %v", err)
	}

	return nil
}
