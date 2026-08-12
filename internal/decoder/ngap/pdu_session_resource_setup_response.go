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
	DLQosFlowPerTNLInformation DLQosFlowPerTNLInformation `json:"dl_qos_flow_per_tnl_information"`
	QosFlowFailedToSetupList   []QosFlowFailedToSetupItem `json:"qos_flow_failed_to_setup_list,omitempty"`
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

			transfer, err := ngap.ParsePDUSessionResourceSetupResponseTransfer(item.Transfer)
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

			transfer, err := ngap.ParsePDUSessionResourceSetupUnsuccessfulTransfer(item.Transfer)
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

	return out
}

// libGTPTunnel renders the only UPTransportLayerInformation alternative the
// library models; the CHOICE is closed by choice-Extensions (TS 38.413 §9.3.2.1).
func libGTPTunnel(u ngap.UPTransportLayerInformation) GTPTunnel {
	return GTPTunnel{
		GTPTEID:               uint32(u.GTPTunnel.GTPTEID),
		TransportLayerAddress: transportLayerAddressToString(u.GTPTunnel.TransportLayerAddress),
	}
}
