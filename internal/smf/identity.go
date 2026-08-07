// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

type SessionIdentity struct {
	PDUSessionID uint8
	EBI          uint8
}

func epsBearerKey(ebi uint8) uint8 { return 64 + ebi }

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

func canonicalName(identifier etsi.SUPI, sessionKey uint8) string {
	return fmt.Sprintf("%s-%d", identifier.String(), sessionKey)
}
