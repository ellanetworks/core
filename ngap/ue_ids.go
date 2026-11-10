// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/per"

func PeekAMFUENGAPID(pdu PDU) (AMFUENGAPID, bool) {
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
		if f.id != IDAMFUENGAPID && f.id != IDSourceAMFUENGAPID {
			continue
		}

		var id AMFUENGAPID

		if err := perIEDecode(f.value, &id); err != nil {
			return 0, false
		}

		return id, true
	}

	return 0, false
}
