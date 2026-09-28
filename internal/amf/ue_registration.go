// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

var causeReleaseDueToCNDetectedMobility = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkReleaseDueToCNDetectedMobility}

type Registrar interface {
	Register(ctx context.Context, imsi string) (int64, error)
	Confirmed(ctx context.Context, imsi string, version int64) bool
	Purge(imsi string)
	Reconcile(ctx context.Context, imsi string, held func() int64, release func(context.Context) bool)
}

func (amf *AMF) RegisterUE(ctx context.Context, ue *UeContext) error {
	supi := ue.Supi()
	if amf.Registrations == nil || !supi.IsIMSI() {
		return nil
	}

	version, err := amf.Registrations.Register(ctx, supi.IMSI())
	if err != nil {
		return err
	}

	ue.registrationVersion.Store(version)

	return nil
}

func (amf *AMF) ConfirmRegistration(ctx context.Context, ue *UeContext) error {
	supi := ue.Supi()
	if amf.Registrations == nil || !supi.IsIMSI() {
		return nil
	}

	if amf.Registrations.Confirmed(ctx, supi.IMSI(), ue.registrationVersion.Load()) {
		return nil
	}

	return amf.RegisterUE(ctx, ue)
}

func (amf *AMF) purgeRegistration(supi etsi.SUPI) {
	if amf.Registrations == nil || !supi.IsIMSI() {
		return
	}

	amf.Registrations.Purge(supi.IMSI())
}

func (amf *AMF) HoldsUE(imsi string) bool {
	_, ok := amf.lookupUeByIMSI(imsi)
	return ok
}

func (amf *AMF) lookupUeByIMSI(imsi string) (*UeContext, bool) {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return nil, false
	}

	return amf.LookupUeBySupi(supi)
}

func (amf *AMF) ReconcileRegistration(ctx context.Context, imsi string) {
	if amf.Registrations == nil {
		return
	}

	ue, ok := amf.lookupUeByIMSI(imsi)
	if !ok || ue.State() != Registered {
		return
	}

	amf.Registrations.Reconcile(ctx, imsi, ue.registrationVersion.Load, func(ctx context.Context) bool {
		return amf.releaseSuperseded(ctx, ue)
	})
}

func (amf *AMF) ReconcileRegistrations(ctx context.Context) {
	for _, ue := range amf.registeredUEs() {
		if supi := ue.Supi(); supi.IsIMSI() {
			amf.ReconcileRegistration(ctx, supi.IMSI())
		}
	}
}

func (amf *AMF) releaseSuperseded(ctx context.Context, ue *UeContext) bool {
	supi := ue.Supi()

	if !amf.ServesUeContext(ue) || ue.State() != Registered {
		return true
	}

	if amf.HandoverInProgress(ue) || amf.HandoverToEPSInProgress(ue) || amf.RelocationFromEPSInProgress(supi) {
		return false
	}

	ueConn := ue.Conn()
	if ueConn != nil && !ueConn.ReleaseWithAction(ctx, UeContextReleaseDueToNwInitiatedDeregistraion, causeReleaseDueToCNDetectedMobility) {
		return false
	}

	logger.From(ctx, logger.AmfLog).Info("UE registered on another node; dropping its local 5GS registration and PDU sessions",
		logger.SUPI(supi.String()))

	if ueConn == nil {
		amf.DeregisterAndRemoveUeContext(ctx, ue)
		return true
	}

	ue.Deregister(ctx)

	return true
}
