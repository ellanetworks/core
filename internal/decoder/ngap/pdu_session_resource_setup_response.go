package ngap

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type DLQosFlowPerTNLInformation struct {
	GTPTunnel          GTPTunnel           `json:"gtp_tunnel"`
	AssociatedQosFlows []AssociatedQosFlow `json:"associated_qos_flows"`
}

type AssociatedQosFlow struct {
	QosFlowIdentifier int64 `json:"qos_flow_identifier"`
}

type QosFlowFailedToSetupItem struct {
	QosFlowIdentifier int64                   `json:"qos_flow_identifier"`
	Cause             utils.EnumField[uint64] `json:"cause"`
}

type PDUSessionResourceSetupResponseTransferDecoded struct {
	DLQosFlowPerTNLInformation DLQosFlowPerTNLInformation `json:"dl_qos_flow_per_tnl_information"`
	QosFlowFailedToSetupList   []QosFlowFailedToSetupItem `json:"qos_flow_failed_to_setup_list,omitempty"`
}

type PDUSessionResourceSetupUnsuccessfulTransferDecoded struct {
	Cause utils.EnumField[uint64] `json:"cause"`
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

func buildPDUSessionResourceSetupResponse(pduSessionResourceSetupResponse ngapType.PDUSessionResourceSetupResponse) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(pduSessionResourceSetupResponse.ProtocolIEs.List); i++ {
		ie := pduSessionResourceSetupResponse.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceSetupListSUResIE(*ie.Value.PDUSessionResourceSetupListSURes),
			})
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListSURes:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceFailedToSetupListSUResIE(*ie.Value.PDUSessionResourceFailedToSetupListSURes),
			})
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildCriticalityDiagnosticsIE(ie.Value.CriticalityDiagnostics),
			})
		default:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Error:       fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
			})
		}
	}

	return NGAPMessageValue{
		IEs: ies,
	}
}

func buildPDUSessionResourceSetupListSUResIE(pduList ngapType.PDUSessionResourceSetupListSURes) []PDUSessionResourceSetupSURes {
	pduSessionList := make([]PDUSessionResourceSetupSURes, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]
		entry := PDUSessionResourceSetupSURes{
			PDUSessionID: item.PDUSessionID.Value,
		}

		transfer, err := decodeSetupResponseTransfer(item.PDUSessionResourceSetupResponseTransfer)
		if err != nil {
			entry.Error = fmt.Sprintf("failed to decode response transfer: %v", err)
		} else {
			entry.PDUSessionResourceSetupResponseTransfer = transfer
		}

		pduSessionList = append(pduSessionList, entry)
	}

	return pduSessionList
}

func buildPDUSessionResourceFailedToSetupListSUResIE(pduList ngapType.PDUSessionResourceFailedToSetupListSURes) []PDUSessionResourceFailedToSetupSURes {
	pduSessionList := make([]PDUSessionResourceFailedToSetupSURes, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]
		entry := PDUSessionResourceFailedToSetupSURes{
			PDUSessionID: item.PDUSessionID.Value,
		}

		transfer, err := decodeSetupUnsuccessfulTransfer(item.PDUSessionResourceSetupUnsuccessfulTransfer)
		if err != nil {
			entry.Error = fmt.Sprintf("failed to decode unsuccessful transfer: %v", err)
		} else {
			entry.PDUSessionResourceSetupUnsuccessfulTransfer = transfer
		}

		pduSessionList = append(pduSessionList, entry)
	}

	return pduSessionList
}

func decodeSetupResponseTransfer(transfer aper.OctetString) (*PDUSessionResourceSetupResponseTransferDecoded, error) {
	if transfer == nil {
		return nil, fmt.Errorf("transfer is nil")
	}

	pdu := &ngapType.PDUSessionResourceSetupResponseTransfer{}

	err := aper.UnmarshalWithParams(transfer, pdu, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response transfer: %v", err)
	}

	result := &PDUSessionResourceSetupResponseTransferDecoded{}

	dlInfo := &pdu.DLQosFlowPerTNLInformation
	if dlInfo.UPTransportLayerInformation.GTPTunnel != nil {
		tunnel := dlInfo.UPTransportLayerInformation.GTPTunnel
		teid := binary.BigEndian.Uint32(tunnel.GTPTEID.Value)
		addr := tunnel.TransportLayerAddress.Value.Bytes
		ip := transportLayerAddressToString(addr)

		result.DLQosFlowPerTNLInformation.GTPTunnel = GTPTunnel{
			GTPTEID:               teid,
			TransportLayerAddress: ip,
		}
	}

	for _, flow := range dlInfo.AssociatedQosFlowList.List {
		result.DLQosFlowPerTNLInformation.AssociatedQosFlows = append(
			result.DLQosFlowPerTNLInformation.AssociatedQosFlows,
			AssociatedQosFlow{QosFlowIdentifier: flow.QosFlowIdentifier.Value},
		)
	}

	if pdu.QosFlowFailedToSetupList != nil {
		for _, flow := range pdu.QosFlowFailedToSetupList.List {
			result.QosFlowFailedToSetupList = append(result.QosFlowFailedToSetupList, QosFlowFailedToSetupItem{
				QosFlowIdentifier: flow.QosFlowIdentifier.Value,
				Cause:             causeToEnum(flow.Cause),
			})
		}
	}

	return result, nil
}

func decodeSetupUnsuccessfulTransfer(transfer aper.OctetString) (*PDUSessionResourceSetupUnsuccessfulTransferDecoded, error) {
	if transfer == nil {
		return nil, fmt.Errorf("transfer is nil")
	}

	pdu := &ngapType.PDUSessionResourceSetupUnsuccessfulTransfer{}

	err := aper.UnmarshalWithParams(transfer, pdu, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal unsuccessful transfer: %v", err)
	}

	return &PDUSessionResourceSetupUnsuccessfulTransferDecoded{
		Cause: causeToEnum(pdu.Cause),
	}, nil
}
