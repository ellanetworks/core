package gmm

import (
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func handleStatus5GMM(ue *amfContext.AmfUe, msg *nasMessage.Status5GMM) error {
	if ue.State == amfContext.Deregistered {
		return fmt.Errorf("UE is in Deregistered state, ignore Status 5GMM message")
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	ue.Log.Error(
		"Received Status 5GMM with cause",
		zap.String("cause_5gmm", nasMessage.Cause5GMMToString(msg.GetCauseValue())),
		logger.ErrorCodeField("nas_status_5gmm_received"),
	)

	return nil
}
