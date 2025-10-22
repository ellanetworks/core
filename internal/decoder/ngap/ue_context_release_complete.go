package ngap

import (
	"fmt"

	"github.com/omec-project/ngap/ngapType"
)

type PDUSessionResourceItemCxtRelCpl struct {
	PDUSessionID int64
}

func buildUEContextReleaseComplete(ueContextReleaseComplete ngapType.UEContextReleaseComplete) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(ueContextReleaseComplete.ProtocolIEs.List); i++ {
		ie := ueContextReleaseComplete.ProtocolIEs.List[i]
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
		case ngapType.ProtocolIEIDUserLocationInformation:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUserLocationInformationIE(*ie.Value.UserLocationInformation),
			})
		case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceListCxtRelCpl(*ie.Value.PDUSessionResourceListCxtRelCpl),
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

func buildPDUSessionResourceListCxtRelCpl(pduList ngapType.PDUSessionResourceListCxtRelCpl) []PDUSessionResourceItemCxtRelCpl {
	pduSessionList := make([]PDUSessionResourceItemCxtRelCpl, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]

		pduSessionList = append(pduSessionList, PDUSessionResourceItemCxtRelCpl{
			PDUSessionID: item.PDUSessionID.Value,
		})
	}

	return pduSessionList
}
