package ngap

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

type PDUSessionResourceFailedToSetupCxtRes struct {
	PDUSessionID                                int64                                               `json:"pdu_session_id"`
	PDUSessionResourceSetupUnsuccessfulTransfer *PDUSessionResourceSetupUnsuccessfulTransferDecoded `json:"pdu_session_resource_setup_unsuccessful_transfer,omitempty"`

	Error string `json:"error,omitempty"`
}

type PDUSessionResourceSetupCxtRes struct {
	PDUSessionID                            int64                                           `json:"pdu_session_id"`
	PDUSessionResourceSetupResponseTransfer *PDUSessionResourceSetupResponseTransferDecoded `json:"pdu_session_resource_setup_response_transfer,omitempty"`

	Error string `json:"error,omitempty"`
}

func buildInitialContextSetupResponse(initialContextSetupResponse ngapType.InitialContextSetupResponse) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(initialContextSetupResponse.ProtocolIEs.List); i++ {
		ie := initialContextSetupResponse.ProtocolIEs.List[i]
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
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceSetupListCxtResIE(*ie.Value.PDUSessionResourceSetupListCxtRes),
			})
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtRes:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceFailedToSetupListCxtResIE(*ie.Value.PDUSessionResourceFailedToSetupListCxtRes),
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

func buildPDUSessionResourceSetupListCxtResIE(pduList ngapType.PDUSessionResourceSetupListCxtRes) []PDUSessionResourceSetupCxtRes {
	pduSessionList := make([]PDUSessionResourceSetupCxtRes, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]
		entry := PDUSessionResourceSetupCxtRes{
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

func buildPDUSessionResourceFailedToSetupListCxtResIE(pduList ngapType.PDUSessionResourceFailedToSetupListCxtRes) []PDUSessionResourceFailedToSetupCxtRes {
	pduSessionList := make([]PDUSessionResourceFailedToSetupCxtRes, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]
		entry := PDUSessionResourceFailedToSetupCxtRes{
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
