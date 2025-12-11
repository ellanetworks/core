package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func handleStatus5GMM(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Status 5GMM", zap.String("supi", ue.Supi))

	_, span := tracer.Start(ctx, "AMF NAS HandleStatus5GMM")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Registered, context.Authentication, context.SecurityMode, context.ContextSetup:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}

		cause := msg.Status5GMM.Cause5GMM.GetCauseValue()
		ue.GmmLog.Error("Received Status 5GMM with cause", zap.String("Cause", nasMessage.Cause5GMMToString(cause)))
		return nil
	default:
		return fmt.Errorf("state mismatch: receive Status 5GMM message in state %s", ue.State.Current())
	}
}
