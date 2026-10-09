// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	libngap "github.com/ellanetworks/core/ngap"
)

type GBRQoSFlow struct {
	QFI    uint8
	FiveQI int32
	ARP    models.Arp
	MFBR   models.Ambr
	GFBR   models.Ambr
}

func gbrQosFlowLevelQosParameters(f GBRQoSFlow) (libngap.QosFlowLevelQosParameters, error) {
	params, err := qosFlowLevelQosParameters(&models.QosData{QFI: f.QFI, Var5qi: f.FiveQI, Arp: &f.ARP})
	if err != nil {
		return libngap.QosFlowLevelQosParameters{}, err
	}

	params.GBRQosInformation = &libngap.GBRQosInformation{
		MaximumFlowBitRateDL:    libngap.BitRate(f.MFBR.Downlink.Bps()),
		MaximumFlowBitRateUL:    libngap.BitRate(f.MFBR.Uplink.Bps()),
		GuaranteedFlowBitRateDL: libngap.BitRate(f.GFBR.Downlink.Bps()),
		GuaranteedFlowBitRateUL: libngap.BitRate(f.GFBR.Uplink.Bps()),
	}

	return params, nil
}

func BuildQoSFlowsModifyRequestTransfer(addOrModify []GBRQoSFlow, release []uint8) ([]byte, error) {
	var transfer libngap.PDUSessionResourceModifyRequestTransfer

	for _, f := range addOrModify {
		params, err := gbrQosFlowLevelQosParameters(f)
		if err != nil {
			return nil, fmt.Errorf("QoS flow %d: %w", f.QFI, err)
		}

		transfer.QosFlowAddOrModifyRequest = append(transfer.QosFlowAddOrModifyRequest, libngap.QosFlowAddOrModifyRequestItem{
			QosFlowIdentifier:         libngap.QosFlowIdentifier(f.QFI),
			QosFlowLevelQosParameters: &params,
		})
	}

	for _, qfi := range release {
		transfer.QosFlowToRelease = append(transfer.QosFlowToRelease, libngap.QosFlowWithCauseItem{
			QosFlowIdentifier: libngap.QosFlowIdentifier(qfi),
			Cause:             libngap.Cause{Group: libngap.CauseGroupNAS, Value: libngap.CauseNASNormalRelease},
		})
	}

	if len(transfer.QosFlowAddOrModifyRequest) == 0 && len(transfer.QosFlowToRelease) == 0 {
		return nil, fmt.Errorf("no QoS flow to add, modify or release")
	}

	buf, err := transfer.Marshal()
	if err != nil {
		return nil, fmt.Errorf("encode PDU Session Resource Modify Request Transfer: %w", err)
	}

	return buf, nil
}

type QoSFlowsOutcome struct {
	Accepted    []uint8
	Failed      []uint8
	EPSFallback bool
	Handover    bool
}

func ParseQoSFlowsModifyResponse(n2 []byte) (QoSFlowsOutcome, error) {
	t, err := libngap.ParsePDUSessionResourceModifyResponseTransfer(n2)
	if err != nil {
		return QoSFlowsOutcome{}, fmt.Errorf("decode PDU Session Resource Modify Response Transfer: %w", err)
	}

	var out QoSFlowsOutcome

	for _, item := range t.QosFlowAddOrModifyResponse {
		out.Accepted = append(out.Accepted, uint8(item.QosFlowIdentifier))
	}

	for _, item := range t.QosFlowFailedToAddOrModify {
		out.Failed = append(out.Failed, uint8(item.QosFlowIdentifier))
		out.EPSFallback = out.EPSFallback || epsFallbackTriggered(item.Cause)
	}

	return out, nil
}

func ParseModifyUnsuccessful(n2 []byte) (QoSFlowsOutcome, error) {
	t, err := libngap.ParsePDUSessionResourceModifyUnsuccessfulTransfer(n2)
	if err != nil {
		return QoSFlowsOutcome{}, fmt.Errorf("decode PDU Session Resource Modify Unsuccessful Transfer: %w", err)
	}

	return QoSFlowsOutcome{EPSFallback: epsFallbackTriggered(t.Cause), Handover: handoverTriggered(t.Cause)}, nil
}

func handoverTriggered(c libngap.Cause) bool {
	if c.Group != libngap.CauseGroupRadioNetwork || c.Extended {
		return false
	}

	switch c.Value {
	case libngap.CauseRadioNetworkNGIntraSystemHandoverTriggered, libngap.CauseRadioNetworkNGInterSystemHandoverTriggered, libngap.CauseRadioNetworkXnHandoverTriggered:
		return true
	default:
		return false
	}
}

func epsFallbackTriggered(c libngap.Cause) bool {
	return c.Group == libngap.CauseGroupRadioNetwork && c.Value == libngap.CauseRadioNetworkIMSVoiceEPSFallbackTriggered && !c.Extended
}

type AdmittedQoSFlows struct {
	Accepted []uint8
	Failed   []uint8
}

func SetupResponseQoSFlows(b []byte) (AdmittedQoSFlows, error) {
	t, err := libngap.ParsePDUSessionResourceSetupResponseTransfer(b)
	if err != nil {
		return AdmittedQoSFlows{}, fmt.Errorf("decode PDU Session Resource Setup Response Transfer: %w", err)
	}

	var out AdmittedQoSFlows

	for _, item := range t.DLQosFlowPerTNLInformation.AssociatedQosFlowList {
		out.Accepted = append(out.Accepted, uint8(item.QosFlowIdentifier))
	}

	for _, tnl := range t.AdditionalDLQosFlowPerTNLInformation {
		for _, item := range tnl.QosFlowPerTNLInformation.AssociatedQosFlowList {
			out.Accepted = append(out.Accepted, uint8(item.QosFlowIdentifier))
		}
	}

	for _, item := range t.QosFlowFailedToSetup {
		out.Failed = append(out.Failed, uint8(item.QosFlowIdentifier))
	}

	return out, nil
}

func PathSwitchQoSFlows(b []byte) (AdmittedQoSFlows, error) {
	t, err := libngap.ParsePathSwitchRequestTransfer(b)
	if err != nil {
		return AdmittedQoSFlows{}, fmt.Errorf("decode Path Switch Request Transfer: %w", err)
	}

	var out AdmittedQoSFlows

	for _, item := range t.QosFlowAccepted {
		out.Accepted = append(out.Accepted, uint8(item.QosFlowIdentifier))
	}

	return out, nil
}

func HandoverAckQoSFlows(b []byte) (AdmittedQoSFlows, error) {
	t, err := libngap.ParseHandoverRequestAcknowledgeTransfer(b)
	if err != nil {
		return AdmittedQoSFlows{}, fmt.Errorf("decode Handover Request Acknowledge Transfer: %w", err)
	}

	var out AdmittedQoSFlows

	for _, item := range t.QosFlowSetupResponse {
		out.Accepted = append(out.Accepted, uint8(item.QosFlowIdentifier))
	}

	for _, item := range t.QosFlowFailedToSetup {
		out.Failed = append(out.Failed, uint8(item.QosFlowIdentifier))
	}

	return out, nil
}

func NotifyReleasedQoSFlows(b []byte) ([]uint8, error) {
	t, err := libngap.ParsePDUSessionResourceNotifyTransfer(b)
	if err != nil {
		return nil, fmt.Errorf("decode PDU Session Resource Notify Transfer: %w", err)
	}

	released := make([]uint8, 0, len(t.QosFlowReleased))
	for _, item := range t.QosFlowReleased {
		released = append(released, uint8(item.QosFlowIdentifier))
	}

	return released, nil
}
