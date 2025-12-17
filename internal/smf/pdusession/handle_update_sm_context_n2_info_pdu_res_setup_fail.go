package pdusession

import (
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
)

func UpdateSmContextN2InfoPduResSetupFail(smContextRef string, n2Data []byte) error {
	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := context.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	err := context.HandlePDUSessionResourceSetupUnsuccessfulTransfer(n2Data)
	if err != nil {
		return fmt.Errorf("handle PDUSessionResourceSetupUnsuccessfulTransfer failed: %v", err)
	}

	return nil
}
