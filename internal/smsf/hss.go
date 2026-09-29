// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"go.uber.org/zap"
)

func (s *SMSF) SendRoutingInfoForSM(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	r, err := s6c.ParseSendRoutingInfoForSMRequest(req)
	if err != nil {
		return s6c.NewErrorAnswer(req, id, err)
	}

	fail := func(code uint32, absent s6c.AbsentUserDiagnostics) *diameter.Message {
		ans, err := s6c.NewSendRoutingInfoForSMErrorAnswer(req, id, s6c.ResultError{Result: tgpp.Experimental(code), Absent: absent}, r.SMSFSupport)
		if err != nil {
			return s6c.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply})
		}

		return ans
	}

	sub, err := s.lookup(ctx, r.MSISDN, r.IMSI)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return fail(tgpp.ResultErrorUserUnknown, s6c.AbsentUserDiagnostics{})
		}

		return s6c.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply})
	}

	settings, err := s.store.GetSMSSettings(ctx)
	if err != nil {
		return s6c.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply})
	}

	if sub.Msisdn == "" || !settings.Enabled() {
		return fail(tgpp.ResultErrorServiceNotSubscribed, s6c.AbsentUserDiagnostics{})
	}

	nodes, absentDiagnostics := s.servingNodes(ctx, sub.Imsi, settings.SMSNumber, r.SMSFSupport)
	if nodes.Serving == nil && nodes.SMSF3GPP == nil && r.DeliveryNotIntended == nil {
		s.logger.Info("Answered a routing request for an unreachable subscriber", zap.String("imsi", sub.Imsi))
		return fail(tgpp.ResultErrorAbsentUser, absentDiagnostics)
	}

	ans, err := s6c.NewSendRoutingInfoForSMAnswer(req, id, s6c.Routing{ServingNodes: nodes, IMSI: sub.Imsi}, r.SMSFSupport)
	if err != nil {
		s.logger.Warn("Could not build a routing answer", zap.String("imsi", sub.Imsi), zap.Error(err))
		return s6c.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply})
	}

	return ans
}

func (s *SMSF) ReportSMDeliveryStatus(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	rep, err := s6c.ParseReportSMDeliveryStatusRequest(req)
	if err != nil {
		return s6c.NewErrorAnswer(req, id, err)
	}

	sub, err := s.lookup(ctx, rep.MSISDN, rep.IMSI)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return s6c.NewAnswer(req, id, tgpp.Experimental(tgpp.ResultErrorUserUnknown))
		}

		return s6c.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply})
	}

	if !rep.SingleAttempt {
		s.recordDeliveryReport(sub.Imsi, rep)
	}

	var result s6c.ReportResult

	if failed := nodeNames(rep.Failed); len(failed) > 0 {
		settings, err := s.store.GetSMSSettings(ctx)
		if err == nil {
			current, _ := s.servingNodes(ctx, sub.Imsi, settings.SMSNumber, rep.SMSFSupport)
			if names := nodeNames(current); len(names) > 0 && !sameNames(names, failed) {
				result.ServingNodes = current
			}
		}
	}

	ans, err := s6c.NewReportSMDeliveryStatusAnswer(req, id, result, rep.SMSFSupport)
	if err != nil {
		return s6c.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply})
	}

	return ans
}

func (s *SMSF) recordDeliveryReport(imsi string, rep s6c.DeliveryReport) {
	for _, o := range []*s6c.DeliveryOutcome{rep.MME, rep.SMSF3GPP, rep.SMSFNon3GPP, rep.MSC, rep.SGSN, rep.IPSMGW} {
		if o == nil {
			continue
		}

		switch o.Cause {
		case s6c.DeliveryCauseSuccessfulTransfer:
			s.clearWaiting(imsi)
			return
		case s6c.DeliveryCauseAbsentUser:
			s.markWaiting(imsi, rep.ServiceCentreAddress)
		case s6c.DeliveryCauseMemoryCapacityExceeded:
			s.mu.Lock()
			s.markMemoryFullLocked(imsi, rep.ServiceCentreAddress)
			s.mu.Unlock()
		}
	}
}

func (s *SMSF) lookup(ctx context.Context, msisdn, imsi string) (*db.Subscriber, error) {
	if msisdn != "" {
		return s.store.GetSubscriberByMSISDN(ctx, msisdn)
	}

	if imsi != "" {
		return s.store.GetSubscriber(ctx, imsi)
	}

	return nil, db.ErrNotFound
}

func (s *SMSF) registration(ctx context.Context, imsi string) *db.UERegistration {
	var active, purged *db.UERegistration

	for _, regType := range []string{db.UERegistrationTypeMME, db.UERegistrationTypeAMF3GPPAccess} {
		reg, err := s.store.GetUERegistration(ctx, imsi, regType)
		if err != nil {
			continue
		}

		switch {
		case !reg.Purged && (active == nil || reg.Version > active.Version):
			active = reg
		case reg.Purged && (purged == nil || reg.Version > purged.Version):
			purged = reg
		}
	}

	if active != nil {
		return active
	}

	return purged
}

func (s *SMSF) servingNodes(ctx context.Context, imsi, smsNumber string, smsfSupport bool) (s6c.ServingNodes, s6c.AbsentUserDiagnostics) {
	var (
		nodes  s6c.ServingNodes
		absent s6c.AbsentUserDiagnostics
	)

	reg := s.registration(ctx, imsi)
	if reg == nil {
		return nodes, absent
	}

	identity, err := s.directory.Identity(ctx, reg.NodeID)
	if err != nil {
		s.logger.Info("Serving node of a subscriber is not a cluster member", zap.String("imsi", imsi), zap.String("node_id", reg.NodeID), zap.Error(err))

		if reg.Purged {
			purged := tgpp.AbsentUserPurgedNonGPRS

			if reg.Type == db.UERegistrationTypeAMF3GPPAccess && smsfSupport {
				absent.SMSF3GPP = &purged
			} else {
				absent.MME = &purged
			}
		}

		return nodes, absent
	}

	address := &s6c.NodeAddress{Name: identity.Host, Realm: identity.Realm, Number: smsNumber}

	if reg.Type == db.UERegistrationTypeAMF3GPPAccess && smsfSupport {
		nodes.SMSF3GPP = address
	} else {
		nodes.Serving = &s6c.ServingNode{MME: address}
	}

	return nodes, absent
}

func (s *SMSF) servedElsewhere(ctx context.Context, imsi string) bool {
	reg := s.registration(ctx, imsi)
	if reg == nil || reg.Purged {
		return false
	}

	node := s.diameter.Node()
	if node == nil {
		return true
	}

	identity, err := s.directory.Identity(ctx, reg.NodeID)

	return err == nil && !strings.EqualFold(identity.Host, node.Identity().OriginHost)
}

func nodeNames(n s6c.ServingNodes) []string {
	var names []string

	for _, sn := range []*s6c.ServingNode{n.Serving, n.Additional} {
		if sn == nil {
			continue
		}

		for _, a := range []*s6c.NodeAddress{sn.MME, sn.SGSN, sn.IPSMGW} {
			if a != nil && a.Name != "" {
				names = append(names, strings.ToLower(a.Name))
			}
		}
	}

	for _, a := range []*s6c.NodeAddress{n.SMSF3GPP, n.SMSFNon3GPP} {
		if a != nil && a.Name != "" {
			names = append(names, strings.ToLower(a.Name))
		}
	}

	return names
}

func sameNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for _, name := range a {
		found := false

		for _, other := range b {
			if name == other {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}
