// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "fmt"

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

// NewUplinkCounter returns a counter for a security context whose uplink NAS
// COUNT is already known — one restored from storage, or taken over from another
// node — with lastAccepted as the count of the last message accepted under it.
//
// A context established here starts from the zero value instead: its first
// uplink message carries NAS COUNT zero (TS 24.301 §4.4.3.1, TS 24.501 §4.4.3.1).
func NewUplinkCounter(lastAccepted Count) UplinkCounter {
	return UplinkCounter{last: lastAccepted, accepted: true}
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

// Accepted reports whether any uplink message has been accepted under this
// security context yet, which says whether LastAccepted names a message.
func (u UplinkCounter) Accepted() bool { return u.accepted }

// Commit records count as accepted, once the integrity of its message has
// verified (TS 24.301 §4.4.3.3, TS 24.501 §4.4.3.3).
//
// It rejects a count Estimate could not have produced for that sequence number:
// committing anything else would move the window somewhere the replay check
// never sanctioned, and would let a caller that mixed up two connections' counts
// accept a replay.
func (u *UplinkCounter) Commit(count Count) error {
	if want := u.Estimate(count.SQN()); count != want {
		return fmt.Errorf("nas: NAS COUNT %#06x is not the %#06x expected for sequence number %#02x",
			uint32(count), uint32(want), count.SQN())
	}

	u.last = count
	u.accepted = true

	return nil
}

// Reset returns the counter to its state for a new NAS security context, whose
// first uplink message carries NAS COUNT zero (TS 24.301 §4.4.3.1,
// TS 24.501 §4.4.3.1).
func (u *UplinkCounter) Reset() {
	*u = UplinkCounter{}
}
