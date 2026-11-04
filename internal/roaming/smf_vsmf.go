// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	vsmfRequestTimeout      = 4 * time.Second
	vsmfRegisterAttempts    = 3
	vsmfRegisterBackoff     = time.Second
	sdmSubscriptionLifetime = 24 * time.Hour
	sdmRenewMargin          = time.Minute
)

type VSMFN32 interface {
	N32
	SEPPID() int
}

type HPLMNBoundSessions interface {
	ReleaseHPLMNBoundSession(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, h smf.HPLMNSubscription, how smf.HPLMNRelease) bool
	ReconcileHPLMNBoundSessions(ctx context.Context, supi etsi.SUPI)
}

type vsmfDeps struct {
	n32      VSMFN32
	home     *HPLMNSBI
	sessions HPLMNBoundSessions
}

type VSMF struct {
	logger *zap.Logger
	now    func() time.Time
	peer   func(context.Context) (sepp.Peer, bool)

	registerBackoff time.Duration

	depsMu sync.Mutex
	deps   atomic.Pointer[vsmfDeps]

	queue serial.KeyedQueue

	mu  sync.Mutex
	ues map[string]*vsmfUE
}

type smKey struct {
	sst int32
	sd  string
	dnn string
}

func (k smKey) snssai() models.Snssai {
	return models.Snssai{Sst: k.sst, Sd: k.sd}
}

func smKeyOf(snssai models.Snssai, dnn string) smKey {
	return smKey{sst: snssai.Sst, sd: models.NormalizeSD(snssai.Sd), dnn: strings.ToLower(dnn)}
}

type smData struct {
	target       HPLMNTarget
	snssai       models.Snssai
	dnn          string
	rules        smf.HPLMNRules
	subscribed   bool
	barred       bool
	subscribing  bool
	instance     string
	subscription string
	expires      time.Time
	renewal      *time.Timer
	refs         int
}

type vsmfUE struct {
	supi     etsi.SUPI
	target   HPLMNTarget
	data     map[smKey]*smData
	sessions map[uint8]*vsmfSession
}

type vsmfSession struct {
	v      *VSMF
	supi   etsi.SUPI
	psi    uint8
	key    smKey
	dnn    string
	target HPLMNTarget
	iwk    bool
	rules  smf.HPLMNRules

	closed     bool
	registered bool
	uecmRoot   string
	instanceID string
	released   bool
}

func NewVSMF(logger *zap.Logger) *VSMF {
	v := &VSMF{
		logger:          logger,
		now:             time.Now,
		peer:            sepp.PeerFrom,
		registerBackoff: vsmfRegisterBackoff,
		ues:             map[string]*vsmfUE{},
	}
	v.deps.Store(&vsmfDeps{})

	return v
}

func (v *VSMF) update(f func(d *vsmfDeps)) {
	v.depsMu.Lock()
	defer v.depsMu.Unlock()

	next := *v.deps.Load()
	f(&next)
	v.deps.Store(&next)
}

func (v *VSMF) SetN32(n32 VSMFN32) {
	v.update(func(d *vsmfDeps) {
		d.n32 = n32
		d.home = nil

		if n32 != nil {
			d.home = NewHPLMNSBI(n32, NewDiscovery(n32))
		}
	})
}

func (v *VSMF) SetSessions(sessions HPLMNBoundSessions) {
	v.update(func(d *vsmfDeps) { d.sessions = sessions })
}

func (v *VSMF) Settle() {
	v.queue.Settle()
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

	deps := v.deps.Load()

	var (
		s   *vsmfSession
		err error
	)

	v.queue.Do(imsi, func() { s, err = v.open(ctx, deps, req) })

	if err != nil || s == nil {
		return nil, err
	}

	return s, nil
}

func (v *VSMF) open(ctx context.Context, deps *vsmfDeps, req smf.HPLMNSubscriptionRequest) (*vsmfSession, error) {
	imsi := req.Supi.IMSI()
	home := req.Target.HPLMNSnssai
	target := hplmnTargetOf(req.Supi, req.Target)

	if deps.home == nil {
		return nil, vsmfUnreachable(errors.New("this node has no N32 interface"))
	}

	key := smKeyOf(home, req.Dnn)

	v.mu.Lock()
	ue := v.ueLocked(req.Supi, target)

	if d := ue.data[key]; d != nil && d.subscribed {
		if d.barred {
			v.mu.Unlock()
			return nil, vsmfBarred(home, req.Dnn)
		}

		d.refs++
		retry := d.subscription == "" && !d.subscribing
		d.subscribing = d.subscribing || retry
		s := v.newSessionLocked(ue, req, key, d.rules, d.target)
		v.mu.Unlock()

		if retry {
			ctx = context.WithoutCancel(ctx)

			v.queue.Go(imsi, func() { v.subscribe(ctx, deps, req.Supi, d) })
		}

		return s, nil
	}
	v.mu.Unlock()

	answer, err := v.fetchSMData(ctx, deps, req.Supi, target, home, req.Dnn)
	if err != nil {
		v.dropIfUnused(imsi)
		return nil, err
	}

	if answer.barred {
		v.dropIfUnused(imsi)
		return nil, vsmfBarred(home, req.Dnn)
	}

	rules, err := vsmfRules(answer.cfg, home)
	if err != nil {
		v.dropIfUnused(imsi)
		return nil, err
	}

	target.UDMInstanceID = answer.instance

	v.mu.Lock()

	d := ue.data[key]
	if d == nil {
		d = &smData{snssai: home, dnn: req.Dnn}
		ue.data[key] = d
	}

	d.target, d.rules, d.subscribed, d.barred, d.instance = target, rules, true, false, answer.instance
	d.refs++
	subscribe := d.subscription == "" && !d.subscribing
	d.subscribing = d.subscribing || subscribe
	s := v.newSessionLocked(ue, req, key, rules, target)
	v.mu.Unlock()

	if subscribe {
		ctx = context.WithoutCancel(ctx)

		v.queue.Go(imsi, func() { v.subscribe(ctx, deps, req.Supi, d) })
	}

	return s, nil
}

func vsmfBarred(snssai models.Snssai, dnn string) error {
	return newSessionError(fgs.GSMCauseOperatorDeterminedBarring, fmt.Errorf("%w: %s %q", ErrSessionBarred, wire.SnssaiFromModel(snssai), dnn))
}

func (v *VSMF) newSessionLocked(ue *vsmfUE, req smf.HPLMNSubscriptionRequest, key smKey, rules smf.HPLMNRules, target HPLMNTarget) *vsmfSession {
	psi := req.PDUSessionID

	if old := ue.sessions[psi]; old != nil && !old.closed {
		old.closed = true

		v.queue.Go(ue.supi.IMSI(), func() { v.close(old) })
	}

	s := &vsmfSession{v: v, supi: ue.supi, psi: psi, key: key, dnn: req.Dnn, target: target, iwk: req.Target.EPSInterworking, rules: rules}
	ue.sessions[psi] = s

	return s
}

func (v *VSMF) ueLocked(supi etsi.SUPI, target HPLMNTarget) *vsmfUE {
	ue := v.ues[supi.IMSI()]
	if ue == nil {
		ue = &vsmfUE{supi: supi, data: map[smKey]*smData{}, sessions: map[uint8]*vsmfSession{}}
		v.ues[supi.IMSI()] = ue
	}

	ue.target = target

	return ue
}

func (v *VSMF) dropIfUnused(imsi string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if ue := v.ues[imsi]; ue != nil && len(ue.data) == 0 && len(ue.sessions) == 0 {
		delete(v.ues, imsi)
	}
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
	v := s.v

	v.mu.Lock()
	defer v.mu.Unlock()

	if ue := v.ues[s.supi.IMSI()]; ue != nil {
		if d := ue.data[s.key]; d != nil {
			rules := d.rules
			rules.Withdrawn = !d.subscribed

			return rules
		}
	}

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
	deps := v.deps.Load()
	imsi := s.supi.IMSI()

	v.mu.Lock()
	deregister := s.registered && !s.released
	s.registered = false
	v.mu.Unlock()

	if deregister {
		v.deregister(ctx, deps, s)
	}

	v.mu.Lock()

	ue := v.ues[imsi]
	if ue == nil {
		v.mu.Unlock()
		return
	}

	if ue.sessions[s.psi] == s {
		delete(ue.sessions, s.psi)
	}

	var unsubscribe *smData

	if d := ue.data[s.key]; d != nil {
		d.refs--
		if d.refs <= 0 {
			delete(ue.data, s.key)

			if d.renewal != nil {
				d.renewal.Stop()
			}

			unsubscribe = d
		}
	}

	if len(ue.data) == 0 && len(ue.sessions) == 0 {
		delete(v.ues, imsi)
	}
	v.mu.Unlock()

	if unsubscribe != nil && unsubscribe.subscription != "" {
		v.unsubscribe(ctx, deps, unsubscribe)
	}
}

func (s *vsmfSession) Register(ctx context.Context) error {
	v := s.v
	imsi := s.supi.IMSI()

	for attempt := 1; ; attempt++ {
		v.mu.Lock()
		done := s.closed || s.registered
		v.mu.Unlock()

		if done {
			return nil
		}

		var err error

		v.queue.Do(imsi, func() { err = v.register(ctx, s) })

		var se *SessionError
		if err == nil || errors.As(err, &se) || attempt >= vsmfRegisterAttempts {
			return err
		}

		v.logger.Debug("Retrying the SMF registration at the home UDM", zap.String("imsi", imsi), zap.Uint8("pduSessionID", s.psi), zap.Error(err))

		select {
		case <-ctx.Done():
			return err
		case <-time.After(v.registerBackoff * time.Duration(attempt)):
		}
	}
}

func (v *VSMF) instanceID(serving wire.PlmnID, seppID int) string {
	return NFInstanceID(sepp.CallbackHost(sepp.CallbackSMF, seppID, serving))
}

func (v *VSMF) callbackURI(n32 VSMFN32, supi etsi.SUPI, serving wire.PlmnID, kind sepp.CallbackKind) string {
	uri, err := sepp.Callback{NF: sepp.CallbackSMF, SEPPID: n32.SEPPID(), Supi: supi.String(), Kind: kind}.URI(serving)
	if err != nil {
		return ""
	}

	return uri
}
