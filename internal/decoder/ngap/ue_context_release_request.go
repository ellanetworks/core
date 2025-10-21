package ngap

import (
	"fmt"

	"github.com/omec-project/ngap/ngapType"
)

func buildUEContextReleaseRequest(ueContextReleaseRequest ngapType.UEContextReleaseRequest) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(ueContextReleaseRequest.ProtocolIEs.List); i++ {
		ie := ueContextReleaseRequest.ProtocolIEs.List[i]
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

		case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceListCxtRelReq(*ie.Value.PDUSessionResourceListCxtRelReq),
			})
		case ngapType.ProtocolIEIDCause:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       causeToEnum(*ie.Value.Cause),
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

type PDUSessionResourceListCxtRelReq struct {
	PDUSessionID int64 `json:"pdu_session_id"`
}

func buildPDUSessionResourceListCxtRelReq(pduList ngapType.PDUSessionResourceListCxtRelReq) []PDUSessionResourceListCxtRelReq {
	pduSessionList := make([]PDUSessionResourceListCxtRelReq, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]

		pduSessionList = append(pduSessionList, PDUSessionResourceListCxtRelReq{
			PDUSessionID: item.PDUSessionID.Value,
		})
	}

	return pduSessionList
}
