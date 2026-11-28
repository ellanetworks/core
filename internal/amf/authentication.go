// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/ausf"
)

type heldAuthentication struct {
	ausf      Authenticator
	authCtxID string
}

func (h heldAuthentication) reportFailure(ctx context.Context) {
	if h.ausf == nil || h.authCtxID == "" {
		return
	}

	h.ausf.ReportAuthenticationFailure(ctx, h.authCtxID)
}

func (amf *AMF) AuthenticateAtAUSF(ctx context.Context, ue *UeContext, conn *UeConn, resync *ausf.ResynchronizationInfo) (*ausf.UEAuthenticationCtx, error) {
	if ue.Tai.PlmnID == nil {
		return nil, fmt.Errorf("tai is not available in UE context")
	}

	conn.releaseAuthentication(ctx)

	authCtx, err := amf.Ausf.Authenticate(ctx, ue.AuthenticationIdentity(conn), *ue.Tai.PlmnID, resync)
	if err != nil {
		return nil, err
	}

	conn.holdAuthentication(ctx, heldAuthentication{ausf: amf.Ausf, authCtxID: authCtx.AuthCtxID})

	return authCtx, nil
}

func (ueConn *UeConn) holdAuthentication(ctx context.Context, h heldAuthentication) {
	ueConn.heldAuthMu.Lock()
	previous := ueConn.heldAuth
	ueConn.heldAuth = h
	ueConn.heldAuthMu.Unlock()

	if previous.authCtxID != h.authCtxID {
		previous.reportFailure(ctx)
	}
}

func (ueConn *UeConn) claimAuthentication(authCtxID string) (heldAuthentication, bool) {
	ueConn.heldAuthMu.Lock()
	defer ueConn.heldAuthMu.Unlock()

	if authCtxID == "" || ueConn.heldAuth.authCtxID != authCtxID {
		return heldAuthentication{}, false
	}

	h := ueConn.heldAuth
	ueConn.heldAuth = heldAuthentication{}

	return h, true
}

func (ueConn *UeConn) releaseAuthentication(ctx context.Context) {
	if ueConn == nil {
		return
	}

	ueConn.heldAuthMu.Lock()
	h := ueConn.heldAuth
	ueConn.heldAuth = heldAuthentication{}
	ueConn.heldAuthMu.Unlock()

	h.reportFailure(ctx)
}

func (ueConn *UeConn) DiscardAuthentication(ctx context.Context, authCtx *ausf.UEAuthenticationCtx) {
	if ueConn == nil || authCtx == nil {
		return
	}

	if h, ok := ueConn.claimAuthentication(authCtx.AuthCtxID); ok {
		h.reportFailure(ctx)
	}
}
