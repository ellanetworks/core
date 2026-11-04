// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"github.com/ellanetworks/core/internal/serial"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

const (
	vsmfRequestTimeout = 4 * time.Second
	sdmRenewBefore     = 10 * time.Minute
	sdmRenewLifetime   = 24 * time.Hour
)

type HPLMNBoundSessions interface {
	ReleaseHPLMNBoundSession(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, h smf.HPLMNSubscription, how smf.HPLMNRelease) bool
	ReconcileHPLMNBoundSessions(ctx context.Context, supi etsi.SUPI)
}

type VSMF struct {
	logger *zap.Logger
	now    func() time.Time
	peer   func(context.Context) (sepp.Peer, bool)

	n32      N32
	home     *HPLMNSBI
	sessions HPLMNBoundSessions

	queue serial.KeyedQueue

	mu  sync.Mutex
	ues map[string]map[uint8]*vsmfSession
}

type vsmfSession struct {
	v      *VSMF
	supi   etsi.SUPI
	psi    uint8
	snssai models.Snssai
	dnn    string
	target HPLMNTarget
	iwk    bool

	rules        smf.HPLMNRules
	closed       bool
	tried        bool
	registered   bool
	released     bool
	uecmRoot     string
	instanceID   string
	subscription string
	expires      time.Time
}

func NewVSMF(logger *zap.Logger) *VSMF {
	return &VSMF{
		logger: logger,
		now:    time.Now,
		peer:   sepp.PeerFrom,
		ues:    map[string]map[uint8]*vsmfSession{},
	}
}

func (v *VSMF) SetN32(n32 N32) {
	v.n32 = n32
	v.home = NewHPLMNSBI(n32, NewDiscovery(n32))
}

func (v *VSMF) SetSessions(sessions HPLMNBoundSessions) {
	v.sessions = sessions
}

func hplmnTargetOf(supi etsi.SUPI, t smf.HPLMNTarget) HPLMNTarget {
	return HPLMNTarget{
		PartnerID:        t.PartnerID,
		PLMN:             wire.PlmnIDFromModel(t.HPLMN),
		Serving:          wire.PlmnIDFromModel(t.ServingNetwork),
		RoutingIndicator: t.RoutingIndicator,
		Supi:             supi.String(),
		UDMGroupID:       t.UDMGroupID,
	}
}

func (v *VSMF) OpenSubscription(ctx context.Context, req smf.HPLMNSubscriptionRequest) (smf.HPLMNSubscription, error) {
	imsi := req.Supi.IMSI()
	if imsi == "" {
		return nil, newSessionError(fgs.GSMCauseRequestRejectedUnspecified, errors.New("a visitor's SUPI is not an IMSI"))
	}

	if v.home == nil {
		return nil, vsmfUnreachable(errors.New("this node has no N32 interface"))
	}

	home := req.Target.HPLMNSnssai
	target := hplmnTargetOf(req.Supi, req.Target)

	answer, err := v.fetchSMData(ctx, req.Supi, target, home, req.Dnn)
	if err != nil {
		return nil, err
	}

	if answer.barred {
		return nil, vsmfBarred(home, req.Dnn)
	}

	rules, err := vsmfRules(answer.cfg, home)
	if err != nil {
		return nil, err
	}

	target.UDMInstanceID = answer.instance

	s := &vsmfSession{v: v, supi: req.Supi, psi: req.PDUSessionID, snssai: home, dnn: req.Dnn, target: target, iwk: req.Target.EPSInterworking, rules: rules}

	v.mu.Lock()

	ue := v.ues[imsi]
	if ue == nil {
		ue = map[uint8]*vsmfSession{}
		v.ues[imsi] = ue
	}

	old := ue[s.psi]
	if old != nil && !old.closed {
		old.closed = true
	} else {
		old = nil
	}

	ue[s.psi] = s
	v.mu.Unlock()

	if old != nil {
		v.queue.Go(imsi, func() { v.close(old) })
	}

	return s, nil
}

func vsmfBarred(snssai models.Snssai, dnn string) error {
	return newSessionError(fgs.GSMCauseOperatorDeterminedBarring, fmt.Errorf("%w: %s %q", ErrSessionBarred, wire.SnssaiFromModel(snssai), dnn))
}

var vsmfPDUSessionTypes = map[wire.PduSessionType]fgs.PDUSessionType{
	wire.PduSessionIPv4:   fgs.PDUSessionTypeIPv4,
	wire.PduSessionIPv6:   fgs.PDUSessionTypeIPv6,
	wire.PduSessionIPv4v6: fgs.PDUSessionTypeIPv4v6,
}

func vsmfRules(cfg wire.DnnConfiguration, home models.Snssai) (smf.HPLMNRules, error) {
	rules := smf.HPLMNRules{HPLMNSnssai: &home}

	if !vsmfSSCMode1(cfg.SscModes) {
		return rules, newSessionError(fgs.GSMCauseNotSupportedSSCMode, ErrSessionSSCMode)
	}

	if cfg.PduSessionTypes != nil {
		allowed := append([]wire.PduSessionType{cfg.PduSessionTypes.DefaultSessionType}, cfg.PduSessionTypes.AllowedSessionTypes...)

		for _, t := range allowed {
			if ft, ok := vsmfPDUSessionTypes[t]; ok && !slices.Contains(rules.PDUSessionTypes, ft) {
				rules.PDUSessionTypes = append(rules.PDUSessionTypes, ft)
			}
		}

		if len(rules.PDUSessionTypes) == 0 {
			return rules, newSessionError(fgs.GSMCauseUnknownPDUSessionType, ErrSessionNotIP)
		}
	}

	if cfg.SessionAmbr == nil {
		return rules, nil
	}

	ambr, err := cfg.SessionAmbr.Model()
	if err != nil {
		return rules, newSessionError(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("%w: Session-AMBR: %w", ErrSessionHPLMNData, err))
	}

	rules.SessionAMBR = &ambr

	return rules, nil
}

func vsmfSSCMode1(m *wire.SscModes) bool {
	return m == nil || m.DefaultSscMode == wire.SscMode1 || slices.Contains(m.AllowedSscModes, wire.SscMode1)
}

func (s *vsmfSession) Rules() smf.HPLMNRules {
	s.v.mu.Lock()
	defer s.v.mu.Unlock()

	return s.rules
}

func (s *vsmfSession) Close() {
	v := s.v

	v.mu.Lock()
	if s.closed {
		v.mu.Unlock()
		return
	}

	s.closed = true
	v.mu.Unlock()

	v.queue.Go(s.supi.IMSI(), func() { v.close(s) })
}

func (v *VSMF) close(s *vsmfSession) {
	ctx := context.Background()
	imsi := s.supi.IMSI()

	v.mu.Lock()
	deregister := s.registered && !s.released
	unsubscribe := s.subscription
	s.registered, s.subscription = false, ""

	if ue := v.ues[imsi]; ue != nil && ue[s.psi] == s {
		delete(ue, s.psi)

		if len(ue) == 0 {
			delete(v.ues, imsi)
		}
	}
	v.mu.Unlock()

	if deregister {
		v.deregister(ctx, s)
	}

	if unsubscribe != "" {
		v.unsubscribe(ctx, s, unsubscribe)
	}
}

func (s *vsmfSession) Register(ctx context.Context) error {
	var err error

	s.v.queue.Do(s.supi.IMSI(), func() { err = s.v.register(ctx, s) })

	return err
}

func (v *VSMF) RenewSubscriptions(ctx context.Context) {
	now := v.now()

	v.mu.Lock()

	var unregistered, due []*vsmfSession

	for _, ue := range v.ues {
		for _, s := range ue {
			switch {
			case s.closed:
			case !s.registered && s.tried:
				unregistered = append(unregistered, s)
			case s.registered && (s.subscription == "" || renewDue(s.expires, now)):
				due = append(due, s)
			}
		}
	}
	v.mu.Unlock()

	ctx = context.WithoutCancel(ctx)

	for _, s := range unregistered {
		v.queue.Go(s.supi.IMSI(), func() { v.reregister(ctx, s) })
	}

	for _, s := range due {
		v.queue.Go(s.supi.IMSI(), func() { v.resubscribe(ctx, s) })
	}
}

func (v *VSMF) reregister(ctx context.Context, s *vsmfSession) {
	err := v.register(ctx, s)
	if err == nil {
		return
	}

	log := v.logger.With(zap.String("supi", s.supi.String()), zap.Uint8("pduSessionID", s.psi), zap.Error(err))

	var c smf.GSMCauser
	if !errors.As(err, &c) {
		log.Warn("Could not register the visitor's PDU session at the home UDM; retrying at the next reconcile")
		return
	}

	log.Warn("The home UDM refused the visitor's PDU session registration; releasing the session")

	if v.sessions != nil {
		go v.sessions.ReleaseHPLMNBoundSession(ctx, s.supi, s.psi, s, smf.HPLMNReleaseRejected)
	}
}

func renewDue(expires, now time.Time) bool {
	return !expires.IsZero() && expires.Sub(now) < sdmRenewBefore
}

func (v *VSMF) instanceID(serving wire.PlmnID) string {
	return NFInstanceID(sepp.CallbackHost(sepp.CallbackSMF, v.n32.SEPPID(), serving))
}

func (v *VSMF) callbackURI(supi etsi.SUPI, serving wire.PlmnID, kind sepp.CallbackKind) string {
	uri, err := sepp.Callback{NF: sepp.CallbackSMF, SEPPID: v.n32.SEPPID(), Supi: supi.String(), Kind: kind}.URI(serving)
	if err != nil {
		return ""
	}

	return uri
}
