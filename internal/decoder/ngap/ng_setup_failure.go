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
				Value:       causeToEnum(ie.Value.Cause),
			})
		case ngapType.ProtocolIEIDTimeToWait:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildTimeToWaitIE(ie.Value.TimeToWait),
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

func buildTimeToWaitIE(timeToWait *ngapType.TimeToWait) *EnumField {
	if timeToWait == nil {
		return nil
	}

	switch timeToWait.Value {
	case ngapType.TimeToWaitPresentV1s:
		return &EnumField{Label: "V1s", Value: int(ngapType.TimeToWaitPresentV1s)}
	case ngapType.TimeToWaitPresentV2s:
		return &EnumField{Label: "V2s", Value: int(ngapType.TimeToWaitPresentV2s)}
	case ngapType.TimeToWaitPresentV5s:
		return &EnumField{Label: "V5s", Value: int(ngapType.TimeToWaitPresentV5s)}
	case ngapType.TimeToWaitPresentV10s:
		return &EnumField{Label: "V10s", Value: int(ngapType.TimeToWaitPresentV10s)}
	case ngapType.TimeToWaitPresentV20s:
		return &EnumField{Label: "V20s", Value: int(ngapType.TimeToWaitPresentV20s)}
	case ngapType.TimeToWaitPresentV60s:
		return &EnumField{Label: "V60s", Value: int(ngapType.TimeToWaitPresentV60s)}
	default:
		return &EnumField{Label: fmt.Sprintf("Unknown (%d)", timeToWait.Value), Value: int(timeToWait.Value)}
	}
}
