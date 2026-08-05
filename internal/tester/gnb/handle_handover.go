// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// handleHandoverRequest records the sessions the AMF asks the target node to
// take over (TS 38.413 §8.4.2). They are keyed by AMF UE NGAP ID: the target
// has not yet assigned a RAN UE NGAP ID.
func handleHandoverRequest(gnb *GnodeB, value []byte) error {
	msg, err := ngap.ParseHandoverRequest(value)
	if err != nil {
		return fmt.Errorf("undecodable HandoverRequest: %w", err)
	}

	for _, item := range msg.PDUSessionResourceSetupListHOReq {
		gnb.StorePDUSession(int64(msg.AMFUENGAPID), &PDUSessionInformation{
			PDUSessionID: int64(item.PDUSessionID),
		})
	}

	return nil
}
