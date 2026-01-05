package pdusession

import (
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
)

func ActivateSmContext(smContextRef string) ([]byte, error) {
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

	n2Buf, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext.PolicyData.SessionRule, smContext.PolicyData.QosData, smContext.Tunnel.DataPath.DPNode.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.DPNode.UpLinkTunnel.N3IP)
	if err != nil {
		return nil, fmt.Errorf("build PDUSession Resource Setup Request Transfer Error: %v", err)
	}

	return n2Buf, nil
}
