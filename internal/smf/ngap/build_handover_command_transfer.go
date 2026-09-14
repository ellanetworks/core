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

type ForwardingPlan struct {
	DLForwardingUPTNLInformation *libngap.UPTransportLayerInformation
	QosFlowToBeForwarded         libngap.QosFlowToBeForwardedList
	DataForwardingResponseDRB    libngap.DataForwardingResponseDRBList
}

func (p *ForwardingPlan) Forwards() bool {
	if p == nil {
		return false
	}

	return p.DLForwardingUPTNLInformation != nil || len(p.QosFlowToBeForwarded) > 0 || len(p.DataForwardingResponseDRB) > 0
}

// Every IE of the transfer is optional (TS 38.413 §9.3.4.10).
func BuildHandoverCommandTransfer(plan *ForwardingPlan) ([]byte, error) {
	transfer := libngap.HandoverCommandTransfer{}

	if plan != nil {
		transfer.DLForwardingUPTNLInformation = plan.DLForwardingUPTNLInformation
		transfer.QosFlowToBeForwarded = plan.QosFlowToBeForwarded
		transfer.DataForwardingResponseDRB = plan.DataForwardingResponseDRB
	}

	buf, err := transfer.Marshal()
	if err != nil {
		return nil, fmt.Errorf("could not encode handover command transfer: %s", err)
	}

	return buf, nil
}

func (p *ForwardingPlan) RelayThrough(teid uint32, addr libngap.TransportLayerAddress) {
	p.DLForwardingUPTNLInformation = &libngap.UPTransportLayerInformation{
		GTPTunnel: libngap.GTPTunnel{
			TransportLayerAddress: addr,
			GTPTEID:               libngap.GTPTEID(teid),
		},
	}
	p.DataForwardingResponseDRB = nil
}

func EncodeTransportLayerAddress(ipv4, ipv6 netip.Addr) (libngap.TransportLayerAddress, error) {
	return encodeTransportLayerAddress(ipv4, ipv6)
}

func ForwardingPlanFrom(ack *libngap.HandoverRequestAcknowledgeTransfer) *ForwardingPlan {
	if ack == nil {
		return nil
	}

	plan := &ForwardingPlan{
		DLForwardingUPTNLInformation: forwardingTNL(ack.DLForwardingUPTNLInformation),
	}

	for _, drb := range ack.DataForwardingResponseDRB {
		drb.DLForwardingUPTNLInformation = forwardingTNL(drb.DLForwardingUPTNLInformation)
		drb.ULForwardingUPTNLInformation = forwardingTNL(drb.ULForwardingUPTNLInformation)

		if drb.DLForwardingUPTNLInformation == nil && drb.ULForwardingUPTNLInformation == nil {
			continue
		}

		plan.DataForwardingResponseDRB = append(plan.DataForwardingResponseDRB, drb)
	}

	for _, flow := range ack.QosFlowSetupResponse {
		if flow.DataForwardingAccepted == nil {
			continue
		}

		plan.QosFlowToBeForwarded = append(plan.QosFlowToBeForwarded,
			libngap.QosFlowToBeForwardedItem{QosFlowIdentifier: flow.QosFlowIdentifier})
	}

	return plan
}

func forwardingTNL(tnl *libngap.UPTransportLayerInformation) *libngap.UPTransportLayerInformation {
	if tnl == nil || !tnl.GTPTunnel.TransportLayerAddress.Valid() {
		return nil
	}

	return tnl
}
