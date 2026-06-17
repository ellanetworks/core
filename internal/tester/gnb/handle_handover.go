// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package gnb

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

// handleHandoverRequest handles an incoming HandoverRequest (InitiatingMessage)
// from the AMF to the target gNB. The target gNB stores the PDU session info
// and allows the scenario to respond with HandoverRequestAcknowledge.
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
					// Store basic PDU session info from the handover request.
					// The detailed tunnel info will be decoded from the
					// HandoverRequestAcknowledge flow.
					pduSessionID := item.PDUSessionID.Value
					psi := &PDUSessionInformation{
						PDUSessionID: pduSessionID,
					}
					// Store under AMF UE NGAP ID since the target doesn't have
					// a RAN UE NGAP ID yet (it assigns one).
					gnb.StorePDUSession(amfUeNgapID, psi)
				}
			}
		}
	}

	return nil
}
