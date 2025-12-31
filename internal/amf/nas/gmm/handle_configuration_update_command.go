package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func handleConfigurationUpdateComplete(ctx context.Context, ue *amfContext.AmfUe) error {
	logger.AmfLog.Debug("Handle Configuration Update Complete", zap.String("supi", ue.Supi))

	if ue.State != amfContext.Registered {
		return fmt.Errorf("state mismatch: receive Configuration Update Complete message in state %s", ue.State)
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3555 != nil {
		ue.T3555.Stop()
		ue.T3555 = nil // clear the timer
	}

	ue.FreeOldGuti()

	return nil
}
