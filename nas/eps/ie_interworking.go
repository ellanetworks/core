// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "fmt"

// UEStatus is the UE status information element (TS 24.301 §9.9.3.54, which
// defers to TS 24.501 §9.11.3.56): the UE's registration state in each system.
// It is what tells the MME an ATTACH or TRACKING AREA UPDATE is an inter-system
// move rather than a fresh arrival — TS 24.301 §5.5.3.2.2 case zd has a UE
// handed over from 5GS report "UE is in 5GMM-REGISTERED state" in the TAU that
// follows.
//
// It is the EPS counterpart of fgs.UEStatus, and the two are the same element:
// TS 24.301 defines it by reference. They stay separate types because neither
// generation's codec reaches across the boundary.
type UEStatus struct {
	// S1ModeReg reports the UE is in EMM-REGISTERED state (octet 3, bit 1).
	S1ModeReg bool
	// N1ModeReg reports the UE is in 5GMM-REGISTERED state (octet 3, bit 2).
	N1ModeReg bool
}

// ParseUEStatus decodes a UE status IE value.
func ParseUEStatus(b []byte) (UEStatus, error) {
	if len(b) != 1 {
		return UEStatus{}, fmt.Errorf("nas/eps: UE status is %d octets, want 1", len(b))
	}

	return UEStatus{S1ModeReg: b[0]&0x01 != 0, N1ModeReg: b[0]&0x02 != 0}, nil
}

// MarshalBinary encodes the UE status IE value.
func (u UEStatus) MarshalBinary() []byte {
	return []byte{boolBit(u.S1ModeReg, 0) | boolBit(u.N1ModeReg, 1)}
}
