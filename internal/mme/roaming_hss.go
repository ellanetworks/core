// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

type DiameterRegistrar interface {
	Handle(applicationID, commandCode uint32, h diameter.Handler)
}

func (m *MME) RegisterHPLMNHandlers(r DiameterRegistrar) {
	r.Handle(s6a.ApplicationID, s6a.CommandCancelLocation, m.countedHPLMNHandler(s6a.CommandCancelLocation, m.CancelLocation))
	r.Handle(s6a.ApplicationID, s6a.CommandInsertSubscriberData, m.countedHPLMNHandler(s6a.CommandInsertSubscriberData, m.InsertSubscriberData))
	r.Handle(s6a.ApplicationID, s6a.CommandDeleteSubscriberData, m.countedHPLMNHandler(s6a.CommandDeleteSubscriberData, m.DeleteSubscriberData))
	r.Handle(s6a.ApplicationID, s6a.CommandReset, m.countedHPLMNHandler(s6a.CommandReset, m.Reset))
}

func userUnknown(req *diameter.Message, id diameter.Identity) *diameter.Message {
	return s6a.NewAnswer(req, id, tgpp.Experimental(tgpp.ResultErrorUserUnknown), s6a.Features{})
}

func unableToComply(req *diameter.Message, id diameter.Identity) *diameter.Message {
	return s6a.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply}, s6a.Features{})
}

func (m *MME) hplmnVisitor(ctx context.Context, imsi string, req *diameter.Message, ingress string) (*UeContext, bool) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return nil, false
	}

	v, ok := ue.Visitor()
	if !ok {
		return nil, false
	}

	origin := tgpp.ParseEnvelope(req).Origin

	if !m.fromPartner(ctx, v.PartnerID, origin, ingress) {
		logger.From(ctx, logger.MmeLog).Warn("S6a request for a visitor from outside its home network",
			logger.SUPIFromIMSI(imsi), zap.String("origin_host", origin.OriginHost), zap.String("origin_realm", origin.OriginRealm),
			zap.String("ingress_peer", ingress))

		return nil, false
	}

	return ue, true
}

func (m *MME) fromPartner(ctx context.Context, partnerID string, origin diameter.Identity, ingress string) bool {
	if m.Roaming == nil || ingress == "" {
		return false
	}

	partners, err := m.Roaming.ListRoamingPartners(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to list roaming partners", zap.Error(err))
		return false
	}

	i := slices.IndexFunc(partners, func(p db.RoamingPartner) bool { return p.ID == partnerID })
	if i < 0 || partners[i].DiameterPeerID == nil || ingress != roaming.DiameterPeerID(*partners[i].DiameterPeerID) {
		return false
	}

	return slices.ContainsFunc(partners[i].PLMNs, func(plmn db.RoamingPLMN) bool {
		realm, err := diameternode.DiameterRealm(plmn.Mcc, plmn.Mnc)
		return err == nil && strings.EqualFold(realm, origin.OriginRealm)
	})
}

func (m *MME) CancelLocation(ctx context.Context, id diameter.Identity, ingress string, req *diameter.Message) *diameter.Message {
	clr, err := s6a.ParseCancelLocationRequest(req)
	if err != nil {
		return s6a.NewErrorAnswer(req, id, err, s6a.Features{})
	}

	ans, _ := s6a.NewCancelLocationAnswer(req, id, s6a.CancelLocation{})

	m.hplmnCancelledMove(ctx, clr.IMSI, tgpp.ParseEnvelope(req).Origin, ingress)

	ue, ok := m.hplmnVisitor(ctx, clr.IMSI, req, ingress)
	if !ok {
		return ans
	}

	log := logger.From(ctx, logger.MmeLog).With(logger.SUPIFromIMSI(clr.IMSI), zap.Stringer("cancellation_type", clr.Type))

	switch clr.Type {
	case s6a.CancellationSubscriptionWithdrawal, s6a.CancellationDisasterConditionTerminated:
		detachType := eps.DetachTypeReattachNotRequired
		if clr.Flags&s6a.CLRFlagReattachRequired != 0 {
			detachType = eps.DetachTypeReattachRequired
		}

		log.Info("home HSS cancelled the visitor's subscription; detaching it")
		m.home.Go(clr.IMSI, func() {
			m.cancelVisitor(context.WithoutCancel(ctx), ue, true, detachType, "subscription withdrawn by the home HSS")
		})
	case s6a.CancellationSGSNUpdateProcedure:
		log.Info("ignoring an SGSN cancellation")
	case s6a.CancellationMMEUpdateProcedure:
		if m.movingTo5GS(ue) || !visitorRegistered(ue) {
			ue.setHPLMNRegistered(false)
			log.Info("home HSS cancelled the location of a visitor that moves to 5GS; the move removes its context")

			return ans
		}

		fallthrough
	default:
		log.Info("home HSS cancelled the visitor's location; removing its context")
		m.home.Go(clr.IMSI, func() {
			m.cancelVisitor(context.WithoutCancel(ctx), ue, false, 0, "location cancelled by the home HSS")
		})
	}

	return ans
}

func visitorRegistered(ue *UeContext) bool {
	v, ok := ue.Visitor()

	return ok && v.Registered
}

func (m *MME) InsertSubscriberData(ctx context.Context, id diameter.Identity, ingress string, req *diameter.Message) *diameter.Message {
	idr, err := s6a.ParseInsertSubscriberDataRequest(req)
	if err != nil {
		return s6a.NewErrorAnswer(req, id, err, s6a.Features{})
	}

	ue, ok := m.hplmnVisitor(ctx, idr.IMSI, req, ingress)
	if !ok {
		return userUnknown(req, id)
	}

	if err := ue.updateHPLMNSubscription(func(s s6a.SubscriptionData) (s6a.SubscriptionData, error) {
		return s.Merge(idr.Subscription), nil
	}); err != nil {
		return unableToComply(req, id)
	}

	ida, comply := m.insertSubscriberDataAnswer(ue, idr.Flags)

	if !comply {
		ue.unconfirmHPLMNLocation()
	}

	m.home.Go(idr.IMSI, func() { m.applyHPLMNSubscription(context.WithoutCancel(ctx), ue) })

	logger.From(ctx, logger.MmeLog).Info("home HSS updated the visitor's subscription",
		logger.SUPIFromIMSI(idr.IMSI), zap.Stringer("idr_flags", idr.Flags), zap.Bool("complied", comply))

	if !comply {
		return unableToComply(req, id)
	}

	ans, err := s6a.NewInsertSubscriberDataAnswer(req, id, ida)
	if err != nil {
		return unableToComply(req, id)
	}

	return ans
}

func (m *MME) insertSubscriberDataAnswer(ue *UeContext, flags s6a.IDRFlags) (s6a.InsertSubscriberData, bool) {
	var ida s6a.InsertSubscriberData

	const unsupported = s6a.IDRFlagUEReachabilityRequest

	if flags&unsupported != 0 {
		return ida, false
	}

	if flags&s6a.IDRFlagCurrentLocationRequest != 0 && flags&s6a.IDRFlagEPSLocationInformationRequest == 0 {
		return ida, false
	}

	attached := ue.EMMState() == EMMRegistered

	if flags&s6a.IDRFlagTADSDataRequest != 0 && attached {
		notSupported := s6a.IMSVoiceOverPSNotSupported
		rat := tgpp.RATEUTRAN
		ida.IMSVoiceOverPSSupported = &notSupported
		ida.LastUEActivityTime = ue.lastSeenTime().UTC().Truncate(time.Second)
		ida.RATType = &rat
	}

	if flags&s6a.IDRFlagEPSUserStateRequest != 0 {
		state := userState(ue)
		ida.UserState = &state
	}

	if flags&s6a.IDRFlagEPSLocationInformationRequest != 0 {
		ida.Location = epsLocation(ue.GetUserLocation(), time.Now())
		if ida.Location == nil {
			ida.Location = &s6a.EPSLocationInformation{}
		}
	}

	return ida, true
}

func userState(ue *UeContext) s6a.UserState {
	if ue.EMMState() != EMMRegistered {
		return s6a.UserStateDetached
	}

	return s6a.UserStateConnectedReachableForPaging
}

func (m *MME) DeleteSubscriberData(ctx context.Context, id diameter.Identity, ingress string, req *diameter.Message) *diameter.Message {
	dsr, err := s6a.ParseDeleteSubscriberDataRequest(req)
	if err != nil {
		return s6a.NewErrorAnswer(req, id, err, s6a.Features{})
	}

	ue, ok := m.hplmnVisitor(ctx, dsr.IMSI, req, ingress)
	if !ok {
		return userUnknown(req, id)
	}

	if err := ue.updateHPLMNSubscription(func(s s6a.SubscriptionData) (s6a.SubscriptionData, error) {
		return s.Withdraw(dsr.SubscriptionDataDeletion)
	}); err != nil {
		logger.From(ctx, logger.MmeLog).Info("refused a deletion of the visitor's subscription data",
			logger.SUPIFromIMSI(dsr.IMSI), zap.Stringer("dsr_flags", dsr.Flags), zap.Error(err))

		return s6a.NewErrorAnswer(req, id, err, s6a.Features{})
	}

	m.home.Go(dsr.IMSI, func() { m.applyHPLMNSubscription(context.WithoutCancel(ctx), ue) })

	logger.From(ctx, logger.MmeLog).Info("home HSS deleted part of the visitor's subscription",
		logger.SUPIFromIMSI(dsr.IMSI), zap.Stringer("dsr_flags", dsr.Flags))

	ans, err := s6a.NewDeleteSubscriberDataAnswer(req, id, s6a.DeleteSubscriberData{})
	if err != nil {
		return unableToComply(req, id)
	}

	return ans
}

func (m *MME) Reset(ctx context.Context, id diameter.Identity, ingress string, req *diameter.Message) *diameter.Message {
	rsr, err := s6a.ParseResetRequest(req)
	if err != nil {
		return s6a.NewErrorAnswer(req, id, err, s6a.Features{})
	}

	origin := tgpp.ParseEnvelope(req).Origin
	hss := origin.OriginHost
	marked := 0
	trusted := map[string]bool{}

	for _, ue := range m.visitorsServedBy(hss) {
		imsi := ue.imsiOrEmpty()

		if len(rsr.UserIDs) > 0 && !slices.ContainsFunc(rsr.UserIDs, func(prefix string) bool { return strings.HasPrefix(imsi, prefix) }) {
			continue
		}

		v, ok := ue.Visitor()
		if !ok {
			continue
		}

		allowed, checked := trusted[v.PartnerID]
		if !checked {
			allowed = m.fromPartner(ctx, v.PartnerID, origin, ingress)
			trusted[v.PartnerID] = allowed
		}

		if !allowed {
			continue
		}

		ue.unconfirmHPLMNLocation()

		marked++
	}

	logger.From(ctx, logger.MmeLog).Info("home HSS reset; visitors will be restored at their next contact",
		zap.String("hss", hss), zap.Int("visitors", marked))

	ans, err := s6a.NewResetAnswer(req, id, s6a.Reset{})
	if err != nil {
		return unableToComply(req, id)
	}

	return ans
}

func (m *MME) visitorsServedBy(hss string) []*UeContext {
	m.mu.RLock()

	ues := make([]*UeContext, 0, len(m.UEs))
	for _, ue := range m.UEs {
		ues = append(ues, ue)
	}

	m.mu.RUnlock()

	out := ues[:0]

	for _, ue := range ues {
		if v, ok := ue.Visitor(); ok && v.Registered && strings.EqualFold(v.Peer.HSS, hss) {
			out = append(out, ue)
		}
	}

	return out
}
