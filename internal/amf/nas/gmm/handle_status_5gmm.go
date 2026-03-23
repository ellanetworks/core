package gmm

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
)

func handleStatus5GMM(ue *amf.AmfUe, msg *nasMessage.Status5GMM) error {
	if ue.GetState() == amf.Deregistered {
		return fmt.Errorf("UE is in Deregistered state, ignore Status 5GMM message")
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	ue.Log.Error("Received Status 5GMM with cause", logger.Cause(nasMessage.Cause5GMMToString(msg.GetCauseValue())))

	return nil
}
