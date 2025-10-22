package ngap

import (
	"fmt"

	"github.com/omec-project/ngap/ngapType"
)

func buildUERadioCapabilityInfoIndication(ueRadioCapabilityInfoIndication ngapType.UERadioCapabilityInfoIndication) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(ueRadioCapabilityInfoIndication.ProtocolIEs.List); i++ {
		ie := ueRadioCapabilityInfoIndication.ProtocolIEs.List[i]
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
		case ngapType.ProtocolIEIDUERadioCapability:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       []byte(ie.Value.UERadioCapability.Value),
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
