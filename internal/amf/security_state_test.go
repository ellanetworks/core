// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/mintsites"
)

// TestAuthProofMintSites enforces that the AuthProof constructors are called
// only from the authorized files — the grep equivalent of "you cannot mint an
// AuthProof outside these files". It does NOT catch in-package AuthProof{}
// struct-literal abuses (the unexported field blocks that outside the amf
// package; inside it relies on reviewer vigilance). Any change to this list is a
// security-boundary change.
func TestAuthProofMintSites(t *testing.T) {
	mintsites.Check(t,
		map[string]map[string]struct{}{
			"MintAuthProofForSMC": {
				"internal/amf/nas/handle_security_mode_complete.go": {},
			},
			"MintAuthProofForRegistrationRequest": {
				"internal/amf/nas/handle_registration_request.go": {},
			},
		},
		[]string{
			"internal/amf/security_state.go",
			"internal/amf/security_state_test.go",
		},
		false,
	)
}
