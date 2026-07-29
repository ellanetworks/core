// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas"

// walkOptionalIEs walks a 5GMM or 5GSM message's variable part under this
// generation's policy: an unrecognized full-octet IEI is self-delimiting
// (TS 24.007 §11.2.4), so the walk steps over it and keeps going, and an element
// that fails to decode is treated as absent unless its table entry is Critical.
//
// Every message parser goes through here rather than calling the walker
// directly, so no message can pick a policy of its own.
func walkOptionalIEs(r *nas.Reader, table []nas.OptionalIE, emit func(iei uint8, value []byte) (bool, error)) ([]nas.RawIE, error) {
	return nas.Walker{Table: table, Unknown: nas.UnknownIESkip5GS, Emit: emit}.Walk(r)
}

// declineAll models no optional element. A message that acts on none of its
// optional elements walks with this, so every element it delimits is preserved
// and the message still re-encodes with everything it arrived with.
func declineAll(uint8, []byte) (bool, error) { return false, nil }
