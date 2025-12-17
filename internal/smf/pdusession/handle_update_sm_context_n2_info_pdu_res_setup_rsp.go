package pdusession

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func UpdateSmContextN2InfoPduResSetupRsp(ctx ctxt.Context, smContextRef string, n2Data []byte) error {
	ctx, span := tracer.Start(ctx, "SMF Update SmContext PDU Resource Setup Response")
	defer span.End()
	span.SetAttributes(
		attribute.String("smf.smContextRef", smContextRef),
	)

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := context.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	pdrList, farList, err := handleUpdateN2MsgPDUResourceSetupResp(n2Data, smContext)
	if err != nil {
		return fmt.Errorf("error handling N2 message: %v", err)
	}

	sessionContext, exist := smContext.PFCPContext[smContext.Tunnel.DataPath.DPNode.UPF.NodeID.String()]
	if !exist {
		return fmt.Errorf("pfcp session context not found for upf: %s", smContext.Tunnel.DataPath.DPNode.UPF.NodeID.String())
	}

	err = pfcp.SendPfcpSessionModificationRequest(ctx, sessionContext.LocalSEID, sessionContext.RemoteSEID, pdrList, farList, nil)
	if err != nil {
		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	return nil
}

func handleUpdateN2MsgPDUResourceSetupResp(binaryDataN2SmInformation []byte, smContext *context.SMContext) ([]*context.PDR, []*context.FAR, error) {
	logger.SmfLog.Debug("received n2 sm info type", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	pdrList := []*context.PDR{}
	farList := []*context.FAR{}
	dataPath := smContext.Tunnel.DataPath
	if dataPath.Activated {
		ANUPF := dataPath.DPNode
		ANUPF.DownLinkTunnel.PDR.FAR.ApplyAction = context.ApplyAction{Buff: false, Drop: false, Dupl: false, Forw: true, Nocp: false}
		ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters = &context.ForwardingParameters{
			DestinationInterface: context.DestinationInterface{
				InterfaceValue: context.DestinationInterfaceAccess,
			},
			NetworkInstance: smContext.Dnn,
		}

		ANUPF.DownLinkTunnel.PDR.State = context.RuleUpdate
		ANUPF.DownLinkTunnel.PDR.FAR.State = context.RuleUpdate

		pdrList = append(pdrList, ANUPF.DownLinkTunnel.PDR)
		farList = append(farList, ANUPF.DownLinkTunnel.PDR.FAR)
	}

	err := context.HandlePDUSessionResourceSetupResponseTransfer(binaryDataN2SmInformation, smContext)
	if err != nil {
		return nil, nil, fmt.Errorf("handle PDUSessionResourceSetupResponseTransfer failed: %v", err)
	}

	return pdrList, farList, nil
}
