// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// ExtractAmfUeNgapIDFromHandoverRequest reads the AMF UE NGAP ID the AMF
// assigned for a handover, which the target node needs before it has one of its
// own (TS 38.413 §8.4.2).
func ExtractAmfUeNgapIDFromHandoverRequest(data []byte) (int64, error) {
	pdu, err := ngap.Unmarshal(data)
	if err != nil {
		return 0, fmt.Errorf("decode NGAP PDU: %w", err)
	}

	im, ok := pdu.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != ngap.ProcHandoverResourceAllocation {
		return 0, fmt.Errorf("not a HandoverRequest message")
	}

	msg, err := ngap.ParseHandoverRequest(im.Value)
	if err != nil {
		return 0, fmt.Errorf("parse HandoverRequest: %w", err)
	}

	return int64(msg.AMFUENGAPID), nil
}
