// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import "testing"

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

func TestSessionKeyPrefersThePDUSessionIdentity(t *testing.T) {
	both := SessionIdentity{PDUSessionID: 3, EBI: 7}

	if got := both.sessionKey(); got != 3 {
		t.Errorf("sessionKey() = %d, want the PDU session identity 3", got)
	}

	if got := (SessionIdentity{EBI: 7}).sessionKey(); got != epsBearerKey(7) {
		t.Errorf("sessionKey() = %d, want the EPS bearer key %d", got, epsBearerKey(7))
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
		{"no identity at all", SessionIdentity{}, false},
		{"lowest PDU session identity", SessionIdentity{PDUSessionID: 1}, true},
		{"highest PDU session identity", SessionIdentity{PDUSessionID: 15}, true},
		{"PDU session identity above the range a UE allocates", SessionIdentity{PDUSessionID: 16}, false},
		{"lowest default bearer", SessionIdentity{EBI: 5}, true},
		{"highest default bearer", SessionIdentity{EBI: 15}, true},
		{"reserved EPS bearer identity", SessionIdentity{EBI: 4}, false},
		{"EPS bearer identity above the range", SessionIdentity{EBI: 16}, false},
		{"both halves", SessionIdentity{PDUSessionID: 3, EBI: 7}, true},
		{"good PDU session identity, bad EPS half", SessionIdentity{PDUSessionID: 3, EBI: 2}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.id.valid(); got != tc.want {
				t.Errorf("%s.valid() = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}
