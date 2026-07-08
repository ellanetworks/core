// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// Radio is the MME's mutable per-eNB record. lastSeen (Unix nanoseconds) is
// updated on the inbound S1AP hot path and read concurrently, so it is atomic;
// name may change via an eNB Configuration Update (guarded by MME.mu); the
// remaining fields are immutable after the eNB associates.
type Radio struct {
	// Conn is the eNB's S1 association, the send target for node-level (non-UE)
	// S1AP. Set at construction and immutable (mirrors the AMF's Radio.Conn).
	Conn S1APWriter
	// m is the owning MME, so node-scoped methods reach the registry lock without
	// threading it through every call (mirrors the AMF's Radio.amf). Set at
	// construction, immutable.
	m           *MME
	name        string
	id          string
	address     string
	connectedAt time.Time
	lastSeen    atomic.Int64
	// setupComplete is set once S1 Setup succeeds for this association, arming the
	// dispatcher's setup-first gate (TS 36.413). Guarded by MME.mu.
	setupComplete bool
	// supportedTAIs are the TAIs the eNB broadcasts, from the Supported TAs IE of
	// its S1 Setup Request; replaced wholesale on an eNB Configuration Update
	// (TS 36.413 §8.7.3.2, §8.7.4). Guarded by MME.mu.
	supportedTAIs []SupportedTAI
	// Log carries the eNB's RAN address for node-level correlation. Keyed by the
	// immutable SCTP address, so it never goes stale.
	Log *zap.Logger
}

// SupportedTAI is a Tracking Area Identity an eNB broadcasts: a served PLMN paired
// with a cell's TAC (TS 36.413 §8.7.3.2). Mirrors the AMF's SupportedTAI; the
// S-NSSAI list is a 5G-only field, so a 4G eNB's TAI omits it.
type SupportedTAI struct {
	Tai models.Tai
}

// RadioInfo is a read-only view of a connected eNB, exposed for status/API.
type RadioInfo struct {
	Name          string
	ID            string
	Address       string
	ConnectedAt   time.Time
	LastSeenAt    time.Time
	SupportedTAIs []SupportedTAI
}

func (r *Radio) info() RadioInfo {
	return RadioInfo{
		Name:          r.name,
		ID:            r.id,
		Address:       r.address,
		ConnectedAt:   r.connectedAt,
		LastSeenAt:    time.Unix(0, r.lastSeen.Load()),
		SupportedTAIs: r.supportedTAIs,
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

	m.radios[key] = s
}

// addRadio records a connected eNB from a decoded S1 Setup Request, stamping its
// connection time and initial last-seen.
func (m *MME) addRadio(conn *sctp.SCTPConn, req *s1ap.S1SetupRequest) {
	address := ""
	if a := conn.RemoteAddr(); a != nil {
		address = a.String()
	}

	now := time.Now()
	m.trackRadio(conn, RadioInfo{
		Name:          req.ENBName,
		ID:            ENBID(req.GlobalENBID),
		Address:       address,
		ConnectedAt:   now,
		LastSeenAt:    now,
		SupportedTAIs: EnbSupportedTAIs(req.SupportedTAs),
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
	s := m.radios[conn]
	m.mu.RUnlock()

	return nodeLog(s, sc)
}

// nodeLogLocked is RadioLog for callers already holding MME.mu, avoiding a re-lock.
func (m *MME) nodeLogLocked(conn S1APWriter) *zap.Logger {
	sc, _ := conn.(*sctp.SCTPConn)

	return nodeLog(m.radios[conn], sc)
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

// MarkRadioSetupComplete records that the eNB on conn completed S1 Setup, arming
// the dispatcher's setup-first gate (TS 36.413). No-op if the eNB is
// not tracked.
func (m *MME) MarkRadioSetupComplete(conn *sctp.SCTPConn) {
	m.mu.Lock()

	s := m.radios[conn]
	if s == nil {
		m.mu.Unlock()
		return
	}

	s.setupComplete = true

	// Index the association by Global eNB ID so an S1 handover can resolve a
	// HANDOVER REQUIRED's target to its association (TS 36.413 §8.4.1). A
	// re-associating eNB that completes S1 Setup under an ID still held by a
	// different live association supersedes it: the stale association is evicted
	// and torn down so the ID resolves to the current one and an S1 handover cannot
	// target a dead eNB.
	var stale S1APWriter

	if prev, ok := m.radiosByID[s.id]; ok && prev.Conn != S1APWriter(conn) {
		stale = prev.Conn
		delete(m.radios, prev.Conn)
	}

	m.radiosByID[s.id] = s
	m.mu.Unlock()

	if stale != nil {
		m.reclaimUEsOnConnLoss(stale)

		if sc, ok := stale.(*sctp.SCTPConn); ok {
			_ = sc.Close()
		}
	}
}

// FindRadioByGlobalENBID resolves a Global eNB ID to the S1-setup-complete association
// of that eNB (nil/false when no such eNB is connected), for routing an S1
// handover's HANDOVER REQUEST to the target (TS 36.413 §8.4.2).
func (m *MME) FindRadioByGlobalENBID(g s1ap.GlobalENBID) (*Radio, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	radio, ok := m.radiosByID[ENBID(g)]

	return radio, ok
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
// (pre-S1 Setup). The dispatcher resolves the *Radio once at ingress and threads it to
// handlers, mirroring the AMF's per-message *Radio.
func (m *MME) RadioForConn(conn S1APWriter) *Radio {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.radios[conn]
}

// SetupComplete reports whether the eNB has completed S1 Setup (TS 36.413), under the
// registry lock.
func (r *Radio) SetupComplete() bool {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()

	return r.setupComplete
}

// TouchLastSeen records inbound S1AP activity from the eNB as its last-seen time.
// Safe for concurrent use (lastSeen is atomic). Mirrors the AMF's Radio.TouchLastSeen.
func (r *Radio) TouchLastSeen() {
	r.lastSeen.Store(time.Now().UnixNano())
}

// NewRadioForTest builds a *Radio wrapping conn for tests in other packages that call
// S1AP handlers directly (which the dispatcher now hands a resolved *Radio). It is not
// registered in the MME, so node-registry methods (SetupComplete) are not usable on it.
func NewRadioForTest(conn S1APWriter) *Radio {
	return &Radio{Conn: conn, Log: logger.MmeLog}
}

// RemoveRadio drops a connected eNB when its association closes.
func (m *MME) RemoveRadio(conn *sctp.SCTPConn) {
	m.mu.Lock()
	if s := m.radios[conn]; s != nil && m.radiosByID[s.id] == s {
		delete(m.radiosByID, s.id)
	}

	delete(m.radios, conn)
	m.mu.Unlock()

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
			// The UE's active connection is gone. A handover prepared on a (still
			// live) target eNB is released explicitly, like the guard-timer abort, so
			// the target does not hold the context until its own TS1RELOCoverall.
			if ho := ue.handover; ho != nil && ho.state == hoPrepared {
				releaseTargets = append(releaseTargets, s1Release{ho.target.conn, ho.target.MMEUES1APID, ho.target.ENBUES1APID})
			}

			orphaned = append(orphaned, ue)
		case ue.handover != nil && ue.handover.target == c:
			m.clearHandoverLocked(ue) // a handover target: abort, leaving the UE on its source
		default:
			c.ue = nil
			m.releaseConnIDLocked(uint32(c.MMEUES1APID))
		}
	}

	m.mu.Unlock()

	for _, r := range releaseTargets {
		SendUEContextRelease(m, context.Background(), r.conn, r.mmeID, r.enbID, true, causeHandoverEUTRANReason)
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
}

// CountRadios returns the number of eNBs currently associated with the MME.
func (m *MME) CountRadios() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.radios)
}

// HasRadio reports whether an eNB with the given name is currently associated.
func (m *MME) HasRadio(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.radios {
		if s.name == name {
			return true
		}
	}

	return false
}

// ListRadios returns the eNBs currently associated with the MME.
func (m *MME) ListRadios() []RadioInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]RadioInfo, 0, len(m.radios))
	for _, s := range m.radios {
		out = append(out, s.info())
	}

	return out
}
