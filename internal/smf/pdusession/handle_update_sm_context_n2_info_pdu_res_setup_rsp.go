package pdusession

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func UpdateSmContextN2InfoPduResSetupRsp(ctx context.Context, smContextRef string, n2Data []byte) error {
	ctx, span := tracer.Start(
		ctx,
		"SMF Update SmContext PDU Resource Setup Response",
		trace.WithAttributes(
			attribute.String("smf.smContextRef", smContextRef),
		),
	)
	defer span.End()

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smf := smfContext.SMFSelf()

	smContext := smf.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	pdrList, farList, err := handleUpdateN2MsgPDUResourceSetupResp(n2Data, smContext)
	if err != nil {
		return fmt.Errorf("error handling N2 message: %v", err)
	}

	if smContext.PFCPContext == nil {
		return fmt.Errorf("pfcp session context not found")
	}

	err = pfcp.SendPfcpSessionModificationRequest(ctx, smf, smContext.PFCPContext.LocalSEID, smContext.PFCPContext.RemoteSEID, pdrList, farList, nil)
	if err != nil {
		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	return nil
}

func handleUpdateN2MsgPDUResourceSetupResp(binaryDataN2SmInformation []byte, smContext *smfContext.SMContext) ([]*smfContext.PDR, []*smfContext.FAR, error) {
	logger.SmfLog.Debug("received n2 sm info type", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	pdrList := []*smfContext.PDR{}
	farList := []*smfContext.FAR{}

	dataPath := smContext.Tunnel.DataPath
	if dataPath.Activated {
		ANUPF := dataPath.DPNode
		ANUPF.DownLinkTunnel.PDR.FAR.ApplyAction = smfContext.ApplyAction{Buff: false, Drop: false, Dupl: false, Forw: true, Nocp: false}
		ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters = &smfContext.ForwardingParameters{
			DestinationInterface: smfContext.DestinationInterface{
				InterfaceValue: smfContext.DestinationInterfaceAccess,
			},
			NetworkInstance: smContext.Dnn,
		}

		ANUPF.DownLinkTunnel.PDR.State = smfContext.RuleUpdate
		ANUPF.DownLinkTunnel.PDR.FAR.State = smfContext.RuleUpdate

		pdrList = append(pdrList, ANUPF.DownLinkTunnel.PDR)
		farList = append(farList, ANUPF.DownLinkTunnel.PDR.FAR)
	}

	err := smfContext.HandlePDUSessionResourceSetupResponseTransfer(binaryDataN2SmInformation, smContext)
	if err != nil {
		return nil, nil, fmt.Errorf("handle PDUSessionResourceSetupResponseTransfer failed: %v", err)
	}

	return pdrList, farList, nil
}
