// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	libngap "github.com/ellanetworks/core/ngap"
)

// BuildPDUSessionResourceModifyRequestTransfer builds the N2 SM Information for a
// PDU Session Resource Modify Request (TS 38.413).
func BuildPDUSessionResourceModifyRequestTransfer(ambr *models.Ambr, qosData *models.QosData) ([]byte, error) {
	var transfer libngap.PDUSessionResourceModifyRequestTransfer

	if ambr != nil {
		transfer.PDUSessionAggregateMaximumBitRate = sessionAMBR(ambr)
	}

	if qosData != nil {
		params, err := qosFlowLevelQosParameters(qosData)
		if err != nil {
			return nil, err
		}

		transfer.QosFlowAddOrModifyRequest = libngap.QosFlowAddOrModifyRequestList{{
			QosFlowIdentifier:         libngap.QosFlowIdentifier(qosData.QFI),
			QosFlowLevelQosParameters: &params,
		}}
	}

	if ambr == nil && qosData == nil {
		return nil, fmt.Errorf("no IEs to encode in PDU Session Resource Modify Request Transfer")
	}

	buf, err := transfer.Marshal()
	if err != nil {
		return nil, fmt.Errorf("encode PDU Session Resource Modify Request Transfer: %w", err)
	}

	return buf, nil
}
