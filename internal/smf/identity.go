// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

// A converged SMF+PGW-C holds both namespaces at once (TS 23.501 §5.17.2), and
// both run 1..15, so neither identity can stand in for the other.
type SessionIdentity struct {
	// UE-allocated, 1..15 (TS 24.007 §11.2.3.1b). 0 when unassigned.
	PDUSessionID uint8
	// Default bearer's, 5..15 (TS 24.301 §9.3.2). 0 when unassigned.
	EBI uint8
}

// An internal alias into 64..95, the range TS 29.571 table 5.4.2-1 reserves for
// core-network allocation and so disjoint from the 1..15 a UE allocates.
func epsBearerKey(ebi uint8) uint8 { return 64 + ebi }

// The key names the session's slot for its whole life.
func (id SessionIdentity) sessionKey() uint8 {
	if id.PDUSessionID != 0 {
		return id.PDUSessionID
	}

	return epsBearerKey(id.EBI)
}

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

// EPS bearer identities 1..4 are refused: TS 24.301 §6.5.0 NOTE 2 has them
// treated as reserved by a UE or network not supporting 15 bearer contexts, and
// this core does not (§9.3.2).
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

func CanonicalName(identifier etsi.SUPI, sessionKey uint8) string {
	return fmt.Sprintf("%s-%d", identifier.String(), sessionKey)
}
