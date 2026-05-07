// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"crypto/rand"
	"fmt"
)

// GenerateIID returns a cryptographically random 64-bit Interface Identifier
// (IID) for use in IPv6 PDU sessions. Per 3GPP TS 24.501, the SMF generates
// a random IID that is sent to the UE in the PDU Session Establishment Accept.
// The UE forms a link-local address (fe80::IID) and uses Router Solicitation /
// Router Advertisement to learn the delegated /64 prefix.
func GenerateIID() ([8]byte, error) {
	var iid [8]byte

	if _, err := rand.Read(iid[:]); err != nil {
		return iid, fmt.Errorf("generate IID: %w", err)
	}

	return iid, nil
}

// iIDMaxRetries is the maximum number of attempts to generate a unique IID
// before giving up. With a 64-bit space, collisions are astronomically unlikely
// (birthday bound ~2^32 samples for 50% probability), but we bound retries
// to avoid an infinite loop in the pathological case.
const iIDMaxRetries = 3

// assignIID generates a unique IID by checking against existing sessions for
// the given DNN. It retries up to iIDMaxRetries times if a collision is found.
func (s *SMF) assignIID(dnn string) ([8]byte, error) {
	s.mu.RLock()
	seen := make(map[[8]byte]bool, len(s.pool))

	for _, ctx := range s.pool {
		if ctx.Dnn == dnn {
			seen[ctx.IPv6IID] = true
		}
	}

	s.mu.RUnlock()

	var iid [8]byte

	for i := 0; i < iIDMaxRetries; i++ {
		var err error

		iid, err = GenerateIID()
		if err != nil {
			return iid, err
		}

		if !seen[iid] {
			return iid, nil
		}
	}

	return iid, fmt.Errorf("failed to generate unique IID after %d retries", iIDMaxRetries)
}
