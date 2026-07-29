// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "errors"

// ErrCountExhausted reports a NAS COUNT that has reached its maximum. Both specs
// forbid reusing a COUNT under the same key (TS 33.501 §6.4.3.1, TS 33.401
// §6.5), so the connection has to be released and a new security context
// established rather than the counter wrapped.
var ErrCountExhausted = errors.New("NAS COUNT exhausted")

// DownlinkCounter is the sender side of the downlink NAS COUNT of a NAS security
// context: the count the next message it protects will carry.
//
// It is the sender's half of the guarantee the receiver's [UplinkCounter] holds
// for the other direction — a COUNT is used once — so the two directions of a
// context are symmetric, as are the 4G and 5G sides of each.
type DownlinkCounter struct {
	next      Count
	exhausted bool
}

// NewDownlinkCounter returns a counter for a security context whose downlink NAS
// COUNT is already known — one restored from storage, or taken over from another
// node — with next as the count its next protected message must carry.
//
// A context established here starts from the zero value instead: its first
// downlink message carries NAS COUNT zero (TS 24.301 §4.4.3.1,
// TS 24.501 §4.4.3.1).
func NewDownlinkCounter(next Count) DownlinkCounter {
	return DownlinkCounter{next: next}
}

// Use returns the NAS COUNT to protect the next downlink message with and
// advances the counter. It returns [ErrCountExhausted] rather than wrapping.
func (d *DownlinkCounter) Use() (Count, error) {
	if d.exhausted {
		return 0, ErrCountExhausted
	}

	count := d.next
	if count == countMask {
		d.exhausted = true
	}

	d.next = count.Next()

	return count, nil
}

// Next returns the NAS COUNT the next protected message will carry, without
// advancing the counter. It is the K_eNB/K_gNB derivation input on the downlink
// (TS 33.401 §A.3, TS 33.501 §A.9).
func (d DownlinkCounter) Next() Count { return d.next }

// Exhausted reports whether every NAS COUNT has been used, after which no
// further message can be protected under this security context.
func (d DownlinkCounter) Exhausted() bool { return d.exhausted }

// Reset returns the counter to its state for a new NAS security context, whose
// first downlink message carries NAS COUNT zero (TS 24.301 §4.4.3.1,
// TS 24.501 §4.4.3.1).
func (d *DownlinkCounter) Reset() { *d = DownlinkCounter{} }
