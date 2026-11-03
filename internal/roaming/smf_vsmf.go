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
	"github.com/ellanetworks/core/internal/udm"
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

type SessionVisitor struct {
	PartnerID        string
	HomePLMN         wire.PlmnID
	ServingPLMN      wire.PlmnID
	RoutingIndicator string
	SMFSelection     *wire.SmfSelectionSubscriptionData
}

type SessionVisitors interface {
	SessionVisitor(imsi string) (SessionVisitor, bool)
}

type HomeSessionControl interface {
	ReleaseHomeSession(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, h smf.HomeSession, how smf.HomeRelease) bool
	ReconcileHomeSessions(ctx context.Context, supi etsi.SUPI)
}

type LocalSessions interface {
	SessionManagement(ctx context.Context, imsi string) (*udm.SessionManagementSubscription, error)
}

type vsmfDeps struct {
	n32      VSMFN32
	home     *HomeSBI
	visitors SessionVisitors
	sessions HomeSessionControl
}

type VSMF struct {
	local  LocalSessions
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

func smKeyOf(snssai models.Snssai, dnn string) smKey {
	return smKey{sst: snssai.Sst, sd: models.NormalizeSD(snssai.Sd), dnn: strings.ToLower(dnn)}
}

type smData struct {
	visitor      SessionVisitor
	snssai       models.Snssai
	dnn          string
	sessionAMBR  *models.Ambr
	rules        smf.HomeSessionRules
	subscribed   bool
	instance     string
	subscription string
	expires      time.Time
	renewal      *time.Timer
	refs         int
}

type vsmfUE struct {
	supi     etsi.SUPI
	visitor  SessionVisitor
	data     map[smKey]*smData
	sessions map[uint8]*vsmfSession
}

type vsmfSession struct {
	v       *VSMF
	supi    etsi.SUPI
	psi     uint8
	key     smKey
	dnn     string
	visitor SessionVisitor
	rules   smf.HomeSessionRules

	closed     bool
	registered bool
	uecmRoot   string
	released   bool
}

func NewVSMF(local LocalSessions, logger *zap.Logger) *VSMF {
	v := &VSMF{
		local:           local,
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
			d.home = NewHomeSBI(n32, NewDiscovery(n32))
		}
	})
}

func (v *VSMF) SetVisitors(visitors SessionVisitors) {
	v.update(func(d *vsmfDeps) { d.visitors = visitors })
}

func (v *VSMF) SetSessions(sessions HomeSessionControl) {
	v.update(func(d *vsmfDeps) { d.sessions = sessions })
}

func (v *VSMF) Settle() {
	v.queue.Settle()
}

func HomeSnssai(serving models.Snssai) models.Snssai {
	return models.Snssai{Sst: serving.Sst, Sd: models.NormalizeSD(serving.Sd)}
}

func (v *VSMF) OpenSession(ctx context.Context, req smf.HomeSessionRequest) (smf.HomeSession, error) {
	imsi := req.Supi.IMSI()
	if imsi == "" {
		return nil, nil
	}

	deps := v.deps.Load()
	if deps.visitors == nil {
		return nil, nil
	}

	visitor, ok := deps.visitors.SessionVisitor(imsi)
	if !ok {
		return nil, nil
	}

	var (
		s   *vsmfSession
		err error
	)

	v.queue.Do(imsi, func() { s, err = v.open(ctx, deps, req, visitor) })

	if err != nil || s == nil {
		return nil, err
	}

	return s, nil
}

func (v *VSMF) open(ctx context.Context, deps *vsmfDeps, req smf.HomeSessionRequest, visitor SessionVisitor) (*vsmfSession, error) {
	imsi := req.Supi.IMSI()

	if err := v.checkLocal(ctx, imsi, req.Snssai, req.Dnn); err != nil {
		return nil, err
	}

	home := HomeSnssai(req.Snssai)

	if err := vsmfLocalBreakout(visitor.SMFSelection, home, req.Dnn); err != nil {
		return nil, err
	}

	if deps.home == nil {
		return nil, vsmfUnreachable(errors.New("this node has no N32 interface"))
	}

	key := smKeyOf(home, req.Dnn)

	v.mu.Lock()
	ue := v.ueLocked(req.Supi, visitor)

	if d := ue.data[key]; d != nil && d.subscribed {
		d.refs++
		s := v.newSessionLocked(ue, req.PDUSessionID, key, req.Dnn, d.rules)
		v.mu.Unlock()

		return s, nil
	}
	v.mu.Unlock()

	cfg, instance, err := v.fetchSMData(ctx, deps, req.Supi, visitor, home, req.Dnn)
	if err != nil {
		v.dropIfUnused(imsi)
		return nil, err
	}

	ambr, rules, err := vsmfRules(cfg, home)
	if err != nil {
		v.dropIfUnused(imsi)
		return nil, err
	}

	v.mu.Lock()

	d := ue.data[key]
	if d == nil {
		d = &smData{snssai: home, dnn: req.Dnn}
		ue.data[key] = d
	}

	d.visitor, d.sessionAMBR, d.rules, d.subscribed, d.instance = visitor, ambr, rules, true, instance
	d.refs++
	subscribe := d.subscription == ""
	s := v.newSessionLocked(ue, req.PDUSessionID, key, req.Dnn, rules)
	v.mu.Unlock()

	if subscribe {
		ctx = context.WithoutCancel(ctx)

		v.queue.Go(imsi, func() { v.subscribe(ctx, deps, req.Supi, d) })
	}

	return s, nil
}

func (v *VSMF) newSessionLocked(ue *vsmfUE, psi uint8, key smKey, dnn string, rules smf.HomeSessionRules) *vsmfSession {
	if old := ue.sessions[psi]; old != nil && !old.closed {
		old.closed = true

		v.queue.Go(ue.supi.IMSI(), func() { v.close(old) })
	}

	s := &vsmfSession{v: v, supi: ue.supi, psi: psi, key: key, dnn: dnn, visitor: ue.visitor, rules: rules}
	ue.sessions[psi] = s

	return s
}

func (v *VSMF) ueLocked(supi etsi.SUPI, visitor SessionVisitor) *vsmfUE {
	ue := v.ues[supi.IMSI()]
	if ue == nil {
		ue = &vsmfUE{supi: supi, data: map[smKey]*smData{}, sessions: map[uint8]*vsmfSession{}}
		v.ues[supi.IMSI()] = ue
	}

	ue.visitor = visitor

	return ue
}

func (v *VSMF) dropIfUnused(imsi string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if ue := v.ues[imsi]; ue != nil && len(ue.data) == 0 && len(ue.sessions) == 0 {
		delete(v.ues, imsi)
	}
}

func (v *VSMF) checkLocal(ctx context.Context, imsi string, snssai models.Snssai, dnn string) error {
	sm, err := v.local.SessionManagement(ctx, imsi)
	if err != nil {
		return newSessionError(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("local subscription of the visitor: %w", err))
	}

	if !sm.Visitor {
		return newSessionError(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("%s is not a visitor", imsi))
	}

	c, ok := sm.ForDNN(snssai, dnn)

	switch {
	case ok && !c.IMS:
		return nil
	case ok || !sm.Subscribes(snssai):
		return newSessionError(fgs.GSMCauseMissingOrUnknownDNN, fmt.Errorf("%w: %q", ErrSessionNotLocalDN, dnn))
	default:
		return newSessionError(fgs.GSMCauseMissingOrUnknownDNNInASlice, fmt.Errorf("%w: %q on sst=%d sd=%q", ErrSessionNotLocalDN, dnn, snssai.Sst, snssai.Sd))
	}
}

func vsmfLocalBreakout(sel *wire.SmfSelectionSubscriptionData, home models.Snssai, dnn string) error {
	if sel == nil {
		return newSessionError(fgs.GSMCauseRequestedServiceOptionNotSubscribed, fmt.Errorf("%w: no SMF selection subscription data for the visitor", ErrSessionHomeRouted))
	}

	info, ok := sel.SnssaiInfo(wire.SnssaiFromModel(home))
	if !ok {
		return newSessionError(fgs.GSMCauseRequestedServiceOptionNotSubscribed, fmt.Errorf("%w: S-NSSAI %s", ErrSessionNotSubscribed, wire.SnssaiFromModel(home)))
	}

	d, ok := info.Dnn(dnn)

	switch {
	case !ok || d.DnnBarred:
		return newSessionError(fgs.GSMCauseRequestedServiceOptionNotSubscribed, fmt.Errorf("%w: DNN %q", ErrSessionNotSubscribed, dnn))
	case !d.LboRoamingAllowed:
		return newSessionError(fgs.GSMCauseRequestedServiceOptionNotSubscribed, fmt.Errorf("%w: DNN %q", ErrSessionHomeRouted, dnn))
	default:
		return nil
	}
}

var vsmfPDUSessionTypes = map[wire.PduSessionType]fgs.PDUSessionType{
	wire.PduSessionIPv4:   fgs.PDUSessionTypeIPv4,
	wire.PduSessionIPv6:   fgs.PDUSessionTypeIPv6,
	wire.PduSessionIPv4v6: fgs.PDUSessionTypeIPv4v6,
}

func vsmfRules(cfg wire.DnnConfiguration, home models.Snssai) (*models.Ambr, smf.HomeSessionRules, error) {
	rules := smf.HomeSessionRules{HomeSnssai: home}

	if !vsmfSSCMode1(cfg.SscModes) {
		return nil, rules, newSessionError(fgs.GSMCauseNotSupportedSSCMode, ErrSessionSSCMode)
	}

	if cfg.PduSessionTypes != nil {
		allowed := append([]wire.PduSessionType{cfg.PduSessionTypes.DefaultSessionType}, cfg.PduSessionTypes.AllowedSessionTypes...)

		for _, t := range allowed {
			if ft, ok := vsmfPDUSessionTypes[t]; ok && !slices.Contains(rules.PDUSessionTypes, ft) {
				rules.PDUSessionTypes = append(rules.PDUSessionTypes, ft)
			}
		}

		if len(rules.PDUSessionTypes) == 0 {
			return nil, rules, newSessionError(fgs.GSMCauseUnknownPDUSessionType, ErrSessionNotIP)
		}
	}

	if cfg.SessionAmbr == nil {
		return nil, rules, nil
	}

	ambr, err := cfg.SessionAmbr.Model()
	if err != nil {
		return nil, rules, newSessionError(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("%w: Session-AMBR: %w", ErrSessionHomeData, err))
	}

	return &ambr, rules, nil
}

func vsmfSSCMode1(m *wire.SscModes) bool {
	return m == nil || m.DefaultSscMode == wire.SscMode1 || slices.Contains(m.AllowedSscModes, wire.SscMode1)
}

func (s *vsmfSession) Rules() smf.HomeSessionRules {
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
