// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import "testing"

// The two namespaces run 1..15 and are held at once by a converged anchor
// (TS 23.501 §5.17.2), so the key that names a session's slot must separate
// them. The EPS half is aliased into 64..95, which TS 29.571 §5.2.2 reserves for
// core-network allocation and a UE therefore never picks.
func TestSessionKeySeparatesTheNamespaces(t *testing.T) {
	for id := uint8(1); id <= 15; id++ {
		pdu := SessionIdentity{PDUSessionID: id}.sessionKey()

		if id >= 5 {
			if eps := (SessionIdentity{EBI: id}).sessionKey(); eps == pdu {
				t.Errorf("PDU session identity %d and EPS bearer identity %d share key %d", id, id, pdu)
			}
		}

		if pdu != id {
			t.Errorf("PDU session identity %d keyed as %d, want it unchanged", id, pdu)
		}
	}
}

// A session that holds both identities keeps the key it was created under, so
// gaining the second identity does not re-key it — the key is also the UE IP
// lease key, and re-keying would strand the address.
func TestSessionKeyPrefersThePDUSessionIdentity(t *testing.T) {
	both := SessionIdentity{PDUSessionID: 3, EBI: 7}

	if got := both.sessionKey(); got != 3 {
		t.Errorf("sessionKey() = %d, want the PDU session identity 3", got)
	}

	if got := (SessionIdentity{EBI: 7}).sessionKey(); got != epsBearerKey(7) {
		t.Errorf("sessionKey() = %d, want the EPS bearer key %d", got, epsBearerKey(7))
	}

	// Both keys index the session, so a lookup in either namespace resolves it.
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
		{"no identity at all", SessionIdentity{}, false},
		{"lowest PDU session identity", SessionIdentity{PDUSessionID: 1}, true},
		{"highest PDU session identity", SessionIdentity{PDUSessionID: 15}, true},
		{"PDU session identity above the range a UE allocates", SessionIdentity{PDUSessionID: 16}, false},
		{"lowest default bearer", SessionIdentity{EBI: 5}, true},
		{"highest default bearer", SessionIdentity{EBI: 15}, true},
		// TS 24.301 §6.1.1: 1..4 are reserved for a network that does not support
		// 15 EPS bearer contexts, which this MME does not advertise.
		{"reserved EPS bearer identity", SessionIdentity{EBI: 4}, false},
		{"EPS bearer identity above the range", SessionIdentity{EBI: 16}, false},
		{"both halves", SessionIdentity{PDUSessionID: 3, EBI: 7}, true},
		// A transferable PDN connection carries a usable PDU session identity; a
		// malformed EPS half still invalidates it, since the session would be
		// indexed under a key no allocator owns.
		{"good PDU session identity, bad EPS half", SessionIdentity{PDUSessionID: 3, EBI: 2}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.id.valid(); got != tc.want {
				t.Errorf("%s.valid() = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}
