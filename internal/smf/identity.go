// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

// SessionIdentity names a session in the two identity namespaces a converged
// SMF+PGW-C context holds at once (TS 23.501 §5.17.2): the 5GS PDU session
// identity the UE allocates, and the default bearer's EPS bearer identity. Both
// run 1..15 and are distinct, so neither can stand in for the other. A session
// carries the identity of the access it was established over and gains the
// other one only where interworking assigns it.
type SessionIdentity struct {
	// PDUSessionID is the UE-allocated 5GS PDU session identity (1..15,
	// TS 24.007 §11.2.3.1b). Zero on an EPS session the UE named with none.
	PDUSessionID uint8
	// EBI is the default bearer's EPS bearer identity (5..15, TS 24.301 §6.1).
	// Zero on a PDU session with no EPS bearer assigned.
	EBI uint8
}

// epsBearerKey maps an EPS bearer identity into the SMF's converged key space.
// The 64..95 range is core-network-allocated and never visible to the UE
// (TS 29.571 §5.2.2), so it cannot collide with the 1..15 a UE allocates for a
// PDU session identity.
func epsBearerKey(ebi uint8) uint8 { return 64 + ebi }

// sessionKey is the key naming the session's slot: its PDU session identity
// when the UE allocated one, else its EPS bearer key. CanonicalName is built
// from it and the UE IP leases are keyed by it, so it has to hold for the
// session's whole life. Both identities are assigned at creation and no key
// derives from the access, so neither an access change nor a later identity
// assignment re-keys a live session.
func (id SessionIdentity) sessionKey() uint8 {
	if id.PDUSessionID != 0 {
		return id.PDUSessionID
	}

	return epsBearerKey(id.EBI)
}

// sessionKeys returns every key the session answers to, so a lookup in either
// namespace resolves the one session that holds both identities.
func (id SessionIdentity) sessionKeys() []uint8 {
	keys := make([]uint8, 0, 2)

	if id.PDUSessionID != 0 {
		keys = append(keys, id.PDUSessionID)
	}

	if id.EBI != 0 {
		keys = append(keys, epsBearerKey(id.EBI))
	}

	return keys
}

// valid reports whether the identity names a session at all. The SMF allocates
// neither half: the UE picks a PDU session identity from 1..15 (TS 24.007
// §11.2.3.1b) and the MME an EPS bearer identity from 5..15, 1..4 being reserved
// (TS 24.301 §6.1).
func (id SessionIdentity) valid() bool {
	if id.PDUSessionID > 15 {
		return false
	}

	if id.EBI != 0 && (id.EBI < 5 || id.EBI > 15) {
		return false
	}

	return id.PDUSessionID != 0 || id.EBI != 0
}

func (id SessionIdentity) String() string {
	return fmt.Sprintf("pdu-session-id=%d ebi=%d", id.PDUSessionID, id.EBI)
}

// CanonicalName is the secondary index key for a (SUPI, session key) slot.
func CanonicalName(identifier etsi.SUPI, sessionKey uint8) string {
	return fmt.Sprintf("%s-%d", identifier.String(), sessionKey)
}
