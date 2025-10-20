package ngap

import (
	"fmt"

	"github.com/omec-project/ngap/ngapType"
)

type PDUSessionResourceSetupSURes struct {
	PDUSessionID                            int64  `json:"pdu_session_id"`
	PDUSessionResourceSetupResponseTransfer []byte `json:"pdu_session_resource_setup_response_transfer"`
}

type PDUSessionResourceFailedToSetupSURes struct {
	PDUSessionID                                int64  `json:"pdu_session_id"`
	PDUSessionResourceSetupUnsuccessfulTransfer []byte `json:"pdu_session_resource_setup_unsuccessful_transfer"`
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

func buildPDUSessionResourceSetupListSUResIE(pduList ngapType.PDUSessionResourceSetupListSURes) []PDUSessionResourceSetupSURes {
	pduSessionList := make([]PDUSessionResourceSetupSURes, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]

		pduSessionList = append(pduSessionList, PDUSessionResourceSetupSURes{
			PDUSessionID:                            item.PDUSessionID.Value,
			PDUSessionResourceSetupResponseTransfer: item.PDUSessionResourceSetupResponseTransfer,
		})
	}

	return pduSessionList
}

func buildPDUSessionResourceFailedToSetupListSUResIE(pduList ngapType.PDUSessionResourceFailedToSetupListSURes) []PDUSessionResourceFailedToSetupSURes {
	pduSessionList := make([]PDUSessionResourceFailedToSetupSURes, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]

		pduSessionList = append(pduSessionList, PDUSessionResourceFailedToSetupSURes{
			PDUSessionID: item.PDUSessionID.Value,
			PDUSessionResourceSetupUnsuccessfulTransfer: item.PDUSessionResourceSetupUnsuccessfulTransfer,
		})
	}

	return pduSessionList
}
