package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func handleStatus5GMM(ctx context.Context, ue *amfContext.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Status 5GMM", zap.String("supi", ue.Supi))

	_, span := tracer.Start(
		ctx,
		"AMF NAS HandleStatus5GMM",
		trace.WithAttributes(
			attribute.String("supi", ue.Supi),
			attribute.String("state", string(ue.State)),
		),
	)
	defer span.End()

	if ue.State == amfContext.Deregistered {
		return fmt.Errorf("UE is in Deregistered state, ignore Status 5GMM message")
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	cause := msg.Status5GMM.GetCauseValue()

	ue.Log.Error("Received Status 5GMM with cause", zap.String("Cause", nasMessage.Cause5GMMToString(cause)))

	return nil
}
