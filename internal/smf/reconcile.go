// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ReconcileSmContext applies policy changes to an active PDU session.
// It compares the old and new policy data, updates the UPF via PFCP if QoS
// parameters changed, and sends N1+N2 messages to the UE and gNB when session
// QoS or AMBR changes.
//
// When the reason is ReconcileSliceMismatch, the session is released with
// cause #39 "reactivation requested" (TS 24.501 table 11.4.2.1) so the UE
// re-establishes on the correct slice.
//
// For QoS/AMBR changes this implements the 3GPP network-requested PDU Session
// Modification procedure (TS 23.502 clause 4.3.3.2) triggered by an
// OAM-initiated policy update.
func (s *SMF) ReconcileSmContext(ctx context.Context, req *models.SessionReconcileRequest) error {
	if req == nil {
		return fmt.Errorf("reconcile request is nil")
	}

	ctx, span := tracer.Start(ctx, "smf/reconcile_sm_context",
		trace.WithAttributes(
			attribute.String("smf.smContextRef", req.SmContextRef),
			attribute.String("smf.reason", string(req.Reason)),
		),
	)
	defer span.End()

	if req.SmContextRef == "" {
		span.SetStatus(codes.Error, "sm context reference is missing")

		return fmt.Errorf("sm context reference is missing")
	}

	smContext := s.GetSession(req.SmContextRef)
	if smContext == nil {
		span.SetStatus(codes.Error, "sm context not found")

		return fmt.Errorf("sm context not found: %s", req.SmContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil || !smContext.Tunnel.DataPath.Activated {
		logger.SmfLog.Debug("session not activated, skipping reconciliation",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)

		return nil
	}

	if smContext.PolicyData == nil {
		logger.SmfLog.Debug("session has no policy data, skipping reconciliation",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)

		return nil
	}

	// --- Slice mismatch: network-initiated release ---
	// When SST/SD changed, the session's stored Snssai no longer matches any
	// slice. Per TS 23.502 §4.3.4.2, release with cause #39 so the UE
	// automatically re-establishes on the new slice.
	if req.Reason == models.ReconcileSliceMismatch {
		return s.sendSessionRelease(ctx, smContext)
	}

	// --- QoS/AMBR modification path ---
	oldQoS := smContext.PolicyData.QosData
	oldAmbr := smContext.PolicyData.Ambr

	hasQoSChange := false
	hasAmbrChange := false

	if req.NewPolicy != nil {
		oldArp := int32(0)
		if oldQoS.Arp != nil {
			oldArp = oldQoS.Arp.PriorityLevel
		}

		if oldQoS.Var5qi != req.NewPolicy.Var5qi || oldArp != req.NewPolicy.Arp {
			hasQoSChange = true
		}

		if oldAmbr.Uplink != req.NewPolicy.SessionAmbrUplink || oldAmbr.Downlink != req.NewPolicy.SessionAmbrDownlink {
			hasAmbrChange = true
		}
	}

	newPolicy := smContext.PolicyData
	if (hasQoSChange || hasAmbrChange) && req.NewPolicy != nil {
		newPolicy = &Policy{
			PolicyID: smContext.PolicyData.PolicyID,
			Ambr: models.Ambr{
				Uplink:   req.NewPolicy.SessionAmbrUplink,
				Downlink: req.NewPolicy.SessionAmbrDownlink,
			},
			QosData: models.QosData{
				QFI:    smContext.PolicyData.QosData.QFI,
				Var5qi: req.NewPolicy.Var5qi,
				Arp: &models.Arp{
					PriorityLevel: req.NewPolicy.Arp,
					PreemptCap:    req.NewPolicy.PreemptCap,
					PreemptVuln:   req.NewPolicy.PreemptVuln,
				},
			},
			NetworkRules: smContext.PolicyData.NetworkRules,
			DNS:          smContext.PolicyData.DNS,
			MTU:          smContext.PolicyData.MTU,
			IPv4Pool:     smContext.PolicyData.IPv4Pool,
			IPv6Pool:     smContext.PolicyData.IPv6Pool,
		}
	}

	// Push QoS changes to the UPF via PFCP Session Modification.
	if hasQoSChange || hasAmbrChange {
		if err := s.updatePFCPRules(ctx, smContext, newPolicy); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to update PFCP rules")

			logger.SmfLog.Error("failed to update PFCP rules during reconciliation",
				zap.Error(err),
				logger.SUPI(smContext.Supi.String()),
				logger.PDUSessionID(smContext.PDUSessionID),
				zap.String("reason", string(req.Reason)),
			)

			return fmt.Errorf("failed to update PFCP rules: %v", err)
		}

		logger.SmfLog.Info("PFCP rules updated during reconciliation",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
			zap.Bool("qosChange", hasQoSChange),
			zap.Bool("ambrChange", hasAmbrChange),
			zap.String("reason", string(req.Reason)),
		)
	}

	// Send N1 (NAS) + N2 (NGAP) messages to notify the UE and gNB of changes.
	// Per TS 23.502 clause 4.3.3.2, the SMF sends:
	//   - N1: PDU Session Modification Command (Session-AMBR, QoS flow descriptions)
	//   - N2: PDU Session Resource Modify Request Transfer (PDU Session Aggregate
	//     Maximum Bit Rate, QoS Flow Add or Modify Request List with QoS profile)
	//
	// If the UE is in CM-IDLE, N1N2 delivery is skipped (per TS 23.502 §4.2.3.3
	// step 3b: the AMF may ignore the N2 SM information when the UE is not
	// reachable). The policy is still committed so that ActivateSmContext
	// returns updated QoS when the UE transitions back to CM-CONNECTED.
	if (hasAmbrChange || hasQoSChange) && req.NewPolicy != nil {
		if err := s.sendSessionModification(ctx, smContext, newPolicy, hasAmbrChange, hasQoSChange); err != nil {
			if errors.Is(err, ErrUENotReachable) {
				logger.SmfLog.Debug("UE is idle, skipping N1N2 delivery; policy committed for next activation",
					logger.SUPI(smContext.Supi.String()),
					logger.PDUSessionID(smContext.PDUSessionID),
				)
			} else {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to send session modification to UE/gNB")

				logger.SmfLog.Error("failed to send session modification to UE/gNB",
					zap.Error(err),
					logger.SUPI(smContext.Supi.String()),
					logger.PDUSessionID(smContext.PDUSessionID),
				)

				return fmt.Errorf("failed to send session modification: %v", err)
			}
		}
	}

	// Commit policy data after PFCP succeeds. N1/N2 delivery may be skipped
	// (UE idle) — this is intentional: ActivateSmContext rebuilds the Setup
	// Transfer from PolicyData, so the UE gets updated QoS on reconnect.
	// If PFCP or a real N1/N2 failure returned above, we never reach here.
	if (hasQoSChange || hasAmbrChange) && req.NewPolicy != nil {
		smContext.PolicyData = newPolicy
	}

	return nil
}

// sendSessionModification builds and sends N1+N2 messages for the network-
// requested PDU Session Modification (TS 23.502 §4.3.3.2).
//
// N1 (to UE): PDU Session Modification Command containing:
//   - Session-AMBR (when AMBR changed, TS 24.501 §8.3.9.3)
//   - Authorized QoS flow descriptions (when 5QI/ARP changed, TS 24.501 §8.3.9.8)
//
// N2 (to gNB): PDU Session Resource Modify Request Transfer containing:
//   - PDU Session Aggregate Maximum Bit Rate (when AMBR changed)
//   - QoS Flow Add or Modify Request List (when 5QI/ARP changed)
func (s *SMF) sendSessionModification(ctx context.Context, smContext *SMContext, policy *Policy, hasAmbrChange, hasQoSChange bool) error {
	// Build N1 NAS message.
	var n1Ambr *models.Ambr
	if hasAmbrChange {
		n1Ambr = &policy.Ambr
	}

	var n1QoS *models.QosData
	if hasQoSChange {
		n1QoS = &policy.QosData
	}

	n1Msg, err := nas.BuildPDUSessionModificationCommand(smContext.PDUSessionID, n1Ambr, n1QoS)
	if err != nil {
		return fmt.Errorf("build PDU Session Modification Command (N1): %w", err)
	}

	// Build N2 NGAP message.
	var n2Ambr *models.Ambr
	if hasAmbrChange {
		n2Ambr = &policy.Ambr
	}

	var n2QoS *models.QosData
	if hasQoSChange {
		n2QoS = &policy.QosData
	}

	n2Msg, err := ngap.BuildPDUSessionResourceModifyRequestTransfer(n2Ambr, n2QoS)
	if err != nil {
		return fmt.Errorf("build PDU Session Resource Modify Request Transfer (N2): %w", err)
	}

	// Deliver combined N1+N2 via the AMF. The AMF will send a
	// PDUSessionResourceModifyRequest (TS 38.413 §9.2.1.5) to the gNB,
	// carrying the NAS PDU piggy-backed in the per-session modify item.
	if err := s.amf.ModifyN1N2(ctx, smContext.Supi, smContext.PDUSessionID, n1Msg, n2Msg); err != nil {
		return fmt.Errorf("transfer N1N2 message: %w", err)
	}

	logger.SmfLog.Info("session modification N1+N2 sent",
		logger.SUPI(smContext.Supi.String()),
		logger.PDUSessionID(smContext.PDUSessionID),
		zap.Bool("ambrChange", hasAmbrChange),
		zap.Bool("qosChange", hasQoSChange),
	)

	return nil
}

// updatePFCPRules rebuilds the QER with the new rate limits and sends a PFCP
// Session Modification Request to the UPF. This enforces the updated QoS
// parameters in the data plane (TS 29.244 clause 7.5.4).
func (s *SMF) updatePFCPRules(ctx context.Context, smContext *SMContext, policy *Policy) error {
	if smContext.PFCPContext == nil || smContext.PFCPContext.RemoteSEID == 0 {
		return fmt.Errorf("PFCP session not established")
	}

	if smContext.Tunnel == nil || smContext.Tunnel.DataPath == nil {
		return fmt.Errorf("data path not available")
	}

	dataPath := smContext.Tunnel.DataPath
	qerList := make([]*QER, 0, 2)

	// Collect QERs from both tunnel directions.
	// Note: QER MBR is set to session AMBR because this implementation supports
	// a single QoS flow per PDU session (non-GBR only). Per TS 23.501 §5.7.2.2,
	// session AMBR is the aggregate limit across all non-GBR flows. With only
	// one flow, session AMBR equals the per-flow MBR. If multiple flows or GBR
	// flows are ever supported, this must be changed to use per-flow MBR values.
	if dataPath.UpLinkTunnel != nil && dataPath.UpLinkTunnel.PDR != nil && dataPath.UpLinkTunnel.PDR.QER != nil {
		qer := dataPath.UpLinkTunnel.PDR.QER
		qer.QFI = policy.QosData.QFI
		qer.MBR = &models.MBR{
			ULMBR: bitRateTokbps(policy.Ambr.Uplink),
			DLMBR: bitRateTokbps(policy.Ambr.Downlink),
		}
		qer.State = RuleUpdate
		qerList = append(qerList, qer)
	}

	if dataPath.DownLinkTunnel != nil && dataPath.DownLinkTunnel.PDR != nil && dataPath.DownLinkTunnel.PDR.QER != nil {
		found := false

		for _, existing := range qerList {
			if existing.QERID == dataPath.DownLinkTunnel.PDR.QER.QERID {
				found = true

				break
			}
		}

		if !found && dataPath.DownLinkTunnel.PDR.QER != nil {
			qer := dataPath.DownLinkTunnel.PDR.QER
			qer.QFI = policy.QosData.QFI
			qer.MBR = &models.MBR{
				ULMBR: bitRateTokbps(policy.Ambr.Uplink),
				DLMBR: bitRateTokbps(policy.Ambr.Downlink),
			}
			qer.State = RuleUpdate
			qerList = append(qerList, qer)
		}
	}

	if len(qerList) == 0 {
		return fmt.Errorf("no QERs found to update")
	}

	var policyID string
	if policy != nil {
		policyID = policy.PolicyID
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.RemoteSEID,
		policyID,
		nil, nil, qerList,
	)); err != nil {
		return fmt.Errorf("failed to modify PFCP session: %v", err)
	}

	return nil
}

// sendSessionRelease performs a network-initiated PDU Session Release:
// This implements TS 23.502 §4.3.4.2 (network-requested PDU session release)
// for the case where the subscriber's slice assignment (SST/SD) has changed.
//
// Per TS 23.502 §4.3.4.2:
//  1. Releases IP addresses and tears down the data plane (PFCP session
//     deletion + GTP tunnel release) — Step 2 in the spec.
//  2. Builds N1 PDU Session Release Command with cause #39 "reactivation
//     requested" and N2 PDU Session Resource Release Command Transfer.
//  3. Sends the combined N1+N2 to UE/gNB via the AMF — Step 3 in the spec.
//  4. Removes the session from the SMF pool.
//
// Upon receiving cause #39 "reactivation requested" (TS 24.501 table 5.5.10),
// the UE is expected to initiate a new PDU session establishment
// (TS 24.501 §6.3.3). The UE's PDU Session Release Complete and gNB's N2
// Resource Release Ack will find the session already removed and return
// idempotently (see UpdateSmContextN2InfoPduResRelRsp).
//
// Caller must hold smContext.Mutex.Lock().
func (s *SMF) sendSessionRelease(ctx context.Context, smContext *SMContext) error {
	// Step 2 (TS 23.502 §4.3.4.2): Release IP addresses and user plane
	// resources before signaling the UE.
	if smContext.PDUIPV4Address != nil {
		if _, err := s.store.ReleaseIP(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID); err != nil {
			logger.SmfLog.Warn("release UE IPv4 address failed during slice-mismatch release, continuing teardown",
				zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}
	}

	if smContext.PDUIPV6Prefix != nil {
		if _, err := s.store.ReleaseIPv6(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID); err != nil {
			logger.SmfLog.Warn("release UE IPv6 address failed during slice-mismatch release, continuing teardown",
				zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}
	}

	if err := s.releaseTunnel(ctx, smContext); err != nil {
		logger.SmfLog.Warn("release tunnel failed during slice-mismatch release, continuing teardown",
			zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
	}

	// Step 3: Build N1+N2 and signal the UE/gNB.
	// Per TS 23.502 §4.3.4.2 step 3: if UE is in CM-IDLE, the AMF uses the
	// "skip indicator" — NAS delivery is not possible without radio resources.
	// The session is released locally (data plane already torn down above).
	// When the UE reconnects, the PDUSessionStatus IE in the Service Request
	// or Registration Request will not include this session, confirming the
	// release implicitly (TS 24.501 §5.4.4).
	n1Msg, err := nas.BuildNetworkInitiatedPDUSessionReleaseCommand(
		smContext.PDUSessionID,
		nasMessage.Cause5GSMReactivationRequested,
	)
	if err != nil {
		return fmt.Errorf("build PDU Session Release Command (N1): %w", err)
	}

	n2Transfer, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
	if err != nil {
		return fmt.Errorf("build PDU Session Resource Release Command Transfer (N2): %w", err)
	}

	if err := s.amf.ReleaseSession(ctx, smContext.Supi, smContext.PDUSessionID, n1Msg, n2Transfer); err != nil {
		if errors.Is(err, ErrUENotReachable) {
			logger.SmfLog.Debug("UE is idle, skipping release signaling; session removed locally",
				logger.SUPI(smContext.Supi.String()),
				logger.PDUSessionID(smContext.PDUSessionID),
			)
		} else {
			return fmt.Errorf("release session signaling: %w", err)
		}
	}

	// Remove session from the SMF pool. When the gNB/UE response arrives at
	// UpdateSmContextN2InfoPduResRelRsp, the session will already be gone and
	// the handler returns nil (idempotent).
	s.removeSessionUnlocked(ctx, smContext.CanonicalName())

	logger.SmfLog.Info("network-initiated session release complete (slice mismatch)",
		logger.SUPI(smContext.Supi.String()),
		logger.PDUSessionID(smContext.PDUSessionID),
		zap.String("cause", "reactivation_requested"),
	)

	return nil
}
