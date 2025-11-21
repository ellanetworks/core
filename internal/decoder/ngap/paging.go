package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

func buildPaging(paging ngapType.Paging) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(paging.ProtocolIEs.List); i++ {
		ie := paging.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDUEPagingIdentity:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUEPagingIdentity(*ie.Value.UEPagingIdentity),
			})
		case ngapType.ProtocolIEIDTAIListForPaging:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildTAIListForPaging(*ie.Value.TAIListForPaging),
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

type UEPagingIdentity struct {
	FiveGSTMSI FiveGSTMSI `json:"five_gs_tmsi"`
}

func buildUEPagingIdentity(uePagingIdentity ngapType.UEPagingIdentity) UEPagingIdentity {
	if uePagingIdentity.FiveGSTMSI == nil {
		return UEPagingIdentity{}
	}

	return UEPagingIdentity{
		FiveGSTMSI: buildFiveGSTMSIIE(*uePagingIdentity.FiveGSTMSI),
	}
}

func buildTAIListForPaging(taiList ngapType.TAIListForPaging) []TAI {
	tais := make([]TAI, 0)

	for _, item := range taiList.List {
		tais = append(tais, buildTAI(item))
	}

	return tais
}

func buildTAI(taiItem ngapType.TAIListForPagingItem) TAI {
	return TAI{
		PLMNID: plmnIDToModels(taiItem.TAI.PLMNIdentity),
		TAC:    hex.EncodeToString(taiItem.TAI.TAC.Value),
	}
}
