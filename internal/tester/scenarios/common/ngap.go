// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"fmt"

	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func ExtractAmfUeNgapIDFromHandoverRequest(data []byte) (int64, error) {
	pdu, err := ngap.Decoder(data)
	if err != nil {
		return 0, fmt.Errorf("decode NGAP PDU: %w", err)
	}

	if pdu.Present != ngapType.NGAPPDUPresentInitiatingMessage {
		return 0, fmt.Errorf("expected InitiatingMessage, got %d", pdu.Present)
	}

	if pdu.InitiatingMessage == nil ||
		pdu.InitiatingMessage.Value.Present != ngapType.InitiatingMessagePresentHandoverRequest {
		return 0, fmt.Errorf("not a HandoverRequest message")
	}

	msg := pdu.InitiatingMessage.Value.HandoverRequest
	if msg == nil {
		return 0, fmt.Errorf("HandoverRequest is nil")
	}

	for _, ie := range msg.ProtocolIEs.List {
		if ie.Value.Present == ngapType.HandoverRequestIEsPresentAMFUENGAPID {
			if ie.Value.AMFUENGAPID != nil {
				return ie.Value.AMFUENGAPID.Value, nil
			}
		}
	}

	return 0, fmt.Errorf("AMF UE NGAP ID not found in HandoverRequest")
}
