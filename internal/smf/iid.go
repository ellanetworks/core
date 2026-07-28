// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"crypto/rand"
	"fmt"
)

// GenerateIID returns a cryptographically random 64-bit IPv6 Interface
// Identifier sent to the UE in the PDU Session Establishment Accept (TS 24.501).
// Each session has its own /64, so the IID needs no cross-session uniqueness.
func GenerateIID() ([8]byte, error) {
	var iid [8]byte

	if _, err := rand.Read(iid[:]); err != nil {
		return iid, fmt.Errorf("generate IID: %w", err)
	}

	return iid, nil
}
