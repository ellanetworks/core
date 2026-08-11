// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import "errors"

const (
	firstEPSBearerIdentity = 5
	lastEPSBearerIdentity  = 15
)

var (
	// ErrNoEPSBearerIdentity reports that the UE's EPS bearer identity space is
	// exhausted (TS 23.502 §4.11.1.4.1).
	ErrNoEPSBearerIdentity = errors.New("amf: no EPS bearer identity available for this UE")
	ErrNoSmContext         = errors.New("amf: no SM context for that PDU session")
)

// AllocateEPSBearerIdentity assigns an EPS bearer identity to a PDU session, so
// that the session can become a PDN connection on mobility to EPS
// (TS 23.502 §4.11.1.4). The AMF owns the identity space, UE-scoped across every
// PDU session; identities 1 to 4 stay reserved because the MME does not advertise
// the 15-bearer capability (TS 24.301 §6.5.0).
//
// A session that already holds one keeps it, so re-establishing the same session
// does not renumber a bearer the UE has been told about.
func (ue *UeContext) AllocateEPSBearerIdentity(pduSessionID uint8) (uint8, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	sc, ok := ue.SmContextList[pduSessionID]
	if !ok {
		return 0, ErrNoSmContext
	}

	if sc.EPSBearerIdentity != 0 {
		return sc.EPSBearerIdentity, nil
	}

	taken := make(map[uint8]struct{}, len(ue.SmContextList))
	for _, other := range ue.SmContextList {
		if other.EPSBearerIdentity != 0 {
			taken[other.EPSBearerIdentity] = struct{}{}
		}
	}

	for ebi := uint8(firstEPSBearerIdentity); ebi <= lastEPSBearerIdentity; ebi++ {
		if _, used := taken[ebi]; used {
			continue
		}

		sc.EPSBearerIdentity = ebi

		return ebi, nil
	}

	return 0, ErrNoEPSBearerIdentity
}

// EPSBearerIdentity returns the identity assigned to a PDU session, reporting
// false when it has none.
func (ue *UeContext) EPSBearerIdentity(pduSessionID uint8) (uint8, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	sc, ok := ue.SmContextList[pduSessionID]
	if !ok || sc.EPSBearerIdentity == 0 {
		return 0, false
	}

	return sc.EPSBearerIdentity, true
}

// EPSBearerIdentities returns every assigned identity, keyed by PDU session.
func (ue *UeContext) EPSBearerIdentities() map[uint8]uint8 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make(map[uint8]uint8, len(ue.SmContextList))

	for pduSessionID, sc := range ue.SmContextList {
		if sc.EPSBearerIdentity != 0 {
			out[pduSessionID] = sc.EPSBearerIdentity
		}
	}

	return out
}
