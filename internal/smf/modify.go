// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/ngap"
	libngap "github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// UpdateSmContextN2ModifyIndication rebinds the downlink tunnel to the NG-RAN's
// new transport address and returns a Modify Confirm Transfer (TS 38.413 §8.2.5.2).
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

	// A session torn down but still in the pool — startRelease leaves it there for
	// the whole T3592 window — has no tunnel to re-point.
	if smContext.Tunnel == nil {
		return nil, fmt.Errorf("sm context has no user-plane tunnel: %s", smContextRef)
	}

	qfis, err := handleModifyIndicationTransfer(n2Data, smContext)
	if err != nil {
		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	n2buf, err := ngap.BuildPDUSessionResourceModifyConfirmTransfer(
		smContext.Tunnel.N3TEID,
		smContext.Tunnel.N3IPv4,
		smContext.Tunnel.N3IPv6,
		qfis,
	)
	if err != nil {
		return nil, fmt.Errorf("build modify confirm transfer: %v", err)
	}

	if smContext.PFCPContext == nil {
		return nil, fmt.Errorf("pfcp session context not found for upf")
	}

	// bindAccessTunnel realigned the uplink PDR's OuterHeaderRemoval as well as
	// the downlink FAR, so both have to reach the UPF: an indication that moves
	// the RAN between address families would otherwise leave it decapsulating
	// uplink GTP-U as the wrong family.
	var (
		pdrList []*PDR
		farList []*FAR
	)

	if smContext.Tunnel.Activated {
		pdrList = []*PDR{smContext.Tunnel.UplinkPDR}
		farList = []*FAR{smContext.Tunnel.DownlinkPDR.FAR}
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.SEID,
		"",
		pdrList, farList, nil,
	)); err != nil {
		return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	s.registerIPv6SessionIfNeeded(ctx, smContext)

	logger.SmfLog.Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return n2buf, nil
}

// handleModifyIndicationTransfer rebinds the downlink access tunnel to the
// address in the Modify Indication Transfer and returns the associated QoS
// flows (TS 38.413 §8.2.5.2).
func handleModifyIndicationTransfer(b []byte, smContext *SMContext) ([]int64, error) {
	transfer, err := libngap.ParsePDUSessionResourceModifyIndicationTransfer(b)
	if err != nil {
		return nil, err
	}

	smContext.bindAccessTunnel(anchorFromGTPTunnel(transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel), Access5G)

	qfis := make([]int64, 0, len(transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList))
	for _, item := range transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList {
		qfis = append(qfis, int64(item.QosFlowIdentifier))
	}

	return qfis, nil
}
