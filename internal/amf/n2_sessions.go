// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"slices"
	"sync"
)

type n2State uint8

const (
	n2Inactive n2State = iota
	n2Pending
	n2Active
)

type n2Session struct {
	state n2State
	proc  N2SetupProcedure
}

// n2Sessions records, per PDU session, what the UE-associated logical NG-connection
// owning it has set up on the NG-RAN node. The state belongs to the connection, not to
// the UE: a UE that turns up on a new NG-connection holds no AN resources there, and
// the UP connection of the PDU sessions that were active on the old one is deactivated
// with it (TS 23.501 §5.3.3.2.4, TS 23.502 §4.2.6).
type n2Sessions struct {
	mu       sync.Mutex
	sessions map[uint8]n2Session
}

// ClaimN2Session moves a PDU session to n2Pending on this connection, so that only one
// setup procedure at a time carries it to the NG-RAN node. It returns false when the
// session is already pending or already set up here.
func (ueConn *UeConn) ClaimN2Session(proc N2SetupProcedure, pduSessionID uint8) bool {
	ueConn.n2Sessions.mu.Lock()
	defer ueConn.n2Sessions.mu.Unlock()

	if ueConn.n2Sessions.sessions[pduSessionID].state != n2Inactive {
		return false
	}

	ueConn.setN2SessionLocked(pduSessionID, n2Session{state: n2Pending, proc: proc})

	return true
}

func (ueConn *UeConn) setN2SessionLocked(pduSessionID uint8, s n2Session) {
	if ueConn.n2Sessions.sessions == nil {
		ueConn.n2Sessions.sessions = make(map[uint8]n2Session)
	}

	ueConn.n2Sessions.sessions[pduSessionID] = s
}

// SetN2SessionActive records that the NG-RAN node has confirmed the AN resources of a
// PDU session on this connection.
func (ueConn *UeConn) SetN2SessionActive(pduSessionID uint8) {
	ueConn.n2Sessions.mu.Lock()
	defer ueConn.n2Sessions.mu.Unlock()

	ueConn.setN2SessionLocked(pduSessionID, n2Session{state: n2Active})
}

// SetN2SessionInactive records that a PDU session holds no AN resources on this
// connection.
func (ueConn *UeConn) SetN2SessionInactive(pduSessionID uint8) {
	if ueConn == nil {
		return
	}

	ueConn.n2Sessions.mu.Lock()
	defer ueConn.n2Sessions.mu.Unlock()

	delete(ueConn.n2Sessions.sessions, pduSessionID)
}

// N2SessionInactive reports whether a PDU session holds no AN resources on this
// connection. A UE on no connection is CM-IDLE, so it holds none.
func (ueConn *UeConn) N2SessionInactive(pduSessionID uint8) bool {
	if ueConn == nil {
		return true
	}

	ueConn.n2Sessions.mu.Lock()
	defer ueConn.n2Sessions.mu.Unlock()

	return ueConn.n2Sessions.sessions[pduSessionID].state == n2Inactive
}

// HasActiveN2Sessions reports whether any PDU session is set up on this connection.
func (ueConn *UeConn) HasActiveN2Sessions() bool {
	if ueConn == nil {
		return false
	}

	ueConn.n2Sessions.mu.Lock()
	defer ueConn.n2Sessions.mu.Unlock()

	for _, s := range ueConn.n2Sessions.sessions {
		if s.state == n2Active {
			return true
		}
	}

	return false
}

// AdoptN2SessionsFrom takes over the AN resources of the connection a completed
// handover moved the UE off: they are the same resources, and what the target does not
// take over has to be deactivated rather than left pointing at the source
// (TS 23.502 §4.9.1.3.3). Claims already made on this connection win.
func (ueConn *UeConn) AdoptN2SessionsFrom(source *UeConn) {
	if ueConn == nil || source == nil || source == ueConn {
		return
	}

	source.n2Sessions.mu.Lock()

	adopted := make([]uint8, 0, len(source.n2Sessions.sessions))

	for id, s := range source.n2Sessions.sessions {
		if s.state == n2Active {
			adopted = append(adopted, id)
		}
	}

	source.n2Sessions.mu.Unlock()

	ueConn.n2Sessions.mu.Lock()
	defer ueConn.n2Sessions.mu.Unlock()

	for _, id := range adopted {
		if ueConn.n2Sessions.sessions[id].state != n2Inactive {
			continue
		}

		ueConn.setN2SessionLocked(id, n2Session{state: n2Active})
	}
}

// releaseAllN2Sessions drops every record of AN resources on this connection: the
// connection is being torn down, and the resources it describes go with it.
func (ueConn *UeConn) releaseAllN2Sessions() {
	if ueConn == nil {
		return
	}

	ueConn.n2Sessions.mu.Lock()
	defer ueConn.n2Sessions.mu.Unlock()

	ueConn.n2Sessions.sessions = nil
}

// activeN2Sessions returns the PDU sessions the NG-RAN node has set up on this
// connection, in ascending order.
func (ueConn *UeConn) activeN2Sessions() []uint8 {
	ueConn.n2Sessions.mu.Lock()
	defer ueConn.n2Sessions.mu.Unlock()

	var ids []uint8

	for id, s := range ueConn.n2Sessions.sessions {
		if s.state == n2Active {
			ids = append(ids, id)
		}
	}

	slices.Sort(ids)

	return ids
}

func (ueConn *UeConn) pendingN2SessionsFor(proc N2SetupProcedure) []uint8 {
	ueConn.n2Sessions.mu.Lock()
	defer ueConn.n2Sessions.mu.Unlock()

	var ids []uint8

	for id, s := range ueConn.n2Sessions.sessions {
		if s.state == n2Pending && s.proc == proc {
			ids = append(ids, id)
		}
	}

	slices.Sort(ids)

	return ids
}

func (ueConn *UeConn) releaseN2SessionIfPending(pduSessionID uint8) bool {
	ueConn.n2Sessions.mu.Lock()
	defer ueConn.n2Sessions.mu.Unlock()

	if ueConn.n2Sessions.sessions[pduSessionID].state != n2Pending {
		return false
	}

	delete(ueConn.n2Sessions.sessions, pduSessionID)

	return true
}
