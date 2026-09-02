// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
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

	out.SecurityIndication = securityIndication(t.SecurityIndication)

	out.UnrecognizedIEs = unmodeledIEs(t.UnknownIEs())

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
	return utils.NamedEnum(uint8(t), t.Name())
}

// securityIndication renders the user-plane protection the SMF asks the NG-RAN
// node to apply (TS 38.413 §9.3.1.27).
func securityIndication(s *ngap.SecurityIndication) *SecurityIndication {
	if s == nil {
		return nil
	}

	out := &SecurityIndication{
		IntegrityProtectionIndication:       utils.NamedEnum(uint8(s.IntegrityProtectionIndication), s.IntegrityProtectionIndication.Name()),
		ConfidentialityProtectionIndication: utils.NamedEnum(uint8(s.ConfidentialityProtectionIndication), s.ConfidentialityProtectionIndication.Name()),
	}

	if r := s.MaximumIntegrityProtectedDataRateUL; r != nil {
		rate := utils.NamedEnum(uint8(*r), r.Name())
		out.MaximumIntegrityProtectedDataRateUL = &rate
	}

	return out
}
