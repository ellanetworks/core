// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/mintsites"
)

// TestAuthProofMintSites enforces that the AuthProof constructors are called
// only from the authorized files — the grep equivalent of "you cannot mint an
// AuthProof outside these files". It does NOT catch in-package AuthProof{}
// struct-literal abuses (the unexported field blocks that outside the mme
// package; inside it relies on reviewer vigilance). Any change to this list is a
// security-boundary change.
func TestAuthProofMintSites(t *testing.T) {
	mintsites.Check(t,
		map[string]map[string]struct{}{
			"MintAuthProofForSecurityMode": {
				"internal/mme/nas/emm.go": {},
			},
			"MintAuthProofForAttachCommit": {
				"internal/mme/nas/bearer.go": {},
			},
		},
		[]string{
			"internal/mme/security_state.go",
			"internal/mme/security_state_test.go",
		},
		true,
	)
}
