// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "fmt"

// PDUSessionID identifies a PDU session (TS 24.007 §11.2.3.1b). It is the 5GS
// counterpart of the EPS bearer identity. The value is 8 bits: 0 marks "no PDU
// session identity assigned", 1 to 15 name a session, and every other value is
// reserved — the note to table 11.2.3.1c.1 gives 64 to 95 to the core network,
// which puts them out of reach here.
type PDUSessionID uint8

// PDUSessionIDUnassigned marks a message that carries no PDU session identity
// (TS 24.007 §11.2.3.1b).
const PDUSessionIDUnassigned PDUSessionID = 0

// PDUSessionIDMax is the largest identity that names a PDU session.
const PDUSessionIDMax PDUSessionID = 15

// Assigned reports whether the identity names a PDU session, which excludes the
// unassigned value and the reserved range.
func (p PDUSessionID) Assigned() bool { return p >= 1 && p <= PDUSessionIDMax }

func (p PDUSessionID) String() string {
	switch {
	case p == PDUSessionIDUnassigned:
		return "unassigned"
	case p.Assigned():
		return fmt.Sprintf("%d", uint8(p))
	default:
		return fmt.Sprintf("reserved (%d)", uint8(p))
	}
}
