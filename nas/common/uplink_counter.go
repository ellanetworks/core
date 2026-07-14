// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

// UplinkCounter is the receiver side of the uplink NAS COUNT of a NAS security
// context: the count of the last accepted message, and whether any message has
// been accepted yet.
//
// It holds replay protection (TS 24.301 §4.4.3.2, TS 24.501 §4.4.3.2): a given
// NAS COUNT is accepted at most once. Except across NAS COUNT wrap-around,
// Estimate never returns a count at or below the last accepted one, so a
// replayed message estimates to a count whose MAC cannot verify. Both specs
// leave the mechanism to the implementation and require the same result of it,
// so the 4G and 5G receivers share this type.
type UplinkCounter struct {
	last     Count
	accepted bool
}

// NextExpected returns the NAS COUNT the next uplink message must carry.
func (u UplinkCounter) NextExpected() Count {
	if !u.accepted {
		return 0
	}

	return u.last.Next()
}

// Estimate returns the NAS COUNT to verify a received uplink message against,
// formed from its sequence number and an estimate of the overflow counter
// (TS 24.301 §4.4.3.1, TS 24.501 §4.4.3.1).
func (u UplinkCounter) Estimate(recvSeq uint8) Count {
	return u.NextExpected().reconcileUplink(recvSeq)
}

// LastAccepted returns the NAS COUNT of the most recently accepted message, zero
// if none has been. It is the K_eNB/K_gNB derivation input (TS 33.401 §A.3,
// TS 33.501 §A.9) and the count an uplink NAS message container is ciphered
// with.
func (u UplinkCounter) LastAccepted() Count {
	return u.last
}

// Commit records count as accepted, once the integrity of its message has
// verified (TS 24.301 §4.4.3.3, TS 24.501 §4.4.3.3).
func (u *UplinkCounter) Commit(count Count) {
	u.last = count
	u.accepted = true
}

// Reset returns the counter to its state for a new NAS security context, whose
// first uplink message carries NAS COUNT zero (TS 24.301 §4.4.3.1,
// TS 24.501 §4.4.3.1).
func (u *UplinkCounter) Reset() {
	*u = UplinkCounter{}
}
