package pdusession

import (
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
)

func UpdateSmContextHandoverFailed(smContextRef string, n2Data []byte) error {
	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := context.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	err := context.HandlePathSwitchRequestSetupFailedTransfer(n2Data)
	if err != nil {
		return fmt.Errorf("handle PathSwitchRequestSetupFailedTransfer failed: %v", err)
	}

	return nil
}
