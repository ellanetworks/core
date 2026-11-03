// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/ueregistration"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

const (
	sdmLifetime    = 24 * time.Hour
	sdmRenewBefore = 5 * time.Minute
)

type HomeRenewer interface {
	Renew(ctx context.Context, t roaming.HomeTarget, uri string, expires wire.DateTime) (wire.DateTime, error)
}

func (amf *AMF) withdrawRoamer(ctx context.Context, ue *UeContext, why ueregistration.Withdrawal) {
	if !amf.ServesUeContext(ue) || !ue.IsVisitor() {
		return
	}

	if why == ueregistration.RoamerDeregistered {
		logger.From(ctx, logger.AmfLog).Info("the operator deregistered the visitor", logger.SUPI(ue.Supi().String()))
		amf.DeregisterUE(ctx, ue, models.ReleaseFor(models.ReleaseDeregistration), NetworkDeregistration{})

		return
	}

	cause := fgs.GMMCausePLMNNotAllowed
	if why == ueregistration.RoamingAgreementWithdrawn {
		cause = fgs.GMMCauseN1ModeNotAllowed
	}

	logger.From(ctx, logger.AmfLog).Info("roaming agreement withdrawn; deregistering the visitor",
		logger.SUPI(ue.Supi().String()), logger.Cause(cause.String()))

	amf.DeregisterUE(ctx, ue, models.ReleaseFor(models.ReleaseSubscriptionWithdrawn), NetworkDeregistration{Cause: &cause})
}

func (amf *AMF) heldVisitor(partnerID, imsi string) (*UeContext, roaming.Visitor5G, bool) {
	ue, ok := amf.lookupUeByIMSI(imsi)
	if !ok {
		return nil, roaming.Visitor5G{}, false
	}

	v, ok := ue.Visitor()
	if !ok || v.PartnerID != partnerID || !v.Registered {
		return nil, roaming.Visitor5G{}, false
	}

	return ue, v, true
}

func (amf *AMF) HomeDeregistration(ctx context.Context, partnerID, imsi string, data wire.DeregistrationData) error {
	if data.DeregReason == wire.Dereg5GSToEPSMobility {
		if handled, err := amf.homeMovedToEPS(ctx, partnerID, imsi, data); handled {
			return err
		}
	}

	ue, _, ok := amf.heldVisitor(partnerID, imsi)
	if !ok {
		return roaming.ErrNoVisitorContext
	}

	if data.AccessType != "" && data.AccessType != wire.Access3GPP {
		return nil
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.Registered = false })

	ctx = context.WithoutCancel(ctx)
	log := logger.From(ctx, logger.AmfLog).With(logger.SUPIFromIMSI(imsi), zap.String("reason", string(data.DeregReason)))

	amf.home.Go(imsi, func() {
		if held, ok := amf.lookupUeByIMSI(imsi); !ok || held != ue {
			return
		}

		switch data.DeregReason {
		case wire.DeregSubscriptionWithdrawn, wire.DeregOperatorDeterminedBarring:
			log.Info("the home UDM withdrew the visitor's registration; deregistering it")
			amf.DeregisterUE(ctx, ue, models.ReleaseFor(models.ReleaseSubscriptionWithdrawn), NetworkDeregistration{})
		case wire.Dereg3GPPAccessNotAllowed:
			cause := fgs.GMMCauseNoSuitableCellsInTrackingArea

			log.Info("the home UDM no longer allows 3GPP access; deregistering the visitor")
			amf.DeregisterUE(ctx, ue, models.ReleaseFor(models.ReleaseSubscriptionWithdrawn), NetworkDeregistration{Cause: &cause})
		case wire.DeregReregistrationRequired, wire.DeregDisasterConditionTerminated:
			log.Info("the home UDM requires the visitor to register again")
			amf.DeregisterUE(ctx, ue, models.ReleaseFor(models.ReleaseDeregistration), NetworkDeregistration{ReRegistration: true})
		default:
			log.Info("the visitor registered elsewhere; removing its context")
			amf.dropVisitorContext(ctx, ue)
		}
	})

	return nil
}

func (amf *AMF) dropVisitorContext(ctx context.Context, ue *UeContext) {
	if !amf.ServesUeContext(ue) {
		return
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		amf.DeregisterAndRemoveUeContext(ctx, ue, models.ReleaseFor(models.ReleaseDeregistration))
		return
	}

	if !ueConn.ReleaseWithAction(ctx, UeContextReleaseDueToNwInitiatedDeregistraion, causeReleaseDueToCNDetectedMobility) {
		return
	}

	ue.Deregister(ctx, models.ReleaseFor(models.ReleaseDeregistration))
}

func (amf *AMF) HomeSubscriptionChanged(ctx context.Context, partnerID, imsi string, n wire.ModificationNotification) error {
	ue, v, ok := amf.heldVisitor(partnerID, imsi)
	if !ok || v.SDM == nil || (n.SubscriptionID != "" && v.SDM.ID != "" && n.SubscriptionID != v.SDM.ID) {
		return roaming.ErrNoVisitorContext
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.LCSPrivacy = nil })

	ctx = context.WithoutCancel(ctx)

	amf.home.Go(imsi, func() {
		if held, ok := amf.lookupUeByIMSI(imsi); !ok || held != ue {
			return
		}

		amf.refreshHomeSubscription(ctx, ue, imsi)
	})

	return nil
}

func (amf *AMF) refreshHomeSubscription(ctx context.Context, ue *UeContext, imsi string) {
	log := logger.From(ctx, logger.AmfLog).With(logger.SUPIFromIMSI(imsi))

	v, ok := ue.Visitor()
	if !ok || amf.HomeNetwork == nil {
		return
	}

	data, err := amf.HomeNetwork.SubscriptionData(ctx, v.Home, "imsi-"+imsi)
	if err != nil {
		log.Warn("failed to read the changed home subscription; keeping the cached one", zap.Error(err))
		return
	}

	if r := homeSubscriptionRestriction(data.AmData); r != nil {
		cause := r.Cause

		log.Info("the home subscription no longer allows service in this network; deregistering the visitor", logger.Cause(r.Reason))
		amf.DeregisterUE(ctx, ue, models.ReleaseFor(models.ReleaseSubscriptionChanged), NetworkDeregistration{Cause: &cause})

		return
	}

	before := ue.AllowedNssai()

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.Subscription = &data })

	if ue.State() != Registered {
		return
	}

	profile, err := amf.SubscriberProfile(ctx, ue.Supi())
	if err != nil {
		log.Warn("failed to merge the changed home subscription", zap.Error(err))
		return
	}

	if !sameSnssais(before, profile.AllowedNssai) {
		log.Info("the visitor's subscribed slices changed; asking it to register again")
		amf.DeregisterUE(ctx, ue, models.ReleaseFor(models.ReleaseSubscriptionChanged), NetworkDeregistration{ReRegistration: true})

		return
	}

	ue.SetAmbr(profile.Ambr)
	amf.SyncUEAMBR(ctx, ue)

	log.Info("applied the changed home subscription")
}

func sameSnssais(a, b []models.Snssai) bool {
	if len(a) != len(b) {
		return false
	}

	for _, s := range a {
		if !slices.ContainsFunc(b, func(o models.Snssai) bool { return o.Sst == s.Sst && o.Sd == s.Sd }) {
			return false
		}
	}

	return true
}

func (amf *AMF) renewHomeSubscriptions(ctx context.Context) {
	renewer, ok := amf.HomeNetwork.(HomeRenewer)
	if !ok {
		return
	}

	now := time.Now()

	for _, ue := range amf.registeredUEs() {
		v, ok := ue.Visitor()
		if !ok || v.SDM == nil || v.SDM.Expires == "" {
			continue
		}

		expires, err := v.SDM.Expires.Time()
		if err != nil || expires.Sub(now) > sdmRenewBefore {
			continue
		}

		supi := ue.Supi()
		if !supi.IsIMSI() {
			continue
		}

		imsi, uri := supi.IMSI(), v.SDM.URI

		amf.home.Go(imsi, func() {
			next, err := renewer.Renew(ctx, v.Home, uri, wire.DateTimeOf(time.Now().Add(sdmLifetime)))
			if err != nil {
				logger.From(ctx, logger.AmfLog).Warn("failed to renew the visitor's home subscription", logger.SUPIFromIMSI(imsi), zap.Error(err))
				return
			}

			ue.updateVisitor(func(cur *roaming.Visitor5G) {
				if cur.SDM != nil && cur.SDM.URI == uri {
					cur.SDM.Expires = next
				}
			})
		})
	}
}
