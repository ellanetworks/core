// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/per"

func PeekMMEUES1APID(pdu PDU) (MMEUES1APID, bool) {
	if pdu == nil {
		return 0, false
	}

	r := per.NewReader(pdu.value())

	if _, err := r.ReadBit(); err != nil {
		return 0, false
	}

	fields, err := decodeIEContainer(r, per.Aligned)
	if err != nil {
		return 0, false
	}

	for _, f := range fields {
		if f.id != IDMMEUES1APID && f.id != IDSourceMMEUES1APID {
			continue
		}

		var id MMEUES1APID

		if err := perIEDecode(f.value, &id); err != nil {
			return 0, false
		}

		return id, true
	}

	return 0, false
}
