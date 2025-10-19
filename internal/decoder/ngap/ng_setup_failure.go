package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

type NGSetupFailure struct {
	IEs []IE `json:"ies"`
}

func buildNGSetupFailure(ngSetupFailure *ngapType.NGSetupFailure) *NGSetupFailure {
	if ngSetupFailure == nil {
		return nil
	}

	ngFail := &NGSetupFailure{}

	for i := 0; i < len(ngSetupFailure.ProtocolIEs.List); i++ {
		ie := ngSetupFailure.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			ngFail.IEs = append(ngFail.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				Cause:       strPtr(causeToString(ie.Value.Cause)),
			})
		case ngapType.ProtocolIEIDTimeToWait:
			ngFail.IEs = append(ngFail.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				TimeToWait:  buildTimeToWaitIE(ie.Value.TimeToWait),
			})
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			ngFail.IEs = append(ngFail.IEs, IE{
				ID:                     protocolIEIDToString(ie.Id.Value),
				Criticality:            criticalityToString(ie.Criticality.Value),
				CriticalityDiagnostics: buildCriticalityDiagnosticsIE(ie.Value.CriticalityDiagnostics),
			})
		default:
			ngFail.IEs = append(ngFail.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	return ngFail
}

func buildTimeToWaitIE(timeToWait *ngapType.TimeToWait) *string {
	if timeToWait == nil {
		return nil
	}

	var str string

	switch timeToWait.Value {
	case ngapType.TimeToWaitPresentV1s:
		str = "V1s (0)"
	case ngapType.TimeToWaitPresentV2s:
		str = "V2s (1)"
	case ngapType.TimeToWaitPresentV5s:
		str = "V5s (2)"
	case ngapType.TimeToWaitPresentV10s:
		str = "V10s (3)"
	case ngapType.TimeToWaitPresentV20s:
		str = "V20s (4)"
	case ngapType.TimeToWaitPresentV60s:
		str = "V60s (5)"
	default:
		str = fmt.Sprintf("Unknown (%d)", timeToWait.Value)
	}

	return &str
}
