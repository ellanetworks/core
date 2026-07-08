// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/free5gc/ngap/ngapType"

func validPDUSessionID(id int64) (uint8, bool) {
	if id < 1 || id > 15 {
		return 0, false
	}

	return uint8(id), true
}

// duplicatePDUSessionID returns the first PDU Session ID appearing more than once
// in the to-be-switched downlink list; TS 38.413 requires the AMF to reject such a
// Path Switch Request with a Failure.
func duplicatePDUSessionID(items []ngapType.PDUSessionResourceToBeSwitchedDLItem) (int64, bool) {
	seen := make(map[int64]struct{}, len(items))

	for _, item := range items {
		id := item.PDUSessionID.Value
		if _, ok := seen[id]; ok {
			return id, true
		}

		seen[id] = struct{}{}
	}

	return 0, false
}
