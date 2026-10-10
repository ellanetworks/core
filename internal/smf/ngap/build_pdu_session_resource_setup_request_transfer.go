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

func BuildPDUSessionResourceSetupRequestTransfer(ambr *models.Ambr, qosData *models.QosData, teid uint32, n3IPv4 netip.Addr, n3IPv6 netip.Addr, pduSessionType libngap.PDUSessionType, erabID uint8, flows []GBRQoSFlow) ([]byte, error) {
	transfer, err := pduSessionResourceSetupRequestTransfer(ambr, qosData, teid, n3IPv4, n3IPv6, pduSessionType, erabID, flows)
	if err != nil {
		return nil, err
	}

	return marshalPDUSessionResourceSetupRequestTransfer(transfer)
}

type DataForwarding uint8

const (
	DataForwardingNone DataForwarding = iota
	DataForwardingDirect
	DataForwardingIndirect
)

func BuildHandoverRequestTransfer(ambr *models.Ambr, qosData *models.QosData, teid uint32, n3IPv4 netip.Addr, n3IPv6 netip.Addr, pduSessionType libngap.PDUSessionType, erabID uint8, forwarding DataForwarding, flows []GBRQoSFlow) ([]byte, error) {
	transfer, err := pduSessionResourceSetupRequestTransfer(ambr, qosData, teid, n3IPv4, n3IPv6, pduSessionType, erabID, flows)
	if err != nil {
		return nil, err
	}

	switch forwarding {
	case DataForwardingDirect:
		transfer.DirectForwardingPathAvailability = libngap.Ptr(libngap.DirectForwardingPathAvailable)
	case DataForwardingIndirect:
	case DataForwardingNone:
		transfer.DataForwardingNotPossible = libngap.Ptr(libngap.DataForwardingNotPossibleTrue)
	}

	return marshalPDUSessionResourceSetupRequestTransfer(transfer)
}

func marshalPDUSessionResourceSetupRequestTransfer(transfer *libngap.PDUSessionResourceSetupRequestTransfer) ([]byte, error) {
	buf, err := transfer.Marshal()
	if err != nil {
		return nil, fmt.Errorf("encode resourceSetupRequestTransfer failed: %s", err)
	}

	return buf, nil
}

func pduSessionResourceSetupRequestTransfer(ambr *models.Ambr, qosData *models.QosData, teid uint32, n3IPv4 netip.Addr, n3IPv6 netip.Addr, pduSessionType libngap.PDUSessionType, erabID uint8, flows []GBRQoSFlow) (*libngap.PDUSessionResourceSetupRequestTransfer, error) {
	if ambr == nil {
		return nil, fmt.Errorf("ambr is nil")
	}

	tla, err := encodeTransportLayerAddress(n3IPv4, n3IPv6)
	if err != nil {
		return nil, fmt.Errorf("encode transport layer address failed: %s", err)
	}

	transfer := &libngap.PDUSessionResourceSetupRequestTransfer{
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

		id, err := optionalERABID(erabID)
		if err != nil {
			return nil, err
		}

		transfer.QosFlowSetupRequest = libngap.QosFlowSetupRequestList{{
			QosFlowIdentifier:         libngap.QosFlowIdentifier(qosData.QFI),
			QosFlowLevelQosParameters: params,
			ERABID:                    id,
		}}
	}

	for _, f := range flows {
		params, err := gbrQosFlowLevelQosParameters(f)
		if err != nil {
			return nil, fmt.Errorf("QoS flow %d: %w", f.QFI, err)
		}

		id, err := optionalERABID(f.ERABID)
		if err != nil {
			return nil, fmt.Errorf("QoS flow %d: %w", f.QFI, err)
		}

		transfer.QosFlowSetupRequest = append(transfer.QosFlowSetupRequest, libngap.QosFlowSetupRequestItem{
			QosFlowIdentifier:         libngap.QosFlowIdentifier(f.QFI),
			QosFlowLevelQosParameters: params,
			ERABID:                    id,
		})
	}

	return transfer, nil
}

func optionalERABID(ebi uint8) (*libngap.ERABID, error) {
	switch {
	case ebi == 0:
		return nil, nil
	case ebi > 15:
		return nil, fmt.Errorf("EPS bearer identity %d does not fit the E-RAB ID", ebi)
	default:
		return libngap.Ptr(libngap.ERABID(ebi)), nil
	}
}
