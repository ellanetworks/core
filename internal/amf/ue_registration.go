// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

type Registrar interface {
	Register(ctx context.Context, imsi string) error
	Purge(ctx context.Context, imsi string, stillAbsent func() bool)
	Superseded(ctx context.Context, imsi string, registeredAt int64) bool
}

func (amf *AMF) RegisterUE(ctx context.Context, ue *UeContext) error {
	supi := ue.Supi()
	if amf.Registrations == nil || !supi.IsIMSI() {
		return nil
	}

	start := time.Now().UnixMilli()

	if err := amf.Registrations.Register(ctx, supi.IMSI()); err != nil {
		return err
	}

	ue.registeredAt.Store(start)

	return nil
}

func (amf *AMF) purgeRegistrationAsync(supi etsi.SUPI) {
	if amf.Registrations == nil || !supi.IsIMSI() {
		return
	}

	go amf.Registrations.Purge(context.Background(), supi.IMSI(), func() bool {
		_, ok := amf.LookupUeBySupi(supi)
		return !ok
	})
}

func (amf *AMF) ReconcileRegistrations(ctx context.Context) {
	if amf.Registrations == nil {
		return
	}

	for _, ue := range amf.registeredUEs() {
		supi := ue.Supi()
		if !supi.IsIMSI() || !amf.Registrations.Superseded(ctx, supi.IMSI(), ue.registeredAt.Load()) {
			continue
		}

		amf.releaseSuperseded(ctx, ue)
	}
}

func (amf *AMF) releaseSuperseded(ctx context.Context, ue *UeContext) {
	supi := ue.Supi()

	if !amf.ServesUeContext(ue) || ue.State() != Registered {
		return
	}

	if amf.HandoverToEPSInProgress(ue) || amf.RelocationFromEPSInProgress(supi) {
		return
	}

	logger.From(ctx, logger.AmfLog).Info("UE registered on another node; dropping its local 5GS registration and PDU sessions",
		logger.SUPI(supi.String()))

	ueConn := ue.Conn()
	if ueConn == nil {
		amf.DeregisterAndRemoveUeContext(ctx, ue)
		return
	}

	ueConn.ReleaseAction = UeContextReleaseDueToNwInitiatedDeregistraion
	ueConn.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkReleaseDueTo5GCGeneratedReason})

	ue.Deregister(ctx)
}
