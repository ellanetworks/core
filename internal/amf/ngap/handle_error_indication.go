package ngap

import (
	gocontext "context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
)

func HandleErrorIndication(ctx gocontext.Context, ran *amf.Radio, msg decode.ErrorIndication) {
	if msg.Cause == nil && msg.CriticalityDiagnostics == nil {
		logger.WithTrace(ctx, ran.Log).Error("[ErrorIndication] both Cause IE and CriticalityDiagnostics IE are nil, should have at least one")
		return
	}

	if msg.Cause != nil {
		logger.WithTrace(ctx, ran.Log).Debug("Error Indication Cause", logger.Cause(causeToString(*msg.Cause)))
	}
}
