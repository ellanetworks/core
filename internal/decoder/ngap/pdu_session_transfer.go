// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

// libSetupRequestTransfer renders the PDU Session Resource Setup Request
// Transfer both the setup request and the initial context setup carry per
// session (TS 38.413 §9.3.4.1).
func libSetupRequestTransfer(raw ngap.TransferContainer) (*PDUSessionResourceSetupRequestTransfer, error) {
	t, err := ngap.ParsePDUSessionResourceSetupRequestTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionResourceSetupRequestTransfer{
		ULNGUUPTNLInformation: &ULNGUUPTNLInformation{
			GTPTunnel: libGTPTunnel(t.ULNGUUPTNLInformation),
		},
		PduSType: ngap.Ptr(libPDUSessionType(t.PDUSessionType)),
	}

	for _, flow := range t.QosFlowSetupRequest {
		out.QosFlowSetupRequestList = append(out.QosFlowSetupRequestList, libQosFlowSetupRequest(flow))
	}

	if t.PDUSessionAggregateMaximumBitRate != nil {
		out.MaximumBitRate = &MaximumBitRate{
			UplinkNAggregateMaximumBitRate:   uint64(t.PDUSessionAggregateMaximumBitRate.UL),
			DownlinkNAggregateMaximumBitRate: uint64(t.PDUSessionAggregateMaximumBitRate.DL),
			Unit:                             "bps",
		}
	}

	if t.SecurityIndication != nil {
		out.SecurityIndication = makeUnsupportedIE()
	}

	for _, ie := range t.UnknownIEs() {
		out.UnsupportedIEs = append(out.UnsupportedIEs, fmt.Sprintf("unsupported ie type %d", ie.ID))
	}

	return out, nil
}

func libQosFlowSetupRequest(flow ngap.QosFlowSetupRequestItem) QosFlowSetupRequest {
	params := flow.QosFlowLevelQosParameters

	entry := QosFlowSetupRequest{
		QosId:  int64(flow.QosFlowIdentifier),
		PriArp: int64(params.AllocationAndRetentionPriority.PriorityLevelARP),
	}

	switch params.QosCharacteristics.Kind {
	case ngap.QosCharacteristicsNonDynamic5QI:
		entry.FiveQi = ngap.Ptr(int64(params.QosCharacteristics.NonDynamic5QI.FiveQI))
	case ngap.QosCharacteristicsDynamic5QI:
		dyn := params.QosCharacteristics.Dynamic5QI
		entry.Dynamic = true
		entry.PriorityLevelQos = ngap.Ptr(int64(dyn.PriorityLevelQos))
		entry.PacketDelayBudget = ngap.Ptr(int64(dyn.PacketDelayBudget))

		if dyn.FiveQI != nil {
			entry.FiveQi = ngap.Ptr(int64(*dyn.FiveQI))
		}
	}

	if params.GBRQosInformation != nil {
		entry.GBRQosInformation = &GBRQosInfo{
			MaximumFlowBitRateDL:    int64(params.GBRQosInformation.MaximumFlowBitRateDL),
			MaximumFlowBitRateUL:    int64(params.GBRQosInformation.MaximumFlowBitRateUL),
			GuaranteedFlowBitRateDL: int64(params.GBRQosInformation.GuaranteedFlowBitRateDL),
			GuaranteedFlowBitRateUL: int64(params.GBRQosInformation.GuaranteedFlowBitRateUL),
		}
	}

	return entry
}

func libPDUSessionType(t ngap.PDUSessionType) utils.EnumField {
	switch t {
	case ngap.PDUSessionTypeIPv4:
		return utils.MakeEnum(int64(t), "ipv4", false)
	case ngap.PDUSessionTypeIPv6:
		return utils.MakeEnum(int64(t), "ipv6", false)
	case ngap.PDUSessionTypeIPv4v6:
		return utils.MakeEnum(int64(t), "ipv4v6", false)
	case ngap.PDUSessionTypeEthernet:
		return utils.MakeEnum(int64(t), "ethernet", false)
	case ngap.PDUSessionTypeUnstructured:
		return utils.MakeEnum(int64(t), "unstructured", false)
	default:
		return utils.MakeEnum(int64(t), "", true)
	}
}
