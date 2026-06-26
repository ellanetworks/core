// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/models"
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
	// supportedTAIs are the TAIs the eNB broadcasts, from the Supported TAs IE of
	// its S1 Setup Request; replaced wholesale on an eNB Configuration Update
	// (TS 36.413 §8.7.3.2, §8.7.4). Guarded by MME.mu.
	supportedTAIs []ENBTAI
}

// ENBTAI is a Tracking Area Identity an eNB broadcasts: a served PLMN paired with
// a cell's TAC. 4G eNBs advertise no network slices, so unlike a 5G gNB's TAI it
// carries no S-NSSAIs.
type ENBTAI struct {
	PlmnID models.PlmnID
	TAC    uint16
}

// ENBInfo is a read-only view of a connected eNB, exposed for status/API.
type ENBInfo struct {
	Name          string
	ID            string
	Address       string
	ConnectedAt   time.Time
	LastSeenAt    time.Time
	SupportedTAIs []ENBTAI
}

func (s *enbState) info() ENBInfo {
	return ENBInfo{
		Name:          s.name,
		ID:            s.id,
		Address:       s.address,
		ConnectedAt:   s.connectedAt,
		LastSeenAt:    time.Unix(0, s.lastSeen.Load()),
		SupportedTAIs: s.supportedTAIs,
	}
}

// enbSupportedTAIs flattens an S1 Setup Request's Supported TAs into the TAIs the
// eNB broadcasts: one entry per (broadcast PLMN, TAC) pair (TS 36.413 §8.7.3.2).
func enbSupportedTAIs(tas s1ap.SupportedTAs) []ENBTAI {
	out := make([]ENBTAI, 0, len(tas))
	for _, ta := range tas {
		for _, plmn := range ta.BroadcastPLMNs {
			out = append(out, ENBTAI{PlmnID: decodePLMN(plmn), TAC: uint16(ta.TAC)})
		}
	}

	return out
}

// enbID renders a Global eNB ID as "<plmn>-<enb-id>" for display.
func enbID(g s1ap.GlobalENBID) string {
	p := g.PLMNIdentity

	return fmt.Sprintf("%02x%02x%02x-%x", p[0], p[1], p[2], g.ENBID.Value)
}

// trackENB records a connected eNB keyed by its SCTP association.
func (m *MME) trackENB(key *sctp.SCTPConn, info ENBInfo) {
	s := &enbState{name: info.Name, id: info.ID, address: info.Address, connectedAt: info.ConnectedAt, supportedTAIs: info.SupportedTAIs}
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
		Name:          req.ENBName,
		ID:            enbID(req.GlobalENBID),
		Address:       address,
		ConnectedAt:   now,
		LastSeenAt:    now,
		SupportedTAIs: enbSupportedTAIs(req.SupportedTAs),
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
		// Index the association by Global eNB ID so an S1 handover can resolve a
		// HANDOVER REQUIRED's target to its association (TS 36.413 §8.4.1).
		m.enbByID[s.id] = conn
	}
}

// findENBByGlobalID resolves a Global eNB ID to the S1-setup-complete association
// of that eNB (nil/false when no such eNB is connected), for routing an S1
// handover's HANDOVER REQUEST to the target (TS 36.413 §8.4.2).
func (m *MME) findENBByGlobalID(g s1ap.GlobalENBID) (nasWriter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, ok := m.enbByID[enbID(g)]

	return conn, ok
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

// updateENBSupportedTAs replaces a connected eNB's broadcast TAIs from an eNB
// Configuration Update's Supported TAs IE, which carries the whole list
// (TS 36.413 §8.7.4). No-op if the eNB is not tracked.
func (m *MME) updateENBSupportedTAs(conn *sctp.SCTPConn, tais []ENBTAI) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s := m.enbs[conn]; s != nil {
		s.supportedTAIs = tais
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
	if s := m.enbs[conn]; s != nil && m.enbByID[s.id] == nasWriter(conn) {
		delete(m.enbByID, s.id)
	}

	delete(m.enbs, conn)
	m.mu.Unlock()

	m.reclaimUEsOnConnLoss(conn)
}

// reclaimUEsOnConnLoss handles the connections of an eNB whose SCTP association
// dropped without a graceful S1 release, so no UE Context Release Complete will
// arrive for them. Idle UEs are left alone — they run conn-independent supervision.
func (m *MME) reclaimUEsOnConnLoss(conn nasWriter) {
	m.reclaimConns(m.connsOnConn(conn), "eNB disconnect")
}

// reclaimConns reclaims a set of UE-associated connections an eNB no longer holds
// (an SCTP drop or an S1 Reset). A UE's active connection moves the UE to ECM-IDLE
// (or, mid-attach, drops it); a handover target connection aborts the handover,
// leaving the UE on its surviving source; a detached or bare connection is removed.
// trigger names the cause for the event log.
func (m *MME) reclaimConns(conns []*s1Conn, trigger string) {
	m.mu.Lock()

	var (
		orphaned       []*UeContext
		releaseTargets []s1Release
		seen           = map[*UeContext]struct{}{}
	)

	for _, c := range conns {
		ue := c.ue
		if ue == nil {
			delete(m.conns, uint32(c.MMEUES1APID))
			continue
		}

		if _, ok := seen[ue]; ok {
			continue
		}

		seen[ue] = struct{}{}

		switch {
		case c == ue.s1:
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
			delete(m.conns, uint32(c.MMEUES1APID))
		}
	}

	m.mu.Unlock()

	for _, r := range releaseTargets {
		m.sendUEContextRelease(context.Background(), r.conn, r.mmeID, r.enbID)
	}

	for _, ue := range orphaned {
		m.releaseUEContextLocally(ue, trigger)
	}
}

// s1Release names a UE-associated connection to send a UE Context Release Command
// to, captured under the lock for a send after it is released.
type s1Release struct {
	conn  nasWriter
	mmeID s1ap.MMEUES1APID
	enbID s1ap.ENBUES1APID
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
