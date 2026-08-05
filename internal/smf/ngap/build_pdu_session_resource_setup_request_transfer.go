// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/models"
	libngap "github.com/ellanetworks/core/ngap"
)

func BuildPDUSessionResourceSetupRequestTransfer(ambr *models.Ambr, qosData *models.QosData, teid uint32, n3IPv4 netip.Addr, n3IPv6 netip.Addr, pduSessionType libngap.PDUSessionType) ([]byte, error) {
	if ambr == nil {
		return nil, fmt.Errorf("ambr is nil")
	}

	tla, err := encodeTransportLayerAddress(n3IPv4, n3IPv6)
	if err != nil {
		return nil, fmt.Errorf("encode transport layer address failed: %s", err)
	}

	transfer := libngap.PDUSessionResourceSetupRequestTransfer{
		// Conditional: present when at least one non-GBR QoS flow is set up.
		PDUSessionAggregateMaximumBitRate: sessionAMBR(ambr),
		ULNGUUPTNLInformation: libngap.UPTransportLayerInformation{
			GTPTunnel: libngap.GTPTunnel{TransportLayerAddress: tla, GTPTEID: libngap.GTPTEID(teid)},
		},
		PDUSessionType: pduSessionType,
	}

	if qosData != nil {
		params, err := qosFlowLevelQosParameters(qosData)
		if err != nil {
			return nil, err
		}

		transfer.QosFlowSetupRequest = libngap.QosFlowSetupRequestList{{
			QosFlowIdentifier:         libngap.QosFlowIdentifier(qosData.QFI),
			QosFlowLevelQosParameters: params,
		}}
	}

	buf, err := transfer.Marshal()
	if err != nil {
		return nil, fmt.Errorf("encode resourceSetupRequestTransfer failed: %s", err)
	}

	return buf, nil
}
