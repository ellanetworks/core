// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// UpdateSmContextN2ModifyIndication handles a PDU Session Resource Modify
// Indication Transfer from the NG-RAN (TS 38.413 §8.2.5.2): it takes the
// downlink transport address the NG-RAN provided as the new downlink address for
// the associated QoS flows and returns a Modify Confirm Transfer carrying the
// uplink tunnel and the confirmed QoS flows.
func (s *SMF) UpdateSmContextN2ModifyIndication(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_n2_modify_indication",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	qfis, err := handleModifyIndicationTransfer(n2Data, smContext)
	if err != nil {
		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	n2buf, err := ngap.BuildPDUSessionResourceModifyConfirmTransfer(
		smContext.Tunnel.DataPath.UpLinkTunnel.TEID,
		smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv4,
		smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv6,
		qfis,
	)
	if err != nil {
		return nil, fmt.Errorf("build modify confirm transfer: %v", err)
	}

	if smContext.PFCPContext == nil {
		return nil, fmt.Errorf("pfcp session context not found for upf")
	}

	var pdrList []*PDR

	var farList []*FAR

	if smContext.Tunnel.DataPath.Activated {
		pdrList = append(pdrList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR)
		farList = append(farList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR)
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.RemoteSEID,
		"",
		pdrList, farList, nil,
	)); err != nil {
		return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	// Re-register the IPv6 session with the new gNB tunnel endpoint.
	s.registerIPv6SessionIfNeeded(ctx, smContext)

	logger.SmfLog.Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return n2buf, nil
}

// handleModifyIndicationTransfer rebinds the downlink access tunnel to the
// address in the Modify Indication Transfer and returns the QoS flows the
// NG-RAN associated with it (TS 38.413 §8.2.5.2).
func handleModifyIndicationTransfer(b []byte, smContext *SMContext) ([]int64, error) {
	transfer := ngapType.PDUSessionResourceModifyIndicationTransfer{}

	if err := aper.UnmarshalWithParams(b, &transfer, "valueExt"); err != nil {
		return nil, err
	}

	tnl := transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation
	if tnl.Present != ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return nil, errors.New("modify indication transfer DL QoS flow per TNL information is not a GTP tunnel")
	}

	smContext.bindAccessTunnel(anchorFromGTPTunnel(tnl.GTPTunnel))

	if smContext.Tunnel.DataPath.Activated {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.State = RuleUpdate
	}

	qfis := make([]int64, 0, len(transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList.List))
	for _, item := range transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList.List {
		qfis = append(qfis, item.QosFlowIdentifier.Value)
	}

	return qfis, nil
}
