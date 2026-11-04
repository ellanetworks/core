// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/ueregistration"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

var causeReleaseDueToCNDetectedMobility = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkReleaseDueToCNDetectedMobility}

type Registrar interface {
	Register(ctx context.Context, imsi string) (int64, error)
	RegisterVisitor(ctx context.Context, imsi, partnerID string) (int64, error)
	Confirmed(ctx context.Context, imsi string, version int64) bool
	Purge(imsi string)
	Reconcile(ctx context.Context, imsi string, held func() int64, release func(context.Context), withdraw func(context.Context, ueregistration.Withdrawal))
}

func (amf *AMF) RegisterUE(ctx context.Context, ue *UeContext) error {
	supi := ue.Supi()
	if amf.Registrations == nil || !supi.IsIMSI() {
		return nil
	}

	var (
		version int64
		err     error
	)

	if v, ok := ue.Visitor(); ok {
		version, err = amf.Registrations.RegisterVisitor(ctx, supi.IMSI(), v.PartnerID)
	} else {
		version, err = amf.Registrations.Register(ctx, supi.IMSI())
	}

	if errors.Is(err, db.ErrRoamingRefused) {
		return reject(fgs.GMMCausePLMNNotAllowed, "the roaming agreement is being withdrawn")
	}

	if errors.Is(err, db.ErrSubscriberLimit) {
		return &Rejection{Cause: fgs.GMMCauseCongestion, BackOff: T3346CapacityBackoff, Reason: "maximum number of subscribers reached"}
	}

	if err != nil {
		return err
	}

	ue.registrationVersion.Store(version)

	amf.syncVisitorMSISDN(ctx, ue)

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

	amf.Registrations.Reconcile(ctx, imsi, ue.registrationVersion.Load, func(ctx context.Context) {
		amf.releaseSuperseded(ctx, ue)
	}, func(ctx context.Context, why ueregistration.Withdrawal) {
		if why == ueregistration.SubscriberDeleted {
			amf.withdrawSubscription(ctx, ue)
			return
		}

		amf.withdrawRoamer(ctx, ue, why)
	})
}

func (amf *AMF) ReconcileRegistrations(ctx context.Context) {
	amf.renewHPLMNSubscriptions(ctx)

	for _, ue := range amf.registeredUEs() {
		if supi := ue.Supi(); supi.IsIMSI() {
			amf.ReconcileRegistration(ctx, supi.IMSI())
		}
	}
}

func (amf *AMF) releaseSuperseded(ctx context.Context, ue *UeContext) {
	supi := ue.Supi()

	if !amf.ServesUeContext(ue) || ue.State() != Registered {
		return
	}

	if amf.HandoverInProgress(ue) || amf.HandoverToEPSInProgress(ue) || amf.RelocationFromEPSInProgress(supi) {
		return
	}

	ueConn := ue.Conn()
	if ueConn != nil && !ueConn.ReleaseWithAction(ctx, UeContextReleaseDueToNwInitiatedDeregistraion, causeReleaseDueToCNDetectedMobility) {
		return
	}

	ue.updateVisitor(func(v *roaming.Visitor5G) { v.Registered = false })

	logger.From(ctx, logger.AmfLog).Info("UE registered on another node; dropping its local 5GS registration and PDU sessions",
		logger.SUPI(supi.String()))

	if ueConn == nil {
		amf.DeregisterAndRemoveUeContext(ctx, ue, models.ReleaseFor(models.ReleaseDeregistration))
		return
	}

	ue.Deregister(ctx, models.ReleaseFor(models.ReleaseDeregistration))
}

func (amf *AMF) withdrawSubscription(ctx context.Context, ue *UeContext) {
	if !amf.ServesUeContext(ue) {
		return
	}

	supi := ue.Supi()

	logger.From(ctx, logger.AmfLog).Info("subscriber deleted; deregistering UE", logger.SUPI(supi.String()))

	amf.DeregisterSubscriber(ctx, supi)
	amf.ForgetSubscriber(supi.IMSI())
}
