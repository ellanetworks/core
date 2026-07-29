// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "fmt"

// EPSBearerIdentity identifies an EPS bearer (TS 24.007 §11.2.3.1.5). It is the
// EPS counterpart of the 5GS PDU session identity. The value is 4 bits: 0 marks
// "no EPS bearer identity assigned" and 1 to 15 name a bearer.
//
// TS 24.301 §9.3.2 has the MME allocate from 5 to 15 only when the UE or the MME
// does not support fifteen bearer contexts, which is a selection rule rather than
// a coding one — 1 to 4 are valid identities either way.
type EPSBearerIdentity uint8

// EPSBearerIdentityUnassigned marks a message that carries no EPS bearer
// identity (TS 24.007 §11.2.3.1.5).
const EPSBearerIdentityUnassigned EPSBearerIdentity = 0

// EPSBearerIdentityMax is the largest identity that names a bearer.
const EPSBearerIdentityMax EPSBearerIdentity = 15

// Assigned reports whether the identity names an EPS bearer, which excludes only
// the unassigned value.
func (e EPSBearerIdentity) Assigned() bool { return e >= 1 && e <= EPSBearerIdentityMax }

func (e EPSBearerIdentity) String() string {
	switch {
	case e == EPSBearerIdentityUnassigned:
		return "unassigned"
	case e.Assigned():
		return fmt.Sprintf("%d", uint8(e))
	default:
		return fmt.Sprintf("reserved (%d)", uint8(e))
	}
}
