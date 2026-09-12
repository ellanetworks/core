// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

const deferredReleaseTimeout = 30 * time.Second

func (ueConn *UeConn) MTSignallingPending() bool {
	if ueConn == nil {
		return false
	}

	if ueConn.Parent().MTDeliveryInProgress() {
		return true
	}

	if ueConn.N2SetupOpen(N2SetupInitialContext) || ueConn.N2SetupOpen(N2SetupPDUSession) {
		return true
	}

	return ueConn.NASGuardActive()
}

func (ueConn *UeConn) DeferRelease(cause ngap.Cause) {
	if ueConn == nil {
		return
	}

	held := cause
	if !ueConn.deferredCause.CompareAndSwap(nil, &held) {
		return
	}

	ueConn.deferGuard.ArmOnce(deferredReleaseTimeout, func() {
		logger.From(context.Background(), ueConn.Log()).Warn("deferred UE Context Release deadline reached; releasing the NG connection",
			zap.String("cause", cause.String()))

		ueConn.resumeDeferredRelease(context.Background())
	})
}

func (ueConn *UeConn) ResumeDeferredReleaseIfSettled() {
	if ueConn == nil || ueConn.deferredCause.Load() == nil {
		return
	}

	if ueConn.MTSignallingPending() {
		return
	}

	ueConn.resumeDeferredRelease(context.Background())
}

func (ueConn *UeConn) resumeDeferredRelease(ctx context.Context) {
	cause := ueConn.deferredCause.Swap(nil)
	if cause == nil {
		return
	}

	ueConn.deferGuard.Stop()

	logger.From(ctx, ueConn.Log()).Info("resuming the deferred UE Context Release: the pending downlink traffic or signalling has settled")

	a := ueConn.amf
	if a == nil {
		return
	}

	a.ReleaseOnRANRequest(ctx, ueConn, *cause, nil)
}

func (ueConn *UeConn) cancelDeferredRelease() {
	if ueConn == nil {
		return
	}

	ueConn.deferredCause.Store(nil)
	ueConn.deferGuard.Stop()
}
