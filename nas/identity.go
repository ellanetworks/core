// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "fmt"

// ProcedureTransactionIdentity identifies a session-management transaction
// within a NAS session-management message (TS 24.007 §11.2.3.1a). It is the same
// field in EPS ESM and 5GS 5GSM, so it is defined once here and used by both.
//
// The value is 8 bits. 0 marks "no procedure transaction identity assigned" and
// 255 is reserved, so a transaction uses 1-254.
type ProcedureTransactionIdentity uint8

// Reserved procedure transaction identity values (TS 24.007 §11.2.3.1a).
const (
	PTIUnassigned ProcedureTransactionIdentity = 0
	PTIReserved   ProcedureTransactionIdentity = 255
)

// Assigned reports whether the identity names a transaction, which excludes the
// unassigned and reserved values.
func (p ProcedureTransactionIdentity) Assigned() bool {
	return p != PTIUnassigned && p != PTIReserved
}

func (p ProcedureTransactionIdentity) String() string {
	switch p {
	case PTIUnassigned:
		return "unassigned"
	case PTIReserved:
		return "reserved"
	default:
		return fmt.Sprintf("%d", uint8(p))
	}
}
