package ngap

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

type PDUSessionResourceFailedToSetupCxtFail struct {
	PDUSessionID                                int64                                               `json:"pdu_session_id"`
	PDUSessionResourceSetupUnsuccessfulTransfer *PDUSessionResourceSetupUnsuccessfulTransferDecoded `json:"pdu_session_resource_setup_unsuccessful_transfer,omitempty"`

	Error string `json:"error,omitempty"`
}

func buildInitialContextSetupFailure(initialContextSetupFailure ngapType.InitialContextSetupFailure) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(initialContextSetupFailure.ProtocolIEs.List); i++ {
		ie := initialContextSetupFailure.ProtocolIEs.List[i]

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
		case ngapType.ProtocolIEIDCause:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       causeToEnum(*ie.Value.Cause),
			})
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtFail:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceFailedToSetupListCxtFailIE(*ie.Value.PDUSessionResourceFailedToSetupListCxtFail),
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

func buildPDUSessionResourceFailedToSetupListCxtFailIE(pduList ngapType.PDUSessionResourceFailedToSetupListCxtFail) []PDUSessionResourceFailedToSetupCxtFail {
	pduSessionList := make([]PDUSessionResourceFailedToSetupCxtFail, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]
		entry := PDUSessionResourceFailedToSetupCxtFail{
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
