// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
)

// enbState is the MME's mutable per-eNB record. lastSeen (Unix nanoseconds) is
// updated on the inbound S1AP hot path and read concurrently, so it is atomic;
// name may change via an eNB Configuration Update (guarded by MME.mu); the
// remaining fields are immutable after the eNB associates.
type enbState struct {
	name        string
	id          string
	address     string
	connectedAt time.Time
	lastSeen    atomic.Int64
	// setupComplete is set once S1 Setup succeeds for this association, arming the
	// dispatcher's setup-first gate (TS 36.413). Guarded by MME.mu.
	setupComplete bool
}

// ENBInfo is a read-only view of a connected eNB, exposed for status/API.
type ENBInfo struct {
	Name        string
	ID          string
	Address     string
	ConnectedAt time.Time
	LastSeenAt  time.Time
}

func (s *enbState) info() ENBInfo {
	return ENBInfo{
		Name:        s.name,
		ID:          s.id,
		Address:     s.address,
		ConnectedAt: s.connectedAt,
		LastSeenAt:  time.Unix(0, s.lastSeen.Load()),
	}
}

// enbID renders a Global eNB ID as "<plmn>-<enb-id>" for display.
func enbID(g s1ap.GlobalENBID) string {
	p := g.PLMNIdentity

	return fmt.Sprintf("%02x%02x%02x-%x", p[0], p[1], p[2], g.ENBID.Value)
}

// trackENB records a connected eNB keyed by its SCTP association.
func (m *MME) trackENB(key *sctp.SCTPConn, info ENBInfo) {
	s := &enbState{name: info.Name, id: info.ID, address: info.Address, connectedAt: info.ConnectedAt}
	s.lastSeen.Store(info.LastSeenAt.UnixNano())

	m.mu.Lock()
	defer m.mu.Unlock()

	m.enbs[key] = s
}

// addENB records a connected eNB from a decoded S1 Setup Request, stamping its
// connection time and initial last-seen.
func (m *MME) addENB(conn *sctp.SCTPConn, req *s1ap.S1SetupRequest) {
	address := ""
	if a := conn.RemoteAddr(); a != nil {
		address = a.String()
	}

	now := time.Now()
	m.trackENB(conn, ENBInfo{
		Name:        req.ENBName,
		ID:          enbID(req.GlobalENBID),
		Address:     address,
		ConnectedAt: now,
		LastSeenAt:  now,
	})
}

// trackENBFromSetup records the eNB from an S1 Setup Request's raw value. Parse
// failures are surfaced by the S1 Setup handler.
func (m *MME) trackENBFromSetup(conn *sctp.SCTPConn, value []byte) {
	req, err := s1ap.ParseS1SetupRequest(value)
	if err != nil {
		return
	}

	m.addENB(conn, req)
}

// markENBSetupComplete records that the eNB on conn completed S1 Setup, arming
// the dispatcher's setup-first gate (TS 36.413). No-op if the eNB is
// not tracked.
func (m *MME) markENBSetupComplete(conn *sctp.SCTPConn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s := m.enbs[conn]; s != nil {
		s.setupComplete = true
	}
}

// updateENBName updates the stored name of a connected eNB from an eNB
// Configuration Update (TS 36.413 §8.7.4).
func (m *MME) updateENBName(conn *sctp.SCTPConn, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s := m.enbs[conn]; s != nil {
		s.name = name
	}
}

// enbSetupComplete reports whether the eNB on conn has completed S1 Setup.
func (m *MME) enbSetupComplete(conn *sctp.SCTPConn) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s := m.enbs[conn]

	return s != nil && s.setupComplete
}

// touchENB records inbound S1AP activity from an eNB association as its last-seen
// time. It is a no-op for an association not yet recorded (pre-S1 Setup).
func (m *MME) touchENB(conn *sctp.SCTPConn) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if s := m.enbs[conn]; s != nil {
		s.lastSeen.Store(time.Now().UnixNano())
	}
}

// removeENB drops a connected eNB when its association closes.
func (m *MME) removeENB(conn *sctp.SCTPConn) {
	m.mu.Lock()
	delete(m.enbs, conn)
	m.mu.Unlock()

	m.reclaimUEsOnConnLoss(conn)
}

// reclaimUEsOnConnLoss handles the UEs of an eNB whose SCTP association dropped
// without a graceful S1 release, so no UE Context Release Complete will arrive
// for them. Idle UEs are left alone — they already run their own
// conn-independent mobile reachable supervision.
func (m *MME) reclaimUEsOnConnLoss(conn nasWriter) {
	m.mu.Lock()

	var orphaned []*UeContext

	for _, ue := range m.ues {
		if ue.conn == conn && ue.ecmState == ECMConnected {
			orphaned = append(orphaned, ue)
		}
	}

	m.mu.Unlock()

	for _, ue := range orphaned {
		m.releaseUEContextLocally(ue, "eNB disconnect")
	}
}

// CountENBs returns the number of eNBs currently associated with the MME.
func (m *MME) CountENBs() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.enbs)
}

// HasENB reports whether an eNB with the given name is currently associated.
func (m *MME) HasENB(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.enbs {
		if s.name == name {
			return true
		}
	}

	return false
}

// ListENBs returns the eNBs currently associated with the MME.
func (m *MME) ListENBs() []ENBInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]ENBInfo, 0, len(m.enbs))
	for _, s := range m.enbs {
		out = append(out, s.info())
	}

	return out
}
