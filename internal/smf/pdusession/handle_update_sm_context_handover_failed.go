package pdusession

import (
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func UpdateSmContextHandoverFailed(smContextRef string, n2Data []byte) error {
	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smf := context.SMFSelf()

	smContext := smf.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	err := handlePathSwitchRequestSetupFailedTransfer(n2Data)
	if err != nil {
		return fmt.Errorf("handle PathSwitchRequestSetupFailedTransfer failed: %v", err)
	}

	return nil
}

func handlePathSwitchRequestSetupFailedTransfer(b []byte) error {
	pathSwitchRequestSetupFailedTransfer := ngapType.PathSwitchRequestSetupFailedTransfer{}

	err := aper.UnmarshalWithParams(b, &pathSwitchRequestSetupFailedTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall path switch request setup failed transfer: %s", err.Error())
	}

	return nil
}
