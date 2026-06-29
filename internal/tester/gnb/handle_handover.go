// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

func handleHandoverRequest(gnb *GnodeB, msg *ngapType.HandoverRequest) error {
	if msg == nil {
		return fmt.Errorf("HandoverRequest message is nil")
	}

	var amfUeNgapID int64

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Value.Present {
		case ngapType.HandoverRequestIEsPresentAMFUENGAPID:
			if ie.Value.AMFUENGAPID != nil {
				amfUeNgapID = ie.Value.AMFUENGAPID.Value
			}
		case ngapType.HandoverRequestIEsPresentPDUSessionResourceSetupListHOReq:
			if ie.Value.PDUSessionResourceSetupListHOReq != nil {
				for _, item := range ie.Value.PDUSessionResourceSetupListHOReq.List {
					pduSessionID := item.PDUSessionID.Value
					psi := &PDUSessionInformation{
						PDUSessionID: pduSessionID,
					}
					// Key by AMF UE NGAP ID: the target has not yet assigned a RAN UE NGAP ID.
					gnb.StorePDUSession(amfUeNgapID, psi)
				}
			}
		}
	}

	return nil
}
