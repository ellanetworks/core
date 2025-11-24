package ngap

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

func buildAMFStatusIndication(amfStatusIndication ngapType.AMFStatusIndication) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(amfStatusIndication.ProtocolIEs.List); i++ {
		ie := amfStatusIndication.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDUnavailableGUAMIList:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUnavailableGuamiList(*ie.Value.UnavailableGUAMIList),
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

func buildUnavailableGuamiList(list ngapType.UnavailableGUAMIList) []Guami {
	guamis := make([]Guami, 0)

	for _, item := range list.List {
		guamis = append(guamis, buildGUAMI(item.GUAMI))
	}

	return guamis
}
