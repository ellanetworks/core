package pdusession

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func UpdateSmContextN2HandoverPrepared(smContextRef string, n2Data []byte) ([]byte, error) {
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

	if err := handleHandoverRequestAcknowledgeTransfer(n2Data, smContext); err != nil {
		return nil, fmt.Errorf("handle HandoverRequestAcknowledgeTransfer failed: %v", err)
	}

	n2Rsp, err := ngap.BuildHandoverCommandTransfer(smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
	if err != nil {
		return nil, fmt.Errorf("build Handover Command Transfer Error: %v", err)
	}

	return n2Rsp, nil
}

func handleHandoverRequestAcknowledgeTransfer(b []byte, smContext *context.SMContext) error {
	handoverRequestAcknowledgeTransfer := ngapType.HandoverRequestAcknowledgeTransfer{}

	err := aper.UnmarshalWithParams(b, &handoverRequestAcknowledgeTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall handover request acknowledge transfer: %s", err.Error())
	}

	DLNGUUPTNLInformation := handoverRequestAcknowledgeTransfer.DLNGUUPTNLInformation
	GTPTunnel := DLNGUUPTNLInformation.GTPTunnel
	TEIDReader := bytes.NewBuffer(GTPTunnel.GTPTEID.Value)

	teid, err := binary.ReadUvarint(TEIDReader)
	if err != nil {
		return fmt.Errorf("parse TEID error %s", err.Error())
	}

	if smContext.Tunnel.DataPath.Activated {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = new(context.OuterHeaderCreation)
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation.OuterHeaderCreationDescription = context.OuterHeaderCreationGtpUUdpIpv4
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation.TeID = uint32(teid)
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation.IPv4Address = GTPTunnel.TransportLayerAddress.Value.Bytes
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.State = context.RuleUpdate
	}

	return nil
}
