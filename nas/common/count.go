// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

// Count is a NAS COUNT: a 24-bit value formed as a NAS overflow counter (the 16
// most significant bits) concatenated with a NAS sequence number (the 8 least
// significant bits) (TS 24.301 §4.4.3.1 / TS 33.501 §6.4.3.1). It is held in the
// low 24 bits of a uint32; the 8 most significant bits are always zero.
type Count uint32

const countMask = 0x00ffffff

// MakeCount forms a NAS COUNT from an overflow counter and a sequence number.
func MakeCount(overflow uint16, sqn uint8) Count {
	return Count(uint32(overflow)<<8|uint32(sqn)) & countMask
}

// SQN returns the 8-bit NAS sequence number exchanged on the wire.
func (c Count) SQN() uint8 { return uint8(c) }

// Overflow returns the 16-bit NAS overflow counter.
func (c Count) Overflow() uint16 { return uint16(c >> 8) }

// Value returns the 32-bit input to the NAS integrity and ciphering algorithms:
// the 24-bit NAS COUNT padded with 8 zeros in the most significant bits
// (TS 24.301 §4.4.3.1).
func (c Count) Value() uint32 { return uint32(c) & countMask }

// Next returns the NAS COUNT for the following message: the sequence number
// increased by one, carrying into the overflow counter on wrap-around
// (TS 24.301 §4.4.3.1).
func (c Count) Next() Count { return (c + 1) & countMask }

// ReconcileUplink estimates the full NAS COUNT of a received uplink message from
// its 8-bit sequence number: when the sequence number regresses relative to the
// stored count, the overflow counter has wrapped and is incremented
// (TS 24.301 §4.4.3.1).
func (c Count) ReconcileUplink(recvSeq uint8) Count {
	overflow := c.Overflow()
	if recvSeq < c.SQN() {
		overflow++
	}

	return MakeCount(overflow, recvSeq)
}
