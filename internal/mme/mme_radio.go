// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// Radio is the MME's mutable per-eNB record. lastSeen (Unix nanoseconds) is read
// concurrently on the inbound S1AP hot path, so it is atomic; name and id are
// guarded by MME.mu; the remaining fields are immutable after the eNB associates.
type Radio struct {
	// Conn is the send target for node-level (non-UE) S1AP.
	Conn S1APWriter
	m    *MME
	name string
	// id is the Global eNB ID, empty until claimed on S1 Setup accept. Empty id also
	// gates the dispatcher's setup-first check: pre-setup UE signalling is dropped
	// (TS 36.413). Guarded by MME.mu.
	id             string
	address        string
	connectedAt    time.Time
	disconnectedAt time.Time
	lastSeen       atomic.Int64
	// supportedTAIs are the TAIs the eNB broadcasts (Supported TAs IE), claimed on
	// accept and replaced wholesale on an eNB Configuration Update (TS 36.413
	// §8.7.3.2, §8.7.4). Guarded by MME.mu.
	supportedTAIs []SupportedTAI
	// Log carries the eNB's RAN address for node-level correlation. Keyed by the
	// immutable SCTP address, so it never goes stale.
	Log *zap.Logger

	advertisedCapacity      *uint8
	retryNotBefore          time.Time
	configUpdateOutstanding bool
	configUpdateGuard       guard.Guard
}

// SupportedTAI is a Tracking Area Identity an eNB broadcasts: a served PLMN paired
// with a cell's TAC (TS 36.413 §8.7.3.2). The S-NSSAI list is a 5G-only field, so
// a 4G eNB's TAI omits it.
type SupportedTAI struct {
	Tai models.Tai
}

type RadioInfo struct {
	Name           string
	ID             string
	Address        string
	Connected      bool
	ConnectedAt    time.Time
	LastSeenAt     time.Time
	DisconnectedAt time.Time
	SupportedTAIs  []SupportedTAI
}

var (
	ErrRadioNotFound = errors.New("radio not found")
	ErrRadioOnline   = errors.New("radio is online")
)

const (
	DefaultRadioOfflineTTL  = 24 * time.Hour
	DefaultMaxOfflineRadios = 100
)

func (r *Radio) connected() bool {
	return r.disconnectedAt.IsZero()
}

func (r *Radio) IDKey() (string, bool) {
	return r.id, r.id != ""
}

func (r *Radio) DisconnectedAt() time.Time {
	return r.disconnectedAt
}

func (r *Radio) SetDisconnectedAt(t time.Time) {
	r.disconnectedAt = t
}

func (r *Radio) info() RadioInfo {
	return RadioInfo{
		Name:           r.name,
		ID:             r.id,
		Address:        r.address,
		Connected:      r.connected(),
		ConnectedAt:    r.connectedAt,
		LastSeenAt:     time.Unix(0, r.lastSeen.Load()),
		DisconnectedAt: r.disconnectedAt,
		SupportedTAIs:  r.supportedTAIs,
	}
}

// EnbSupportedTAIs flattens an S1 Setup Request's Supported TAs into the TAIs the
// eNB broadcasts: one entry per (broadcast PLMN, TAC) pair (TS 36.413 §8.7.3.2).
func EnbSupportedTAIs(tas s1ap.SupportedTAs) []SupportedTAI {
	out := make([]SupportedTAI, 0, len(tas))
	for _, ta := range tas {
		// TS 23.003: the 16-bit LTE TAC is the two least-significant octets of the
		// 6-hex-digit TAC, matching how gNB TAIs render theirs.
		tac := fmt.Sprintf("%06x", uint16(ta.TAC))
		for _, plmn := range ta.BroadcastPLMNs {
			p := decodePLMN(plmn)
			out = append(out, SupportedTAI{Tai: models.Tai{PlmnID: &p, Tac: tac}})
		}
	}

	return out
}

// ENBID renders a Global eNB ID as "<plmn>-<enb-id>" for display.
func ENBID(g s1ap.GlobalENBID) string {
	p := g.PLMNIdentity

	return fmt.Sprintf("%02x%02x%02x-%x", p[0], p[1], p[2], g.ENBID.Value)
}

// trackRadio records a connected eNB keyed by its SCTP association.
func (m *MME) trackRadio(key *sctp.SCTPConn, info RadioInfo) {
	s := &Radio{Conn: key, m: m, name: info.Name, id: info.ID, address: info.Address, connectedAt: info.ConnectedAt, supportedTAIs: info.SupportedTAIs}
	s.lastSeen.Store(info.LastSeenAt.UnixNano())
	s.Log = logger.MmeLog.With(logger.RanAddr(info.Address))

	m.mu.Lock()
	defer m.mu.Unlock()

	m.reg.Track(key, s)
}

// addRadio records a connected eNB from its S1 Setup Request, carrying only the
// node-level logging identity (name, address). The Global eNB ID and broadcast TAIs
// are claimed on accept (TS 36.413).
func (m *MME) addRadio(conn *sctp.SCTPConn, req *s1ap.S1SetupRequest) {
	address := ""
	if a := conn.RemoteAddr(); a != nil {
		address = a.String()
	}

	name := ""
	if req.ENBName != nil {
		name = *req.ENBName
	}

	now := time.Now()
	m.trackRadio(conn, RadioInfo{
		Name:        name,
		Address:     address,
		ConnectedAt: now,
		LastSeenAt:  now,
	})
}

// TrackRadioFromSetup records the eNB from an S1 Setup Request's raw value. A parse
// failure is reported by the S1 Setup handler, so it is dropped here.
func (m *MME) TrackRadioFromSetup(conn *sctp.SCTPConn, value []byte) {
	req, err := s1ap.ParseS1SetupRequest(value)
	if err != nil {
		return
	}

	m.addRadio(conn, req)
}

// RadioLog returns a node-scoped logger carrying the eNB's RAN address. Before S1
// Setup (no tracked eNB) it falls back to a logger built from the connection's
// remote address, so node-level events are attributed to the RAN address
// throughout the association.
func (m *MME) RadioLog(conn S1APWriter) *zap.Logger {
	sc, _ := conn.(*sctp.SCTPConn)

	m.mu.RLock()
	s, _ := m.reg.Radio(conn)
	m.mu.RUnlock()

	return nodeLog(s, sc)
}

// nodeLogLocked is RadioLog for callers already holding MME.mu, avoiding a re-lock.
func (m *MME) nodeLogLocked(conn S1APWriter) *zap.Logger {
	sc, _ := conn.(*sctp.SCTPConn)

	radio, _ := m.reg.Radio(conn)

	return nodeLog(radio, sc)
}

func nodeLog(s *Radio, conn *sctp.SCTPConn) *zap.Logger {
	if s != nil && s.Log != nil {
		return s.Log
	}

	if conn != nil {
		return logger.MmeLog.With(logger.RanAddr(AddrString(conn.RemoteAddr())))
	}

	return logger.MmeLog
}

// ClaimENBID assigns the eNB's Global eNB ID on S1 Setup accept and indexes the
// association by ID so an S1 handover can resolve a HANDOVER REQUIRED's target
// (TS 36.413 §8.4.1). When a re-associating eNB claims an ID still held by a different
// live association, the stale one is evicted and torn down so the ID resolves to the
// current association and a handover cannot target a dead eNB.
//
// UE contexts are released on both the stale association and radio itself: S1
// Setup re-initialises the S1AP UE-related contexts unless the two nodes agree to
// retain them (TS 36.413 §8.7.3.1), and Ella Core never offers UE retention. An
// eNB repeating S1 Setup on its existing association — what an SCTP restart
// produces — would otherwise keep UEs the eNB has already forgotten.
func (m *MME) ClaimENBID(radio *Radio, g s1ap.GlobalENBID, advertisedCapacity uint8) {
	id := ENBID(g)

	m.mu.Lock()

	radio.id = id
	radio.advertisedCapacity = &advertisedCapacity

	var stale S1APWriter

	if prev := m.reg.Claim(id, radio); prev != nil && prev.Conn != radio.Conn && prev.connected() {
		stale = prev.Conn

		delete(m.reg.ByConn, prev.Conn)
	}

	m.mu.Unlock()

	m.ReclaimConns(m.ConnsOnConn(radio.Conn), "S1 Setup")

	if stale != nil {
		m.reclaimUEsOnConnLoss(stale)

		if sc, ok := stale.(*sctp.SCTPConn); ok {
			// Aborted, not shut down: the incumbent has been superseded and a
			// graceful close would stall this S1 Setup until it times out.
			_ = sc.Abort()
		}
	}
}

// TS 36.413 §8.4.2
func (m *MME) FindConnectedRadioByGlobalENBID(g s1ap.GlobalENBID) (*Radio, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.reg.FindConnected(ENBID(g))
}

// NodeName returns the eNB's human-readable name, empty when it has not sent
// one. Mirrors amf.Radio.NodeName.
func (r *Radio) NodeName() string {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()

	return r.name
}

// UpdateRadioName updates the stored name of a connected eNB from an eNB
// Configuration Update (TS 36.413 §8.7.4).
func (m *MME) UpdateRadioName(radio *Radio, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	radio.name = name
}

// UpdateRadioSupportedTAs replaces a connected eNB's broadcast TAIs from an eNB
// Configuration Update's Supported TAs IE, which carries the whole list
// (TS 36.413 §8.7.4).
func (m *MME) UpdateRadioSupportedTAs(radio *Radio, tais []SupportedTAI) {
	m.mu.Lock()
	defer m.mu.Unlock()

	radio.supportedTAIs = tais
}

// RadioForConn returns the eNB tracked on conn, or nil if none is recorded yet
// (pre-S1 Setup).
func (m *MME) RadioForConn(conn S1APWriter) *Radio {
	m.mu.RLock()
	defer m.mu.RUnlock()

	radio, _ := m.reg.Radio(conn)

	return radio
}

// SetupComplete reports whether the eNB has completed S1 Setup, i.e. its Global eNB ID
// is claimed (TS 36.413). Under the registry lock.
func (r *Radio) SetupComplete() bool {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()

	return r.id != ""
}

// TouchLastSeen records inbound S1AP activity from the eNB as its last-seen time.
func (r *Radio) TouchLastSeen() {
	r.lastSeen.Store(time.Now().UnixNano())
}

// NewRadioForTest builds a *Radio wrapping conn for tests in other packages that call
// S1AP handlers directly (which the dispatcher hands a resolved *Radio). It is not
// registered in the MME, so node-registry methods (SetupComplete) are not usable on it.
func NewRadioForTest(conn S1APWriter) *Radio {
	return &Radio{Conn: conn, Log: logger.MmeLog}
}

// RadioSupportedTAsForTest reads the encapsulated Radio field under the registry
// lock for tests in other packages. Mirrors amf.AMF.RadioSupportedTAIsForTest.
func (m *MME) RadioSupportedTAsForTest(r *Radio) []SupportedTAI {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return r.supportedTAIs
}

// BindMMEForTest wires a test-constructed Radio to an MME and registers it under
// its conn, as a connected eNB is registered in prod, so the node-registry
// methods (NodeName, SetupComplete) are usable on it. Mirrors
// amf.Radio.BindAMFForTest.
func (r *Radio) BindMMEForTest(m *MME) {
	r.m = m

	if r.Conn == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if key, ok := r.Conn.(*sctp.SCTPConn); ok {
		m.reg.Track(key, r)
	}
}

func (m *MME) DisconnectRadio(conn *sctp.SCTPConn) {
	m.mu.Lock()

	var dropped *Radio

	if radio, ok := m.reg.Radio(conn); ok {
		m.reg.Disconnect(conn, radio)

		dropped = radio
		radio.advertisedCapacity = nil
		radio.retryNotBefore = time.Time{}
		radio.configUpdateOutstanding = false
	}

	m.mu.Unlock()

	if dropped != nil {
		dropped.configUpdateGuard.Stop()
	}

	m.reclaimUEsOnConnLoss(conn)
}

// reclaimUEsOnConnLoss handles the connections of an eNB whose SCTP association
// dropped without a graceful S1 release, so no UE Context Release Complete will
// arrive for them. Idle UEs are left alone — they run conn-independent supervision.
func (m *MME) reclaimUEsOnConnLoss(conn S1APWriter) {
	m.ReclaimConns(m.ConnsOnConn(conn), "eNB disconnect")
}

// ReclaimConns reclaims a set of UE-associated connections dropped by an eNB
// (an SCTP drop or an S1 Reset). A UE's active connection moves the UE to ECM-IDLE
// (or, mid-attach, drops it); a handover target connection aborts the handover,
// leaving the UE on its surviving source; a detached or bare connection is removed.
// trigger names the cause for the event log.
func (m *MME) ReclaimConns(conns []*UeConn, trigger string) {
	m.mu.Lock()

	var (
		orphaned       []*UeContext
		releaseTargets []s1Release
		seen           = map[*UeContext]struct{}{}
	)

	for _, c := range conns {
		ue := c.ue
		if ue == nil {
			m.releaseConnIDLocked(uint32(c.MMEUES1APID))
			continue
		}

		if _, ok := seen[ue]; ok {
			continue
		}

		seen[ue] = struct{}{}

		switch {
		case c == ue.Conn():
			// The UE's active connection is gone. A handover target — prepared or still
			// preparing on a live target eNB — is released explicitly (as abandonHandover
			// does) so it does not hold the HANDOVER REQUEST resources until its own
			// TS1RELOCoverall. A preparing target is addressed by its MME-UE-S1AP-ID alone
			// (its eNB-UE-S1AP-ID never arrived).
			if ho := ue.handover; ho != nil && ho.state != hoCommitting && ho.target != nil {
				releaseTargets = append(releaseTargets, s1Release{ho.target.Conn(), ho.target.MMEUES1APID, ho.target.ENBUES1APID, ho.state == hoPrepared})
			}

			orphaned = append(orphaned, ue)
		case ue.handover != nil && ue.handover.target == c:
			relocating := ue.handover.source == nil

			m.clearHandoverLocked(ue) // a handover target: abort, leaving the UE on its source

			if relocating {
				orphaned = append(orphaned, ue)
			}
		default:
			c.ue = nil
			m.releaseConnIDLocked(uint32(c.MMEUES1APID))
		}
	}

	m.mu.Unlock()

	for _, r := range releaseTargets {
		SendUEContextRelease(context.Background(), m, r.conn, r.mmeID, r.enbID, r.pair, causeHandoverEUTRANReason)
	}

	for _, ue := range orphaned {
		m.ReleaseUEContextLocally(ue, trigger)
	}
}

// s1Release names a UE-associated connection to send a UE Context Release Command
// to, captured under the lock for a send after it is released.
type s1Release struct {
	conn  S1APWriter
	mmeID s1ap.MMEUES1APID
	enbID s1ap.ENBUES1APID
	// pair selects the UE-S1AP-ID alternative: the full pair for a prepared target, or
	// the MME-UE-S1AP-ID alone for a still-preparing target (no eNB-UE-S1AP-ID yet).
	pair bool
}

func (m *MME) ConnectedRadios() []*Radio {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.reg.Connected()
}

func (m *MME) CountRadios() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.reg.CountConnected()
}

func (m *MME) HasRadio(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.reg.Has(named(name))
}

func named(name string) func(*Radio) bool {
	return func(r *Radio) bool { return r.name == name }
}

func identified(id string) func(*Radio) bool {
	return func(r *Radio) bool { return r.id == id }
}

func (m *MME) ListRadios() []RadioInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	radios := m.reg.All()

	out := make([]RadioInfo, 0, len(radios))
	for _, s := range radios {
		out = append(out, s.info())
	}

	return out
}

func (m *MME) ForgetRadio(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	online, forgotten := m.reg.Forget(identified(id))

	switch {
	case online:
		return ErrRadioOnline
	case forgotten == 0:
		return ErrRadioNotFound
	}

	return nil
}

func (m *MME) OfflineRadioCountForTest() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.reg.CountOffline()
}

func (m *MME) SetRadioRetentionForTest(ttl time.Duration, maxOffline int, now func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reg.SetRetention(ttl, maxOffline, now)
}
