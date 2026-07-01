// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// BuildPDUSessionResourceModifyConfirmTransfer encodes the SMF's response to a
// PDU Session Resource Modify Indication Transfer (TS 38.413 §8.2.5.2): it
// returns the UL NG-U tunnel the NG-RAN sends uplink to and confirms the QoS
// flows whose downlink transport the NG-RAN moved.
func BuildPDUSessionResourceModifyConfirmTransfer(teid uint32, n3IPv4 netip.Addr, n3IPv6 netip.Addr, qfis []int64) ([]byte, error) {
	teidOct := make([]byte, 4)
	binary.BigEndian.PutUint32(teidOct, teid)

	transfer := ngapType.PDUSessionResourceModifyConfirmTransfer{}

	for _, qfi := range qfis {
		item := ngapType.QosFlowModifyConfirmItem{}
		item.QosFlowIdentifier.Value = qfi
		transfer.QosFlowModifyConfirmList.List = append(transfer.QosFlowModifyConfirmList.List, item)
	}

	transfer.ULNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transfer.ULNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)
	transfer.ULNGUUPTNLInformation.GTPTunnel.GTPTEID.Value = teidOct

	tla, err := encodeTransportLayerAddress(n3IPv4, n3IPv6)
	if err != nil {
		return nil, fmt.Errorf("encode transport layer address failed: %s", err)
	}

	transfer.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value = tla

	buf, err := aper.MarshalWithParams(transfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not encode pdu session resource modify confirm transfer: %s", err)
	}

	return buf, nil
}
