package ngap

import (
	"fmt"

	"github.com/omec-project/ngap/ngapType"
)

func buildNGSetupFailure(ngSetupFailure ngapType.NGSetupFailure) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(ngSetupFailure.ProtocolIEs.List); i++ {
		ie := ngSetupFailure.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       causeToEnum(*ie.Value.Cause),
			})
		case ngapType.ProtocolIEIDTimeToWait:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildTimeToWaitIE(*ie.Value.TimeToWait),
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

func buildTimeToWaitIE(timeToWait ngapType.TimeToWait) EnumField {
	switch timeToWait.Value {
	case ngapType.TimeToWaitPresentV1s:
		return makeEnum(int(ngapType.TimeToWaitPresentV1s), "V1s", false)
	case ngapType.TimeToWaitPresentV2s:
		return makeEnum(int(ngapType.TimeToWaitPresentV2s), "V2s", false)
	case ngapType.TimeToWaitPresentV5s:
		return makeEnum(int(ngapType.TimeToWaitPresentV5s), "V5s", false)
	case ngapType.TimeToWaitPresentV10s:
		return makeEnum(int(ngapType.TimeToWaitPresentV10s), "V10s", false)
	case ngapType.TimeToWaitPresentV20s:
		return makeEnum(int(ngapType.TimeToWaitPresentV20s), "V20s", false)
	case ngapType.TimeToWaitPresentV60s:
		return makeEnum(int(ngapType.TimeToWaitPresentV60s), "V60s", false)
	default:
		return makeEnum(int(timeToWait.Value), "", true)
	}
}
