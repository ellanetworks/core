package pdusession

import (
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
)

func UpdateSmContextN2HandoverPreparing(smContextRef string, n2Data []byte) ([]byte, error) {
	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smf := context.SMFSelf()

	smContext := smf.GetSMContext(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if err := context.HandleHandoverRequiredTransfer(n2Data); err != nil {
		return nil, fmt.Errorf("handle HandoverRequiredTransfer failed: %v", err)
	}

	n2Rsp, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext.PolicyData.SessionRule, smContext.PolicyData.QosData, smContext.Tunnel.DataPath.DPNode)
	if err != nil {
		return nil, fmt.Errorf("build PDUSession Resource Setup Request Transfer Error: %v", err)
	}

	return n2Rsp, nil
}
