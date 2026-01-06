package pdusession

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	ctx, span := tracer.Start(
		ctx,
		"SMF Update SmContext Handover Path Switch Request",
		trace.WithAttributes(
			attribute.String("smf.smContextRef", smContextRef),
		),
	)
	defer span.End()

	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smf := smfContext.SMFSelf()

	smContext := smf.GetSMContext(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	pdrList, farList, n2buf, err := handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data, smContext)
	if err != nil {
		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	if smContext.PFCPContext == nil {
		return nil, fmt.Errorf("pfcp session context not found for upf")
	}

	err = pfcp.SendPfcpSessionModificationRequest(ctx, smf.CPNodeID, smContext.PFCPContext.LocalSEID, smContext.PFCPContext.RemoteSEID, pdrList, farList, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	return n2buf, nil
}

func handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data []byte, smContext *smfContext.SMContext) ([]*smfContext.PDR, []*smfContext.FAR, []byte, error) {
	logger.SmfLog.Debug("handle Path Switch Request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	if err := handlePathSwitchRequestTransfer(n2Data, smContext); err != nil {
		return nil, nil, nil, fmt.Errorf("handle PathSwitchRequestTransfer failed: %v", err)
	}

	n2Buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build Path Switch Transfer Error: %v", err)
	}

	pdrList := []*smfContext.PDR{}
	farList := []*smfContext.FAR{}

	if smContext.Tunnel.DataPath.Activated {
		pdrList = append(pdrList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR)
		farList = append(farList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR)
	}

	return pdrList, farList, n2Buf, nil
}

func handlePathSwitchRequestTransfer(b []byte, smContext *smfContext.SMContext) error {
	pathSwitchRequestTransfer := ngapType.PathSwitchRequestTransfer{}

	if err := aper.UnmarshalWithParams(b, &pathSwitchRequestTransfer, "valueExt"); err != nil {
		return err
	}

	if pathSwitchRequestTransfer.DLNGUUPTNLInformation.Present != ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return errors.New("pathSwitchRequestTransfer.DLNGUUPTNLInformation.Present")
	}

	gtpTunnel := pathSwitchRequestTransfer.DLNGUUPTNLInformation.GTPTunnel

	teid := binary.BigEndian.Uint32(gtpTunnel.GTPTEID.Value)

	smContext.Tunnel.ANInformation.IPAddress = gtpTunnel.TransportLayerAddress.Value.Bytes
	smContext.Tunnel.ANInformation.TEID = teid

	if smContext.Tunnel.DataPath.Activated {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = new(smfContext.OuterHeaderCreation)
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation.OuterHeaderCreationDescription = smfContext.OuterHeaderCreationGtpUUdpIpv4
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation.TeID = teid
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation.IPv4Address = gtpTunnel.TransportLayerAddress.Value.Bytes
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.State = smfContext.RuleUpdate
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.PFCPSMReqFlags = new(smfContext.PFCPSMReqFlags)
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.PFCPSMReqFlags.Sndem = true
	}

	return nil
}
