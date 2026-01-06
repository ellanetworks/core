package pdusession

import (
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
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

	if err := handleHandoverRequiredTransfer(n2Data); err != nil {
		return nil, fmt.Errorf("handle HandoverRequiredTransfer failed: %v", err)
	}

	n2Rsp, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(smContext.PolicyData, smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
	if err != nil {
		return nil, fmt.Errorf("build PDUSession Resource Setup Request Transfer Error: %v", err)
	}

	return n2Rsp, nil
}

func handleHandoverRequiredTransfer(b []byte) error {
	handoverRequiredTransfer := ngapType.HandoverRequiredTransfer{}

	err := aper.UnmarshalWithParams(b, &handoverRequiredTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall handover required transfer: %s", err.Error())
	}

	return nil
}
