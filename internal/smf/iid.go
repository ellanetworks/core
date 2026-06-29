// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"crypto/rand"
	"fmt"
)

// GenerateIID returns a cryptographically random 64-bit IPv6 Interface
// Identifier sent to the UE in the PDU Session Establishment Accept (TS 24.501).
func GenerateIID() ([8]byte, error) {
	var iid [8]byte

	if _, err := rand.Read(iid[:]); err != nil {
		return iid, fmt.Errorf("generate IID: %w", err)
	}

	return iid, nil
}

// iIDMaxRetries bounds the unique-IID search so a pathological run of collisions
// cannot loop forever; collisions in a 64-bit space are already astronomically rare.
const iIDMaxRetries = 3

// assignIID returns an IID not already in use by another session on the DNN.
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
