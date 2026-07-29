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

// tv1Value returns the low-nibble value the walker emits for a type-1 IE, or nil
// if the walker passed no value octet.
func tv1Value(value []byte) *uint8 {
	if len(value) == 0 {
		return nil
	}

	v := value[0] & 0x0F

	return &v
}

// tv1Flag returns bit 1 of the value the walker emits for a single-bit type-1
// IE, or nil if the walker passed no value octet. Both values of such a bit are
// assigned, so an element that arrived carrying zero is present and false, not
// absent.
func tv1Flag(value []byte) *bool {
	v := tv1Value(value)
	if v == nil {
		return nil
	}

	set := *v&0x01 != 0

	return &set
}
