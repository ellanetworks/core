package ngap

import (
	"fmt"

	"github.com/omec-project/ngap/ngapType"
)

type InitialContextSetupResponse struct {
	IEs []IE `json:"ies"`
}

func buildInitialContextSetupResponse(initialContextSetupResponse *ngapType.InitialContextSetupResponse) *InitialContextSetupResponse {
	if initialContextSetupResponse == nil {
		return nil
	}

	icsResponse := &InitialContextSetupResponse{}

	for i := 0; i < len(initialContextSetupResponse.ProtocolIEs.List); i++ {
		ie := initialContextSetupResponse.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			icsResponse.IEs = append(icsResponse.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				AMFUENGAPID: &ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			icsResponse.IEs = append(icsResponse.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes:
			icsResponse.IEs = append(icsResponse.IEs, IE{
				ID:                                protocolIEIDToEnum(ie.Id.Value),
				Criticality:                       criticalityToEnum(ie.Criticality.Value),
				PDUSessionResourceSetupListCxtRes: buildPDUSessionResourceSetupListCxtResIE(ie.Value.PDUSessionResourceSetupListCxtRes),
			})
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtRes:
			icsResponse.IEs = append(icsResponse.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				PDUSessionResourceFailedToSetupListCxtRes: buildPDUSessionResourceFailedToSetupListCxtResIE(ie.Value.PDUSessionResourceFailedToSetupListCxtRes),
			})
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			icsResponse.IEs = append(icsResponse.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildCriticalityDiagnosticsIE(ie.Value.CriticalityDiagnostics),
			})
		default:
			icsResponse.IEs = append(icsResponse.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value: UnknownIE{
					Reason: fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
				},
			})
		}
	}

	return icsResponse
}

func buildPDUSessionResourceSetupListCxtResIE(pduList *ngapType.PDUSessionResourceSetupListCxtRes) []PDUSessionResourceSetupCxtRes {
	if pduList == nil {
		return nil
	}

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

func buildPDUSessionResourceFailedToSetupListCxtResIE(pduList *ngapType.PDUSessionResourceFailedToSetupListCxtRes) []PDUSessionResourceFailedToSetupCxtRes {
	if pduList == nil {
		return nil
	}

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
