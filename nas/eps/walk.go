// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// walkOptionalIEs walks an EMM or ESM message's variable part under this
// generation's policy: an unrecognized full-octet IEI is skipped by the EMM/ESM
// rule of TS 24.007 §11.2.4 and preserved, and an element that fails to decode is
// treated as absent unless SecurityCriticalIE names it.
//
// Every message parser goes through here rather than calling the walker
// directly, so no message can pick a policy of its own.
func walkOptionalIEs(r *nas.Reader, table []nas.OptionalIE, emit func(iei uint8, value []byte) (bool, error)) ([]nas.RawIE, error) {
	return nas.Walker{Table: table, Unknown: nas.UnknownIESkipEPS, Emit: emit}.Walk(r)
}

// declineAll models no optional element. A message that acts on none of its
// optional elements walks with this, so every element it delimits is preserved
// and the message still re-encodes with everything it arrived with.
func declineAll(uint8, []byte) (bool, error) { return false, nil }
