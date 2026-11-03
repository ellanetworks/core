// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
)

type HomeSessionBinder interface {
	BindHomeSessions(ctx context.Context, supi etsi.SUPI)
}

func (ue *UeContext) EPSInterworkingSubscribed(snssai models.Snssai, dnn string) bool {
	v, ok := ue.Visitor()
	if !ok {
		return true
	}

	return roaming.EPSInterworkingSubscribed(v.SmfSelection(), snssai, dnn)
}

func (amf *AMF) handVisitorToEPS(ctx context.Context, ue *UeContext) {
	supi := ue.Supi()

	ue.mu.Lock()

	v := ue.visitor
	if v == nil {
		ue.mu.Unlock()
		return
	}

	sdm, home, partner := v.SDM, v.Home, v.PartnerName
	v.Registered = false
	v.Guami = nil
	v.Subscription = nil
	v.SDM = nil
	v.ConfirmURI = ""
	v.Confirmed = false
	v.SoR = nil

	ue.mu.Unlock()

	logger.From(ctx, logger.AmfLog).Info("visitor moved to EPS; its home registration passes to the MME, no purge",
		logger.SUPI(supi.String()), zap.String("roaming_partner", partner))

	if sdm == nil || amf.HomeNetwork == nil || !supi.IsIMSI() {
		return
	}

	ctx = context.WithoutCancel(ctx)

	amf.home.Go(supi.IMSI(), func() {
		if err := amf.HomeNetwork.Unsubscribe(ctx, home, sdm.URI); err != nil {
			logger.From(ctx, logger.AmfLog).Info("failed to unsubscribe from the home subscription data of a visitor that moved to EPS",
				logger.SUPI(supi.String()), zap.Error(err))
		}
	})
}

func (amf *AMF) homeMovedToEPS(ctx context.Context, partnerID, imsi string, data wire.DeregistrationData) (bool, error) {
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

func (amf *AMF) BindHomeSessions(ctx context.Context, ue *UeContext) {
	if !ue.IsVisitor() {
		return
	}

	binder, ok := amf.Session.(HomeSessionBinder)
	if !ok {
		return
	}

	supi := ue.Supi()
	ctx = context.WithoutCancel(ctx)

	go binder.BindHomeSessions(ctx, supi)
}
