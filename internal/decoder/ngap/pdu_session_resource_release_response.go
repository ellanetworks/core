package ngap

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

func buildPDUSessionResourceReleaseResponse(resp ngapType.PDUSessionResourceReleaseResponse) NGAPMessageValue {
	ies := make([]IE, 0)

	// AMFUENGAPID := int64(0)

	for i := 0; i < len(resp.ProtocolIEs.List); i++ {
		ie := resp.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			// AMFUENGAPID = ie.Value.AMFUENGAPID.Value
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
		case ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceReleasedListRelRes(*ie.Value.PDUSessionResourceReleasedListRelRes),
			})
		case ngapType.ProtocolIEIDUserLocationInformation:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUserLocationInformationIE(*ie.Value.UserLocationInformation),
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

type PDUSessionResourceReleasedItemRelRes struct {
	PDUSessionID                              int64  `json:"pdu_session_id"`
	PDUSessionResourceReleaseResponseTransfer []byte `json:"pdu_session_resource_release_response_transfer"`
}

func buildPDUSessionResourceReleasedListRelRes(pduList ngapType.PDUSessionResourceReleasedListRelRes) []PDUSessionResourceReleasedItemRelRes {
	pduSessionList := make([]PDUSessionResourceReleasedItemRelRes, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]

		pduSessionList = append(pduSessionList, PDUSessionResourceReleasedItemRelRes{
			PDUSessionID: item.PDUSessionID.Value,
			PDUSessionResourceReleaseResponseTransfer: item.PDUSessionResourceReleaseResponseTransfer,
		})
	}

	return pduSessionList
}
