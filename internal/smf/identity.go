// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

// SessionIdentity names a session in the two identity namespaces a converged
// SMF+PGW-C context holds at once (TS 23.501 §5.17.2). Both run 1..15 and are
// distinct, so neither can stand in for the other.
type SessionIdentity struct {
	// PDUSessionID is the UE-allocated 5GS PDU session identity (1..15,
	// TS 24.007 §11.2.3.1b), 0 when unassigned.
	PDUSessionID uint8
	// EBI is the default bearer's EPS bearer identity (5..15, TS 24.301 §6.1),
	// 0 when unassigned.
	EBI uint8
}

// epsBearerKey maps an EPS bearer identity into the SMF's converged key space.
// TS 29.571 §5.2.2: 64..95 is core-network-allocated, disjoint from the 1..15 a
// UE allocates.
func epsBearerKey(ebi uint8) uint8 { return 64 + ebi }

// sessionKey names the session's slot: its PDU session identity when the UE
// allocated one, else its EPS bearer key. CanonicalName and the UE IP leases
// are keyed by it, so it holds for the session's whole life.
func (id SessionIdentity) sessionKey() uint8 {
	if id.PDUSessionID != 0 {
		return id.PDUSessionID
	}

	return epsBearerKey(id.EBI)
}

// sessionKeys returns every key the session answers to.
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

// valid reports whether the identity names a session at all: a PDU session
// identity of 1..15 (TS 24.007 §11.2.3.1b), an EPS bearer identity of 5..15
// with 1..4 reserved (TS 24.301 §6.1), or both.
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
