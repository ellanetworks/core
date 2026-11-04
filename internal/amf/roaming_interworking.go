// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

func HPLMNSnssai(serving models.Snssai) models.Snssai {
	return models.Snssai{Sst: serving.Sst, Sd: models.NormalizeSD(serving.Sd)}
}

func (ue *UeContext) EPSInterworkingSubscribed(snssai models.Snssai, dnn string) bool {
	v, ok := ue.Visitor()
	if !ok {
		return true
	}

	return roaming.EPSInterworkingSubscribed(v.SmfSelection(), HPLMNSnssai(snssai), dnn)
}

func (ue *UeContext) SessionHPLMNTarget(snssai models.Snssai, dnn string, epsInterworking bool) *smf.HPLMNTarget {
	v, ok := ue.Visitor()
	if !ok {
		return nil
	}

	hplmn := HPLMNSnssai(snssai)

	return &smf.HPLMNTarget{
		PartnerID:        v.PartnerID,
		HPLMN:            v.HPLMN.PLMN.Model(),
		ServingNetwork:   v.HPLMN.Serving.Model(),
		RoutingIndicator: v.HPLMN.RoutingIndicator,
		UDMGroupID:       v.HPLMN.UDMGroupID,
		HPLMNSnssai:      hplmn,
		LBO:              roaming.LocalBreakoutAllowed5G(v.SmfSelection(), hplmn, dnn),
		EPSInterworking:  epsInterworking,
	}
}

func (ue *UeContext) ArrivingHPLMNTarget(snssai models.Snssai) *smf.HPLMNTarget {
	v, ok := ue.Visitor()
	if !ok {
		return nil
	}

	return &smf.HPLMNTarget{
		PartnerID:        v.PartnerID,
		HPLMN:            v.HPLMN.PLMN.Model(),
		ServingNetwork:   v.HPLMN.Serving.Model(),
		RoutingIndicator: v.HPLMN.RoutingIndicator,
		UDMGroupID:       v.HPLMN.UDMGroupID,
		HPLMNSnssai:      HPLMNSnssai(snssai),
		LBO:              true,
		EPSInterworking:  true,
	}
}

func (amf *AMF) handVisitorToEPS(ctx context.Context, ue *UeContext) {
	supi := ue.Supi()

	ue.mu.Lock()

	v := ue.visitor
	if v == nil {
		ue.mu.Unlock()
		return
	}

	sdm, home, partnerID, partner, registered, guami := v.SDM, v.HPLMN, v.PartnerID, v.PartnerName, v.Registered, v.Guami
	v.Registered = false
	v.Guami = nil
	v.Subscription = nil
	v.SDM = nil
	v.ConfirmURI = ""
	v.Confirmed = false
	v.SoR = nil

	ue.mu.Unlock()

	log := logger.From(ctx, logger.AmfLog).With(logger.SUPI(supi.String()), zap.String("roaming_partner", partner))

	if amf.HPLMN == nil || !supi.IsIMSI() {
		return
	}

	imsi := supi.IMSI()
	ctx = context.WithoutCancel(ctx)

	if registered && guami != nil {
		log.Info("visitor moved to EPS; its home registration passes to the MME, purging only if the home network does not cancel it")
		amf.guardCrossCancel(imsi, partnerID, home, *guami)
	} else {
		log.Info("visitor moved to EPS; the home network already cancelled its registration in this AMF")
	}

	if sdm == nil {
		return
	}

	amf.home.Go(imsi, func() {
		if err := amf.HPLMN.Unsubscribe(ctx, home, sdm.URI); err != nil {
			logger.From(ctx, logger.AmfLog).Info("failed to unsubscribe from the home subscription data of a visitor that moved to EPS",
				logger.SUPI(supi.String()), zap.Error(err))
		}
	})
}

func (amf *AMF) guardCrossCancel(imsi, partnerID string, home roaming.HPLMNTarget, guami wire.Guami) {
	amf.crossCancel.Arm(imsi, partnerID, func() {
		amf.home.Go(imsi, func() {
			if amf.HoldsUE(imsi) {
				return
			}

			log := logger.AmfLog.With(logger.SUPIFromIMSI(imsi), zap.String("roaming_partner", partnerID))

			if err := amf.HPLMN.Deregister(context.Background(), home, "imsi-"+imsi, guami); err != nil {
				log.Warn("the home network did not cancel the registration of a visitor that moved to EPS, and purging it failed", zap.Error(err))
				return
			}

			log.Info("the home network did not cancel the registration of a visitor that moved to EPS; purged it")
		})
	})
}

func (amf *AMF) SetCrossCancelWait(d time.Duration) {
	amf.crossCancel.SetWait(d)
}

func (amf *AMF) hplmnMovedToEPS(ctx context.Context, partnerID, imsi string, data wire.DeregistrationData) (bool, error) {
	if pending, ok := amf.crossCancel.Partner(imsi); ok && pending == partnerID && amf.crossCancel.Disarm(imsi) {
		logger.From(ctx, logger.AmfLog).Info("the home network cancelled the registration of a visitor that moved to EPS", logger.SUPIFromIMSI(imsi))
	}

	ue, ok := amf.lookupUeByIMSI(imsi)
	if !ok {
		return true, roaming.ErrNoVisitorContext
	}

	v, ok := ue.Visitor()
	if !ok || v.PartnerID != partnerID {
		return true, roaming.ErrNoVisitorContext
	}

	if data.AccessType != "" && data.AccessType != wire.Access3GPP {
		return true, nil
	}

	log := logger.From(ctx, logger.AmfLog).With(logger.SUPIFromIMSI(imsi), zap.String("reason", string(data.DeregReason)))

	if !v.Registered {
		log.Info("the home network confirmed the visitor's move to EPS")
		return true, nil
	}

	if !amf.HandoverToEPSInProgress(ue) {
		return false, nil
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.Registered = false })

	log.Info("the home network registered the visitor in EPS during its handover; the handover completes the move")

	return true, nil
}
