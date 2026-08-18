// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
)

func (a *AMF) releaseToEPS(ctx context.Context, ue *UeContext) {
	ue.RetainForEPS(epsContextRetention)
	ue.Deregister(ctx)
	a.StartMobileReachable(ue)
}

func (a *AMF) CancelRegistration(ctx context.Context, supi etsi.SUPI) {
	ue, ok := a.LookupUeBySupi(supi)
	if !ok || ue.State() != Registered {
		return
	}

	if a.HandoverToEPSInProgress(ue) || a.RelocationFromEPSInProgress(supi) {
		logger.From(ctx, logger.AmfLog).Debug("leaving the 5GS registration to the interworking procedure that already owns it",
			logger.SUPI(supi.String()))

		return
	}

	a.releaseToEPS(ctx, ue)

	logger.From(ctx, logger.AmfLog).Info("UE attached in EPS; dropping its 5GS registration and keeping its 5G security context for a return",
		logger.SUPI(supi.String()))
}

func (a *AMF) RelocationFromEPSInProgress(supi etsi.SUPI) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	_, busy := a.relocatingFromEPS[supi]

	return busy
}

func (a *AMF) SupersedeEPSRegistration(ctx context.Context, ue *UeContext) {
	if a.EPS == nil || ue == nil {
		return
	}

	supi := ue.Supi()
	if !supi.IsValid() {
		return
	}

	if a.RelocationFromEPSInProgress(supi) {
		return
	}

	a.EPS.CancelRegistration(ctx, supi)
}
