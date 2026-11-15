// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type DLQosFlowPerTNLInformation struct {
	GTPTunnel          GTPTunnel           `json:"gtp_tunnel"`
	AssociatedQosFlows []AssociatedQosFlow `json:"associated_qos_flows"`
}

type AssociatedQosFlow struct {
	QosFlowIdentifier int64 `json:"qos_flow_identifier"`
}

type QosFlowFailedToSetupItem struct {
	QosFlowIdentifier int64 `json:"qos_flow_identifier"`
	Cause             Cause `json:"cause"`
}

type PDUSessionResourceSetupResponseTransferDecoded struct {
	DLQosFlowPerTNLInformation                    DLQosFlowPerTNLInformation `json:"dl_qos_flow_per_tnl_information"`
	AdditionalDLQosFlowPerTNLInformation          []QosFlowPerTNLInformation `json:"additional_dl_qos_flow_per_tnl_information,omitempty"`
	SecurityResult                                *SecurityResult            `json:"security_result,omitempty"`
	QosFlowFailedToSetupList                      []QosFlowFailedToSetupItem `json:"qos_flow_failed_to_setup_list,omitempty"`
	RedundantDLQosFlowPerTNLInformation           *QosFlowPerTNLInformation  `json:"redundant_dl_qos_flow_per_tnl_information,omitempty"`
	AdditionalRedundantDLQosFlowPerTNLInformation []QosFlowPerTNLInformation `json:"additional_redundant_dl_qos_flow_per_tnl_information,omitempty"`
}

type PDUSessionResourceSetupUnsuccessfulTransferDecoded struct {
	Cause Cause `json:"cause"`
}

type PDUSessionResourceSetupSURes struct {
	PDUSessionID                            int64                                           `json:"pdu_session_id"`
	PDUSessionResourceSetupResponseTransfer *PDUSessionResourceSetupResponseTransferDecoded `json:"pdu_session_resource_setup_response_transfer,omitempty"`

	Error string `json:"error,omitempty"`
}

type PDUSessionResourceFailedToSetupSURes struct {
	PDUSessionID                                int64                                               `json:"pdu_session_id"`
	PDUSessionResourceSetupUnsuccessfulTransfer *PDUSessionResourceSetupUnsuccessfulTransferDecoded `json:"pdu_session_resource_setup_unsuccessful_transfer,omitempty"`

	Error string `json:"error,omitempty"`
}

// PDU Session Resource Setup Response reports, per session, the tunnel the
// NG-RAN node bound and any QoS flows it could not set up (TS 38.413 §9.2.1.2).
func buildPDUSessionResourceSetupResponse(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePDUSessionResourceSetupResponse(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PDU Session Resource Setup Response: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.PDUSessionResourceSetup != nil {
		out := make([]PDUSessionResourceSetupSURes, 0, len(m.PDUSessionResourceSetup))

		for _, item := range m.PDUSessionResourceSetup {
			entry := PDUSessionResourceSetupSURes{PDUSessionID: int64(item.PDUSessionID)}

			transfer, err := ngap.DecodePDUSessionResourceSetupResponseTransfer(item.Transfer)
			if err != nil {
				entry.Error = fmt.Sprintf("failed to decode response transfer: %v", err)
			} else {
				entry.PDUSessionResourceSetupResponseTransfer = libSetupResponseTransfer(transfer)
			}

			out = append(out, entry)
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceSetupListSURes, ngap.CriticalityIgnore, out))
	}

	if m.PDUSessionResourceFailed != nil {
		out := make([]PDUSessionResourceFailedToSetupSURes, 0, len(m.PDUSessionResourceFailed))

		for _, item := range m.PDUSessionResourceFailed {
			entry := PDUSessionResourceFailedToSetupSURes{PDUSessionID: int64(item.PDUSessionID)}

			transfer, err := ngap.DecodePDUSessionResourceSetupUnsuccessfulTransfer(item.Transfer)
			if err != nil {
				entry.Error = fmt.Sprintf("failed to decode unsuccessful transfer: %v", err)
			} else {
				entry.PDUSessionResourceSetupUnsuccessfulTransfer = &PDUSessionResourceSetupUnsuccessfulTransferDecoded{
					Cause: cause(transfer.Cause),
				}
			}

			out = append(out, entry)
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceFailedToSetupListSURes, ngap.CriticalityIgnore, out))
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(ngap.IDUserLocationInformation, ngap.CriticalityIgnore,
			userLocationInformation(*m.UserLocationInformation)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore,
			criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func libSetupResponseTransfer(t *ngap.PDUSessionResourceSetupResponseTransfer) *PDUSessionResourceSetupResponseTransferDecoded {
	out := &PDUSessionResourceSetupResponseTransferDecoded{
		DLQosFlowPerTNLInformation: DLQosFlowPerTNLInformation{
			GTPTunnel: libGTPTunnel(t.DLQosFlowPerTNLInformation.UPTransportLayerInformation),
		},
	}

	for _, flow := range t.DLQosFlowPerTNLInformation.AssociatedQosFlowList {
		out.DLQosFlowPerTNLInformation.AssociatedQosFlows = append(
			out.DLQosFlowPerTNLInformation.AssociatedQosFlows,
			AssociatedQosFlow{QosFlowIdentifier: int64(flow.QosFlowIdentifier)},
		)
	}

	for _, flow := range t.QosFlowFailedToSetup {
		out.QosFlowFailedToSetupList = append(out.QosFlowFailedToSetupList, QosFlowFailedToSetupItem{
			QosFlowIdentifier: int64(flow.QosFlowIdentifier),
			Cause:             cause(flow.Cause),
		})
	}

	out.AdditionalDLQosFlowPerTNLInformation = qosFlowPerTNLInformationList(t.AdditionalDLQosFlowPerTNLInformation)

	if t.SecurityResult != nil {
		result := securityResult(*t.SecurityResult)
		out.SecurityResult = &result
	}

	if t.RedundantDLQosFlowPerTNLInformation != nil {
		q := qosFlowPerTNLInformation(*t.RedundantDLQosFlowPerTNLInformation)
		out.RedundantDLQosFlowPerTNLInformation = &q
	}

	out.AdditionalRedundantDLQosFlowPerTNLInformation = qosFlowPerTNLInformationList(t.AdditionalRedundantDLQosFlowPerTNLInformation)

	return out
}

// libGTPTunnel renders the GTP tunnel alternative of UPTransportLayerInformation;
// any other alternative is rendered as the CHOICE itself (TS 38.413 §9.3.2.1).
func libGTPTunnel(u ngap.UPTransportLayerInformation) GTPTunnel {
	if u.GTPTunnel == nil {
		return GTPTunnel{alternative: render(u)}
	}

	return GTPTunnel{
		GTPTEID:               uint32(u.GTPTunnel.GTPTEID),
		TransportLayerAddress: transportLayerAddressToString(u.GTPTunnel.TransportLayerAddress),
	}
}

func optionalGTPTunnel(u *ngap.UPTransportLayerInformation) *GTPTunnel {
	if u == nil {
		return nil
	}

	tunnel := libGTPTunnel(*u)

	return &tunnel
}

func gtpTunnelList(list ngap.UPTransportLayerInformationList) []GTPTunnel {
	var out []GTPTunnel

	for _, item := range list {
		out = append(out, libGTPTunnel(item.NGUUPTNLInformation))
	}

	return out
}

func gtpTunnelPairs(list ngap.UPTransportLayerInformationPairList) []ULNGUUPTNLModify {
	var out []ULNGUUPTNLModify

	for _, p := range list {
		out = append(out, ULNGUUPTNLModify{
			ULNGUUPTNLInformation: libGTPTunnel(p.ULNGUUPTNLInformation),
			DLNGUUPTNLInformation: libGTPTunnel(p.DLNGUUPTNLInformation),
		})
	}

	return out
}
