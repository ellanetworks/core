package pdusession

import (
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
)

func ActivateSmContext(smContextRef string) ([]byte, error) {
	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smContext := context.GetSMContext(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	n2Buf, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext.SmPolicyUpdates, smContext.SmPolicyData, smContext.Tunnel.DataPath.DPNode)
	if err != nil {
		return nil, fmt.Errorf("build PDUSession Resource Setup Request Transfer Error: %v", err)
	}

	return n2Buf, nil
}
