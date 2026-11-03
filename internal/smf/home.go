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
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

type HomeSessionRequest struct {
	Supi         etsi.SUPI
	PDUSessionID uint8
	Snssai       models.Snssai
	Dnn          string
}

type HomeSessionRules struct {
	HomeSnssai      models.Snssai
	PDUSessionTypes []fgs.PDUSessionType
}

type HomeSession interface {
	Rules() HomeSessionRules
	Register(ctx context.Context) error
	Close()
}

type HomeNetwork interface {
	OpenSession(ctx context.Context, req HomeSessionRequest) (HomeSession, error)
}

type GSMCauser interface {
	GSMCause() fgs.GSMCause
}

type HomeRelease uint8

const (
	HomeReleaseReactivate HomeRelease = iota
	HomeReleaseDeactivate
	HomeReleaseRejected
	HomeReleaseDuplicate
	HomeReleaseDuplicateNotify
)

type homeBinding struct {
	session HomeSession
}

func WithHomeNetwork(h HomeNetwork) Option { return func(s *SMF) { s.SetHomeNetwork(h) } }

func (s *SMF) SetHomeNetwork(h HomeNetwork) {
	if h == nil {
		s.home.Store(nil)
		return
	}

	s.home.Store(&homeNetwork{h})
}

type homeNetwork struct {
	HomeNetwork
}

func (s *SMF) openHomeSession(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, dnn string) (HomeSession, error) {
	h := s.home.Load()
	if h == nil || snssai == nil {
		return nil, nil
	}

	return h.OpenSession(ctx, HomeSessionRequest{Supi: supi, PDUSessionID: pduSessionID, Snssai: *snssai, Dnn: dnn})
}

func homeRejectCause(err error) fgs.GSMCause {
	var c GSMCauser
	if errors.As(err, &c) {
		return c.GSMCause()
	}

	return fgs.GSMCauseRequestRejectedUnspecified
}

func restrictToHome(policy *Policy, requested uint8, rules HomeSessionRules) *Policy {
	if len(rules.PDUSessionTypes) == 0 {
		return policy
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
	}

	return &restricted
}

func (sc *SMContext) homeSession() HomeSession {
	if b := sc.home.Load(); b != nil {
		return b.session
	}

	return nil
}

func (s *SMF) registerHomeSession(ctx context.Context, sc *SMContext) {
	h := sc.homeSession()
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

		s.releaseHomeSessionContext(ctx, sc, h, HomeReleaseRejected, c.GSMCause())
	}()
}

func (s *SMF) ReleaseHomeSession(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, h HomeSession, how HomeRelease) bool {
	sc := s.currentPDUSession(supi, pduSessionID)
	if sc == nil || sc.homeSession() != h {
		return false
	}

	cause := fgs.GSMCauseRegularDeactivation
	if how == HomeReleaseReactivate {
		cause = fgs.GSMCauseReactivationRequested
	}

	if how == HomeReleaseRejected {
		how = HomeReleaseDeactivate
	}

	s.releaseHomeSessionContext(ctx, sc, h, how, cause)

	return true
}

func (s *SMF) releaseHomeSessionContext(ctx context.Context, sc *SMContext, h HomeSession, how HomeRelease, cause fgs.GSMCause) {
	reason := models.ReleaseDuplicateSessionID

	switch how {
	case HomeReleaseReactivate:
		reason = models.ReleaseSubscriptionChanged
	case HomeReleaseDeactivate:
		reason = models.ReleaseSubscriptionWithdrawn
	case HomeReleaseRejected:
		reason = models.ReleaseEstablishmentFailure
	}

	sc.Mutex.Lock()

	if s.GetSession(sc.Ref) != sc || sc.homeSession() != h {
		sc.Mutex.Unlock()
		return
	}

	supi, pduSessionID, ref := sc.Supi, sc.PDUSessionID, sc.Ref

	if how == HomeReleaseReactivate || how == HomeReleaseDeactivate || how == HomeReleaseRejected {
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

		how = HomeReleaseDuplicateNotify
	}

	sc.stopProcedureTimer()
	sc.abandonPendingLocked()
	s.teardownAndRemove(ctx, sc, models.ReleaseFor(reason))
	sc.Mutex.Unlock()

	if how == HomeReleaseDuplicateNotify && s.amf != nil {
		s.amf.SessionDropped(ctx, supi, pduSessionID, ref, nil)
	}
}

func (s *SMF) BindHomeSessions(ctx context.Context, supi etsi.SUPI) {
	if s.home.Load() == nil {
		return
	}

	s.mu.RLock()

	var unbound []*SMContext

	for _, sc := range s.pool {
		if sc.Supi == supi && sc.homeSession() == nil {
			unbound = append(unbound, sc)
		}
	}

	s.mu.RUnlock()

	for _, sc := range unbound {
		s.bindHomeSession(ctx, sc)
	}
}

func (s *SMF) bindHomeSession(ctx context.Context, sc *SMContext) {
	sc.Mutex.Lock()
	access, pduSessionID, dnn, idle := sc.Access, sc.PDUSessionID, sc.Dnn, !sc.releasing && sc.pending == nil

	var snssai *models.Snssai
	if sc.Snssai != nil {
		snssai = new(*sc.Snssai)
	}
	sc.Mutex.Unlock()

	if access != Access5G || !idle || snssai == nil {
		return
	}

	h, err := s.openHomeSession(ctx, sc.Supi, pduSessionID, snssai, dnn)

	var refusal GSMCauser

	switch {
	case err != nil && (!errors.As(err, &refusal) || refusal.GSMCause() == fgs.GSMCauseNetworkFailure):
		logger.From(ctx, logger.SmfLog).Warn("could not reach the home network for a visitor's session that moved onto 5GS; binding it at the next registration",
			logger.SUPI(sc.Supi.String()), logger.SMContextRef(sc.Ref), logger.DNN(dnn), zap.Error(err))

		return
	case err != nil:
		logger.From(ctx, logger.SmfLog).Warn("the home network does not allow a visitor's session that moved onto 5GS; releasing it",
			logger.SUPI(sc.Supi.String()), logger.SMContextRef(sc.Ref), logger.DNN(dnn), zap.Error(err))

		s.releaseUnboundHomeSession(ctx, sc, refusal.GSMCause())

		return
	}

	if h == nil {
		return
	}

	sc.Mutex.Lock()

	if s.GetSession(sc.Ref) != sc || sc.releasing || sc.homeSession() != nil || sc.Access != Access5G {
		sc.Mutex.Unlock()
		h.Close()

		return
	}

	sc.home.Store(&homeBinding{session: h})
	sc.Mutex.Unlock()

	logger.From(ctx, logger.SmfLog).Info("bound a visitor's session that moved onto 5GS to its home network",
		logger.SUPI(sc.Supi.String()), logger.SMContextRef(sc.Ref), logger.PDUSessionID(pduSessionID), logger.DNN(dnn))

	s.registerHomeSession(ctx, sc)

	if err := s.ReconcileSession(ctx, sc.Ref); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("visitor session reconcile failed", logger.SMContextRef(sc.Ref), zap.Error(err))
	}
}

func (s *SMF) releaseUnboundHomeSession(ctx context.Context, sc *SMContext, cause fgs.GSMCause) {
	sc.Mutex.Lock()

	if s.GetSession(sc.Ref) != sc || sc.releasing {
		sc.Mutex.Unlock()
		return
	}

	supi, pduSessionID, ref := sc.Supi, sc.PDUSessionID, sc.Ref
	release := models.ReleaseFor(models.ReleaseSubscriptionWithdrawn)

	err := s.startRelease(ctx, sc, 0, cause, release)
	if err == nil {
		sc.Mutex.Unlock()
		return
	}

	logger.From(ctx, logger.SmfLog).Warn("could not signal the release of a visitor's PDU session; removing it locally",
		logger.SUPI(supi.String()), logger.SMContextRef(ref), zap.Error(err))

	sc.stopProcedureTimer()
	sc.abandonPendingLocked()
	s.teardownAndRemove(ctx, sc, release)
	sc.Mutex.Unlock()

	if s.amf != nil {
		s.amf.SessionDropped(ctx, supi, pduSessionID, ref, nil)
	}
}

func (s *SMF) ReconcileHomeSessions(ctx context.Context, supi etsi.SUPI) {
	s.mu.RLock()

	var refs []string

	for ref, sc := range s.pool {
		if sc.Supi == supi && sc.homeSession() != nil {
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
