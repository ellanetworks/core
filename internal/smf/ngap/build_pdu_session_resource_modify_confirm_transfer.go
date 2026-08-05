// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"
	"net/netip"

	libngap "github.com/ellanetworks/core/ngap"
)

// BuildPDUSessionResourceModifyConfirmTransfer encodes the UL NG-U tunnel and
// confirmed QoS flows for a PDU Session Resource Modify Confirm (TS 38.413 §8.2.5.2).
func BuildPDUSessionResourceModifyConfirmTransfer(teid uint32, n3IPv4 netip.Addr, n3IPv6 netip.Addr, qfis []int64) ([]byte, error) {
	tla, err := encodeTransportLayerAddress(n3IPv4, n3IPv6)
	if err != nil {
		return nil, fmt.Errorf("encode transport layer address failed: %s", err)
	}

	transfer := libngap.PDUSessionResourceModifyConfirmTransfer{
		ULNGUUPTNLInformation: libngap.UPTransportLayerInformation{
			GTPTunnel: libngap.GTPTunnel{TransportLayerAddress: tla, GTPTEID: libngap.GTPTEID(teid)},
		},
	}

	for _, qfi := range qfis {
		transfer.QosFlowModifyConfirm = append(transfer.QosFlowModifyConfirm,
			libngap.QosFlowModifyConfirmItem{QosFlowIdentifier: libngap.QosFlowIdentifier(qfi)})
	}

	buf, err := transfer.Marshal()
	if err != nil {
		return nil, fmt.Errorf("could not encode pdu session resource modify confirm transfer: %s", err)
	}

	return buf, nil
}
