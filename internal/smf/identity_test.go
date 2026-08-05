// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import "testing"

func TestSessionKeyRangesAreDisjoint(t *testing.T) {
	seen := make(map[uint8]SessionIdentity)

	for id := uint8(1); id <= 15; id++ {
		for _, identity := range []SessionIdentity{{PDUSessionID: id}, {EBI: id}} {
			key := identity.sessionKey()
			if prev, dup := seen[key]; dup {
				t.Errorf("%s and %s both key to %d", prev, identity, key)
			}

			seen[key] = identity
		}
	}
}

func TestSessionKeyPrefersPDUSessionID(t *testing.T) {
	both := SessionIdentity{PDUSessionID: 3, EBI: 7}

	// The PDU session identity is assigned first on either access, so keying on
	// it keeps the key — and the UE IP lease — stable when an EPS bearer
	// identity is added later.
	if got := both.sessionKey(); got != 3 {
		t.Errorf("sessionKey() = %d, want 3", got)
	}

	keys := both.sessionKeys()
	if len(keys) != 2 || keys[0] != 3 || keys[1] != epsBearerKey(7) {
		t.Errorf("sessionKeys() = %v, want [3 %d]", keys, epsBearerKey(7))
	}
}

func TestSessionIdentityValid(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   SessionIdentity
		want bool
	}{
		{"pdu session only", SessionIdentity{PDUSessionID: 1}, true},
		{"eps bearer only", SessionIdentity{EBI: 5}, true},
		{"both", SessionIdentity{PDUSessionID: 1, EBI: 5}, true},
		{"neither", SessionIdentity{}, false},
		{"pdu session out of range", SessionIdentity{PDUSessionID: 16}, false},
		{"eps bearer out of range", SessionIdentity{EBI: 16}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.id.valid(); got != tc.want {
				t.Errorf("valid() = %v, want %v", got, tc.want)
			}
		})
	}
}
