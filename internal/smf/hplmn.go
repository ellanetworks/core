// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"slices"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

var (
	ErrHPLMNTargetMissing = errors.New("a roamer's session arrived without its home network target")
	ErrHomeRoutedSession  = errors.New("home-routed roaming is not supported")
	ErrNoHPLMNClient      = errors.New("this SMF has no client toward home networks")
	ErrHPLMNWithdrawn     = errors.New("the home network withdrew the subscription")
)

type HPLMNTarget struct {
	PartnerID        string
	HPLMN            models.PlmnID
	ServingNetwork   models.PlmnID
	RoutingIndicator string
	UDMGroupID       string
	HPLMNSnssai      models.Snssai
	LBO              bool
	EPSInterworking  bool
}

type HPLMNSubscriptionRequest struct {
	Supi         etsi.SUPI
	PDUSessionID uint8
	Dnn          string
	Target       HPLMNTarget
}

type HPLMNRules struct {
	HPLMNSnssai     *models.Snssai
	PDUSessionTypes []fgs.PDUSessionType
	SessionAMBR     *models.Ambr
	Withdrawn       bool
}

type HPLMNSubscription interface {
	Rules() HPLMNRules
	Register(ctx context.Context) error
	Close()
}

type HPLMNClient interface {
	OpenSubscription(ctx context.Context, req HPLMNSubscriptionRequest) (HPLMNSubscription, error)
}

type GSMCauser interface {
	GSMCause() fgs.GSMCause
}

type HPLMNRelease uint8

const (
	HPLMNReleaseReactivate HPLMNRelease = iota
	HPLMNReleaseDeactivate
	HPLMNReleaseRejected
	HPLMNReleaseDuplicate
	HPLMNReleaseDuplicateNotify
)

type hplmnBinding struct {
	session HPLMNSubscription
}

func WithHPLMNClient(h HPLMNClient) Option { return func(s *SMF) { s.hplmn = h } }

type hplmnRefusal struct {
	cause fgs.GSMCause
	err   error
}

func (r *hplmnRefusal) Error() string          { return r.err.Error() }
func (r *hplmnRefusal) Unwrap() error          { return r.err }
func (r *hplmnRefusal) GSMCause() fgs.GSMCause { return r.cause }

func (s *SMF) openHPLMNSubscription(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, visitor bool, target *HPLMNTarget) (HPLMNSubscription, error) {
	if !visitor {
		return nil, nil
	}

	switch {
	case target == nil:
		return nil, &hplmnRefusal{cause: fgs.GSMCauseRequestRejectedUnspecified, err: ErrHPLMNTargetMissing}
	case !target.LBO:
		return nil, &hplmnRefusal{cause: fgs.GSMCauseRequestedServiceOptionNotSubscribed, err: ErrHomeRoutedSession}
	}

	if s.hplmn == nil {
		return nil, &hplmnRefusal{cause: fgs.GSMCauseNetworkFailure, err: ErrNoHPLMNClient}
	}

	return s.hplmn.OpenSubscription(ctx, HPLMNSubscriptionRequest{Supi: supi, PDUSessionID: pduSessionID, Dnn: dnn, Target: *target})
}

func (r *HPLMNRules) capQoS(q SubscribedQoS) SubscribedQoS {
	if r != nil && r.SessionAMBR != nil {
		q.SessionAMBR = q.SessionAMBR.Min(*r.SessionAMBR)
	}

	return q
}

func apnRules(sub *models.APNSubscription) HPLMNRules {
	rules := HPLMNRules{SessionAMBR: sub.APNAMBR}

	for _, t := range sub.PDNTypes {
		switch t {
		case eps.PDNTypeIPv4:
			rules.PDUSessionTypes = append(rules.PDUSessionTypes, fgs.PDUSessionTypeIPv4)
		case eps.PDNTypeIPv6:
			rules.PDUSessionTypes = append(rules.PDUSessionTypes, fgs.PDUSessionTypeIPv6)
		case eps.PDNTypeIPv4v6:
			rules.PDUSessionTypes = append(rules.PDUSessionTypes, fgs.PDUSessionTypeIPv4v6)
		}
	}

	return rules
}

func (sc *SMContext) hplmnRulesLocked() *HPLMNRules {
	h := sc.hplmnSubscription()

	if sc.apnSubscription != nil && (sc.Access == Access4G || h == nil) {
		rules := apnRules(sc.apnSubscription)
		return &rules
	}

	if h != nil {
		rules := h.Rules()
		return &rules
	}

	return nil
}

func (sc *SMContext) hplmnSnssaiLocked() *models.Snssai {
	if h := sc.hplmnSubscription(); h != nil {
		return h.Rules().HPLMNSnssai
	}

	return nil
}

func (s *SMF) UpdateAPNSubscription(ctx context.Context, ref string, sub *models.APNSubscription) {
	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	sc.Mutex.Lock()
	sc.apnSubscription, sc.apnWithdrawn = sub, sub == nil
	sc.Mutex.Unlock()

	logger.From(ctx, logger.SmfLog).Debug("updated the home APN subscription of a visitor's PDN connection", logger.SMContextRef(ref))
}

func (s *SMF) hplmnSubscriptionForTransfer(ctx context.Context, sc *SMContext, visitor bool, target *HPLMNTarget) (HPLMNSubscription, error) {
	if !visitor || sc.hplmnSubscription() != nil {
		return nil, nil
	}

	sc.Mutex.Lock()
	supi, pduSessionID, dnn := sc.Supi, sc.PDUSessionID, sc.Dnn
	sc.Mutex.Unlock()

	return s.openHPLMNSubscription(ctx, supi, pduSessionID, dnn, true, target)
}

func (s *SMF) ReleaseBarredSession(ctx context.Context, ref string) error {
	sc := s.GetSession(ref)
	if sc == nil {
		return nil
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if s.GetSession(ref) != sc || sc.releasing {
		return nil
	}

	return s.startRelease(ctx, sc, 0, fgs.GSMCauseOperatorDeterminedBarring, models.ReleaseFor(models.ReleaseSubscriptionChanged))
}

func closeHPLMNSubscription(h HPLMNSubscription) {
	if h != nil {
		h.Close()
	}
}

func (s *SMF) adoptHPLMNSubscription(ctx context.Context, sc *SMContext, h HPLMNSubscription) {
	if h == nil {
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if s.GetSession(sc.Ref) != sc || sc.releasing || sc.hplmnSubscription() != nil || sc.pending == nil || sc.pending.to != Access5G {
		h.Close()
		return
	}

	sc.hplmn.Store(&hplmnBinding{session: h})
	sc.pending.hplmn = h

	logger.From(ctx, logger.SmfLog).Info("bound a visitor's session that moves onto 5GS to its home network",
		logger.SUPI(sc.Supi.String()), logger.SMContextRef(sc.Ref), logger.PDUSessionID(sc.PDUSessionID), logger.DNN(sc.Dnn))
}

func (sc *SMContext) unbindAbandonedMoveLocked(move *pendingTransfer) {
	if move == nil || move.hplmn == nil || sc.hplmnSubscription() != move.hplmn {
		return
	}

	sc.hplmn.Store(nil)
	move.hplmn.Close()

	logger.SmfLog.Info("unbound a visitor's session from its home network after its move onto 5GS was abandoned",
		logger.SUPI(sc.Supi.String()), logger.SMContextRef(sc.Ref), logger.PDUSessionID(sc.PDUSessionID), logger.DNN(sc.Dnn))
}

func hplmnRejectCause(err error) fgs.GSMCause {
	var c GSMCauser
	if errors.As(err, &c) {
		return c.GSMCause()
	}

	return fgs.GSMCauseRequestRejectedUnspecified
}

func restrictToHPLMN(policy *Policy, requested uint8, rules *HPLMNRules) (*Policy, bool) {
	if rules == nil || len(rules.PDUSessionTypes) == 0 {
		return policy, false
	}

	allows := func(t fgs.PDUSessionType) bool { return slices.Contains(rules.PDUSessionTypes, t) }
	dual := allows(fgs.PDUSessionTypeIPv4v6)

	restricted := *policy
	if !dual && !allows(fgs.PDUSessionTypeIPv4) {
		restricted.IPv4Pool = ""
	}

	if !dual && !allows(fgs.PDUSessionTypeIPv6) {
		restricted.IPv6Pool = ""
	}

	if requested == uint8(fgs.PDUSessionTypeIPv4v6) && !dual && restricted.IPv4Pool != "" && restricted.IPv6Pool != "" {
		restricted.IPv6Pool = ""
		return &restricted, true
	}

	return &restricted, false
}

func (sc *SMContext) hplmnSubscription() HPLMNSubscription {
	if b := sc.hplmn.Load(); b != nil {
		return b.session
	}

	return nil
}

func (s *SMF) registerHPLMNSubscription(ctx context.Context, sc *SMContext) {
	h := sc.hplmnSubscription()
	if h == nil {
		return
	}

	ctx = context.WithoutCancel(ctx)

	go func() {
		err := h.Register(ctx)
		if err == nil {
			return
		}

		var c GSMCauser
		if !errors.As(err, &c) {
			logger.From(ctx, logger.SmfLog).Warn("could not register the visitor's PDU session with the home UDM",
				logger.SUPI(sc.Supi.String()), logger.SMContextRef(sc.Ref), zap.Error(err))

			return
		}

		logger.From(ctx, logger.SmfLog).Warn("the home UDM refused the visitor's PDU session registration; releasing the session",
			logger.SUPI(sc.Supi.String()), logger.SMContextRef(sc.Ref), zap.Error(err))

		s.releaseHPLMNBoundSession(ctx, sc, h, HPLMNReleaseRejected, c.GSMCause())
	}()
}

func (s *SMF) ReleaseHPLMNBoundSession(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, h HPLMNSubscription, how HPLMNRelease) bool {
	sc := s.currentPDUSession(supi, pduSessionID)
	if sc == nil || sc.hplmnSubscription() != h {
		return false
	}

	cause := fgs.GSMCauseRegularDeactivation
	if how == HPLMNReleaseReactivate {
		cause = fgs.GSMCauseReactivationRequested
	}

	if how == HPLMNReleaseRejected {
		how = HPLMNReleaseDeactivate
	}

	s.releaseHPLMNBoundSession(ctx, sc, h, how, cause)

	return true
}

func (s *SMF) releaseHPLMNBoundSession(ctx context.Context, sc *SMContext, h HPLMNSubscription, how HPLMNRelease, cause fgs.GSMCause) {
	reason := models.ReleaseDuplicateSessionID

	switch how {
	case HPLMNReleaseReactivate:
		reason = models.ReleaseSubscriptionChanged
	case HPLMNReleaseDeactivate:
		reason = models.ReleaseSubscriptionWithdrawn
	case HPLMNReleaseRejected:
		reason = models.ReleaseEstablishmentFailure
	}

	sc.Mutex.Lock()

	if s.GetSession(sc.Ref) != sc || sc.hplmnSubscription() != h {
		sc.Mutex.Unlock()
		return
	}

	supi, pduSessionID, ref := sc.Supi, sc.PDUSessionID, sc.Ref

	if how == HPLMNReleaseReactivate || how == HPLMNReleaseDeactivate || how == HPLMNReleaseRejected {
		if sc.releasing {
			sc.Mutex.Unlock()
			return
		}

		err := s.startRelease(ctx, sc, 0, cause, models.ReleaseFor(reason))
		if err == nil {
			sc.Mutex.Unlock()
			return
		}

		logger.From(ctx, logger.SmfLog).Warn("could not signal the release of a visitor's PDU session; removing it locally",
			logger.SUPI(supi.String()), logger.SMContextRef(ref), zap.Error(err))

		how = HPLMNReleaseDuplicateNotify
	}

	sc.stopProcedureTimer()
	sc.abandonPendingLocked()
	s.teardownAndRemove(ctx, sc, models.ReleaseFor(reason))
	sc.Mutex.Unlock()

	if how == HPLMNReleaseDuplicateNotify && s.amf != nil {
		s.amf.SessionDropped(ctx, supi, pduSessionID, ref, nil)
	}
}

func (s *SMF) ReconcileHPLMNBoundSessions(ctx context.Context, supi etsi.SUPI) {
	s.mu.RLock()

	var refs []string

	for ref, sc := range s.pool {
		if sc.Supi == supi && sc.hplmnSubscription() != nil {
			refs = append(refs, ref)
		}
	}

	s.mu.RUnlock()

	for _, ref := range refs {
		if err := s.ReconcileSession(ctx, ref); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("visitor session reconcile failed", logger.SMContextRef(ref), zap.Error(err))
		}
	}
}
