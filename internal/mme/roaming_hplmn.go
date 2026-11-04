// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"encoding/binary"
	"errors"
	"strconv"
	"time"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/ueregistration"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

const DefaultMTMSIFreezeTime = T3412PeriodicTAU

func (m *MME) purgeRegistration(ue *UeContext, supi etsi.SUPI, tmsis []etsi.TMSI) bool {
	if !supi.IsIMSI() {
		return false
	}

	imsi := supi.IMSI()

	ue.mu.Lock()
	v, registered := ue.hplmnRegistrationLocked()
	loc := ue.Location
	ue.mu.Unlock()

	if !registered || m.HPLMN == nil {
		m.purgeLocalRegistration(imsi)
		return false
	}

	m.home.Go(imsi, func() {
		freeze := m.purgeAtHPLMN(imsi, v.Peer, epsLocation(loc, time.Now()))
		m.releaseMTMSIs(tmsis, freeze)
		m.purgeLocalRegistration(imsi)
	})

	return true
}

func (m *MME) purgeLocalRegistration(imsi string) {
	if m.Registrations != nil {
		m.Registrations.Purge(imsi)
	}
}

func (m *MME) purgeAtHPLMN(imsi string, home roaming.HPLMNPeer, loc *s6a.EPSLocationInformation) bool {
	if m.HoldsUE(imsi) {
		return false
	}

	log := logger.MmeLog.With(logger.SUPIFromIMSI(imsi), zap.String("home_realm", home.Realm))

	pua, err := m.HPLMN.PurgeUE(context.Background(), home, s6a.PurgeUERequest{IMSI: imsi, Location: loc})
	if err != nil {
		log.Warn("failed to purge the visitor at its home HSS", zap.Error(err))
		return false
	}

	freeze := pua.Flags&s6a.PUAFlagFreezeMTMSI != 0
	log.Info("purged the visitor at its home HSS", zap.Bool("freeze_m_tmsi", freeze))

	return freeze
}

func (m *MME) releaseMTMSIs(tmsis []etsi.TMSI, freeze bool) {
	if len(tmsis) == 0 {
		return
	}

	free := func() {
		for _, t := range tmsis {
			m.tmsi.Free(t)
		}
	}

	if !freeze {
		free()
		return
	}

	time.AfterFunc(m.mtmsiFreezeTime, free)
}

func epsLocation(loc models.UserLocation, now time.Time) *s6a.EPSLocationInformation {
	e := loc.EutraLocation
	if e == nil {
		return nil
	}

	var mme s6a.MMELocationInformation

	if e.Ecgi != nil && e.Ecgi.PlmnID != nil {
		plmn, err1 := nas.PLMN{MCC: e.Ecgi.PlmnID.Mcc, MNC: e.Ecgi.PlmnID.Mnc}.Octets()
		cell, err2 := strconv.ParseUint(e.Ecgi.EutraCellID, 16, 28)

		if err1 == nil && err2 == nil {
			mme.ECGI = binary.BigEndian.AppendUint32(plmn[:], uint32(cell))
		}
	}

	if e.Tai != nil && e.Tai.PlmnID != nil {
		plmn, err1 := nas.PLMN{MCC: e.Tai.PlmnID.Mcc, MNC: e.Tai.PlmnID.Mnc}.Octets()
		tac, err2 := strconv.ParseUint(e.Tai.Tac, 16, 16)

		if err1 == nil && err2 == nil {
			mme.TAI = binary.BigEndian.AppendUint16(plmn[:], uint16(tac))
		}
	}

	if e.UeLocationTimestamp != nil {
		age := uint32(max(now.Sub(*e.UeLocationTimestamp), 0) / time.Minute)
		mme.Age = &age
	}

	if len(mme.ECGI) == 0 && len(mme.TAI) == 0 {
		return nil
	}

	return &s6a.EPSLocationInformation{MME: &mme}
}

func (m *MME) NotifyEquipmentChange(ctx context.Context, ue *UeContext) {
	v, ok := ue.Visitor()
	if !ok || !v.Registered || v.Subscription == nil || m.HPLMN == nil {
		return
	}

	terminal := terminalInformation(ue)
	if terminal == nil {
		return
	}

	imsi := ue.IMSI()
	ctx = context.WithoutCancel(ctx)

	m.home.Go(imsi, func() {
		err := m.HPLMN.Notify(ctx, v.Peer, s6a.NotifyRequest{IMSI: imsi, Terminal: terminal})

		var re *s6a.ResultError

		switch {
		case err == nil:
			logger.From(ctx, logger.MmeLog).Info("notified the home HSS of the new equipment identity", logger.SUPIFromIMSI(imsi))
		case errors.As(err, &re) && re.IsExperimental(tgpp.ResultErrorUserUnknown):
			logger.From(ctx, logger.MmeLog).Info("the home HSS no longer knows the visitor; detaching it", logger.SUPIFromIMSI(imsi))
			m.cancelVisitor(ctx, ue, true, eps.DetachTypeReattachNotRequired, "unknown user at the home HSS")
		case errors.As(err, &re) && re.IsExperimental(tgpp.ResultErrorUnknownServingNode):
			logger.From(ctx, logger.MmeLog).Info("the home HSS does not hold this MME as serving node; restoring at the next contact", logger.SUPIFromIMSI(imsi))
			ue.unconfirmHPLMNLocation()
		default:
			logger.From(ctx, logger.MmeLog).Warn("failed to notify the home HSS of the new equipment identity", logger.SUPIFromIMSI(imsi), zap.Error(err))
		}
	})
}

func (m *MME) ConfirmHPLMNLocation(ctx context.Context, ue *UeContext) {
	v, ok := ue.Visitor()
	if !ok || !v.Registered || v.Confirmed || m.HPLMN == nil || !ue.hplmnRestoring.CompareAndSwap(false, true) {
		return
	}

	serving, err := m.OperatorPLMN(ctx)
	if err != nil {
		ue.hplmnRestoring.Store(false)
		logger.From(ctx, logger.MmeLog).Warn("cannot restore the visitor at its home HSS: no serving PLMN", zap.Error(err))

		return
	}

	imsi := ue.IMSI()
	ctx = context.WithoutCancel(ctx)

	m.home.Go(imsi, func() {
		defer ue.hplmnRestoring.Store(false)

		if held, ok := m.LookupUeByIMSI(imsi); !ok || held != ue {
			return
		}

		if err := m.updateLocation(ctx, ue, serving, ULRRestoration); err != nil {
			if r, permanent := restorationRefusal(err); permanent {
				logger.From(ctx, logger.MmeLog).Info("the home HSS refused to restore the visitor; detaching it",
					logger.SUPIFromIMSI(imsi), logger.Cause(r.Cause.String()), zap.Error(err))
				m.detachRefusedVisitor(ctx, ue, r)

				return
			}

			logger.From(ctx, logger.MmeLog).Warn("failed to restore the visitor at its home HSS; retrying at the next contact",
				logger.SUPIFromIMSI(imsi), zap.Error(err))

			return
		}

		logger.From(ctx, logger.MmeLog).Info("restored the visitor at its home HSS", logger.SUPIFromIMSI(imsi))
		m.applyHPLMNSubscription(ctx, ue)
	})
}

func (m *MME) applyHPLMNSubscription(ctx context.Context, ue *UeContext) {
	if held, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); !ok || held != ue || ue.EMMState() != EMMRegistered {
		return
	}

	v, ok := ue.Visitor()
	if !ok || v.Subscription == nil {
		return
	}

	if err := subscriptionRestriction(*v.Subscription); err != nil {
		var r *Rejection

		cause := eps.EMMCauseNoSuitableCellsInTrackingArea
		if errors.As(err, &r) {
			cause = r.Cause
		}

		logger.From(ctx, logger.MmeLog).Info("the home subscription no longer allows service in this network; detaching the visitor",
			logger.SUPI(ue.Supi().String()), logger.Cause(err.Error()))
		m.detachUE(ctx, ue, eps.DetachTypeReattachNotRequired, &cause, models.ReleaseFor(models.ReleaseSubscriptionChanged), "home subscription change")

		return
	}

	if m.detachBarredVisitor(ctx, ue, *v.Subscription) {
		return
	}

	m.recordVisitorMSISDN(ctx, ue)

	if subscribed, err := SubscribedUEAMBR(ctx, m, ue.IMSI()); err == nil {
		ue.SetSubscribedUEAMBR(subscribed)
	}

	m.ReconcileUE(ctx, ue)
}

func restorationRefusal(err error) (*Rejection, bool) {
	var re *s6a.ResultError
	if !errors.As(err, &re) {
		return nil, false
	}

	switch {
	case re.IsExperimental(tgpp.ResultErrorUserUnknown),
		re.IsExperimental(tgpp.ResultErrorRoamingNotAllowed),
		re.IsExperimental(tgpp.ResultErrorUnknownEPSSubscription),
		re.IsExperimental(tgpp.ResultErrorRATNotAllowed):
		return HPLMNRejection(err), true
	default:
		return nil, false
	}
}

func (m *MME) detachRefusedVisitor(ctx context.Context, ue *UeContext, r *Rejection) {
	if held, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); !ok || held != ue {
		return
	}

	ue.setHPLMNRegistered(false)

	cause := r.Cause
	m.detachUE(ctx, ue, eps.DetachTypeReattachNotRequired, &cause, models.ReleaseFor(models.ReleaseSubscriptionWithdrawn), "home HSS refused the restoration")
}

func (m *MME) detachBarredVisitor(ctx context.Context, ue *UeContext, sub s6a.SubscriptionData) bool {
	if !sub.BarsVisitedPLMNAccessPoints() {
		return false
	}

	logger.From(ctx, logger.MmeLog).Info("operator determined barring bars every locally broken out PDN connection of the visitor; detaching it",
		logger.SUPI(ue.Supi().String()))
	m.detachUE(ctx, ue, eps.DetachTypeReattachNotRequired, nil, models.ReleaseFor(models.ReleaseSubscriptionChanged), "operator determined barring")

	return true
}

func (m *MME) cancelVisitor(ctx context.Context, ue *UeContext, detach bool, detachType eps.DetachTypeNetwork, reason string) {
	if held, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); !ok || held != ue {
		return
	}

	ue.setHPLMNRegistered(false)

	if detach {
		m.detachUE(ctx, ue, detachType, nil, models.ReleaseFor(models.ReleaseSubscriptionWithdrawn), reason)
		return
	}

	logger.From(ctx, logger.MmeLog).Info("removing the visitor's context: "+reason, logger.SUPI(ue.Supi().String()))

	m.ReleaseAllSessions(ctx, ue, models.ReleaseFor(models.ReleaseDeregistration))
	ue.TransitionTo(ctx, EMMDeregistered)
	m.ReleaseUEContext(ctx, ue, CauseNASNormalRelease)
}

func (m *MME) withdrawRoamer(ctx context.Context, ue *UeContext, why ueregistration.Withdrawal) {
	if held, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); !ok || held != ue || !ue.IsVisitor() {
		return
	}

	if why == ueregistration.RoamerDeregistered {
		logger.From(ctx, logger.MmeLog).Info("the operator deregistered the visitor; detaching it", logger.SUPI(ue.Supi().String()))
		m.detachUE(ctx, ue, eps.DetachTypeReattachNotRequired, nil, models.ReleaseFor(models.ReleaseDeregistration), "operator deregistration of the visitor")

		return
	}

	cause := eps.EMMCausePLMNNotAllowed
	if why == ueregistration.RoamingAgreementWithdrawn {
		cause = eps.EMMCauseNoSuitableCellsInTrackingArea
	}

	logger.From(ctx, logger.MmeLog).Info("roaming agreement withdrawn; detaching the visitor",
		logger.SUPI(ue.Supi().String()), logger.Cause(cause.String()))

	m.detachUE(ctx, ue, eps.DetachTypeReattachNotRequired, &cause, models.ReleaseFor(models.ReleaseSubscriptionWithdrawn), "roaming agreement withdrawal")
}
