package ngap

import (
	"fmt"

	"github.com/omec-project/ngap/ngapType"
)

type PDUSessionResourceFailedToSetupCxtRes struct {
	PDUSessionID                                int64  `json:"pdu_session_id"`
	PDUSessionResourceSetupUnsuccessfulTransfer []byte `json:"pdu_session_resource_setup_unsuccessful_transfer"`
}

type PDUSessionResourceSetupCxtRes struct {
	PDUSessionID                            int64  `json:"pdu_session_id"`
	PDUSessionResourceSetupResponseTransfer []byte `json:"pdu_session_resource_setup_response_transfer"`
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
				Value: UnknownIE{
					Reason: fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
				},
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

		pduSessionList = append(pduSessionList, PDUSessionResourceSetupCxtRes{
			PDUSessionID:                            item.PDUSessionID.Value,
			PDUSessionResourceSetupResponseTransfer: item.PDUSessionResourceSetupResponseTransfer,
		})
	}

	return pduSessionList
}

func buildPDUSessionResourceFailedToSetupListCxtResIE(pduList ngapType.PDUSessionResourceFailedToSetupListCxtRes) []PDUSessionResourceFailedToSetupCxtRes {
	pduSessionList := make([]PDUSessionResourceFailedToSetupCxtRes, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]

		pduSessionList = append(pduSessionList, PDUSessionResourceFailedToSetupCxtRes{
			PDUSessionID: item.PDUSessionID.Value,
			PDUSessionResourceSetupUnsuccessfulTransfer: item.PDUSessionResourceSetupUnsuccessfulTransfer,
		})
	}

	return pduSessionList
}
