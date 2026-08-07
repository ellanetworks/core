// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"errors"
	"fmt"
)

// AccessType is the radio access a session is established over. As the combined
// SMF+PGW-C (TS 23.501), the SMF keys its 4G/5G differences off it.
type AccessType uint8

const (
	Access5G AccessType = iota // N3 user plane; 5GSM NAS terminated in the SMF
	Access4G                   // S1-U user plane; ESM owned by the MME (PGW-C role)
)

// ErrSessionNotFound reports a reference no session in the pool answers, for the
// entry points that treat a reference to a released session as a no-op.
var ErrSessionNotFound = errors.New("sm context not found")

// IsEPS reports whether the session is a 4G EPS bearer (PGW-C role).
func (sc *SMContext) IsEPS() bool { return sc.Access == Access4G }

// binds reports whether the access may bind the session's downlink: the one
// serving it, or the target of a move it has not bound, whose bind is what
// commits that move (TS 23.401 §5.10.2 step 13a, TS 23.502 §4.3.2.2.1 step 16a
// NOTE 11). Every other entry point of the target access speaks for a session it
// does not yet serve. Caller holds mu.
func (sc *SMContext) binds(access AccessType) bool {
	return sc.Access == access || (sc.pending != nil && sc.pending.to == access)
}

// sessionFor resolves ref, takes the session's lock and refuses an access other
// than the one serving it. A reference outlives a move, so a request can arrive
// on the access the session left; refusing it is a local choice, as no clause
// covers a request from an access that has given the session up. The returned
// function unlocks, and is nil when the error is not.
func (s *SMF) sessionFor(ref string, access AccessType) (*SMContext, func(), error) {
	sc, unlock, err := s.lockSession(ref)
	if err != nil {
		return nil, nil, err
	}

	if sc.Access != access {
		unlock()

		return nil, nil, fmt.Errorf("session %q is on %s", ref, sc.Access)
	}

	return sc, unlock, nil
}

// sessionBinding is sessionFor for the two entry points that bind a downlink,
// which is where a move commits.
func (s *SMF) sessionBinding(ref string, access AccessType) (*SMContext, func(), error) {
	sc, unlock, err := s.lockSession(ref)
	if err != nil {
		return nil, nil, err
	}

	if !sc.binds(access) {
		unlock()

		return nil, nil, fmt.Errorf("session %q is on %s", ref, sc.Access)
	}

	return sc, unlock, nil
}

// The access is checked under the same lock hold as the work it guards, so no
// move can land between them.
func (s *SMF) lockSession(ref string) (*SMContext, func(), error) {
	// An empty reference names no session and is not one that has been released,
	// so it is an error to the entry points that treat the latter as a no-op.
	if ref == "" {
		return nil, nil, fmt.Errorf("SM context reference is missing")
	}

	sc := s.GetSession(ref)
	if sc == nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrSessionNotFound, ref)
	}

	sc.mu.Lock()

	return sc, sc.mu.Unlock, nil
}

func (a AccessType) String() string {
	if a == Access4G {
		return "EPS"
	}

	return "5GS"
}

// usesPSC reports whether the user-plane GTP-U carries the PDU Session Container
// (and thus the QFI). 5G N3/N9 do; 4G S1-U does not. TS 23.501, TS 38.415.
func (a AccessType) usesPSC() bool { return a == Access5G }
