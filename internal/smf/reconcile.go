// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"
	"net"

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
// For QoS/AMBR/DNS changes this implements the 3GPP network-requested PDU Session
// Modification procedure (TS 23.502 clause 4.3.3.2) triggered by an
// OAM-initiated policy update.
//
// For MTU or IP pool changes the session is released with cause #39 so the UE
// re-establishes with the updated configuration (TS 23.501 §5.6.10.4 does not
// address dynamic MTU adjustment; IP pools have no in-place modification
// mechanism).
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

	// --- MTU or IP pool change: network-initiated release ---
	// Per TS 23.501 §5.6.10.4 NOTE 3, dynamic MTU adjustment during an active
	// session is not addressed. IP pool changes would invalidate already-
	// assigned addresses. Release with cause #39 so the UE re-establishes.
	//
	// Zero/empty values in the delta are treated as "not specified" (unchanged)
	// to avoid spurious releases when the caller provides a partial delta.
	if req.NewPolicy != nil {
		mtuChanged := req.NewPolicy.MTU != 0 && smContext.PolicyData.MTU != req.NewPolicy.MTU
		ipv4PoolChanged := req.NewPolicy.IPv4Pool != "" && smContext.PolicyData.IPv4Pool != req.NewPolicy.IPv4Pool
		ipv6PoolChanged := req.NewPolicy.IPv6Pool != "" && smContext.PolicyData.IPv6Pool != req.NewPolicy.IPv6Pool

		if mtuChanged || ipv4PoolChanged || ipv6PoolChanged {
			logger.SmfLog.Info("MTU or IP pool changed, releasing session for re-establishment",
				logger.SUPI(smContext.Supi.String()),
				logger.PDUSessionID(smContext.PDUSessionID),
				zap.Uint16("oldMTU", smContext.PolicyData.MTU),
				zap.Uint16("newMTU", req.NewPolicy.MTU),
				zap.String("oldIPv4Pool", smContext.PolicyData.IPv4Pool),
				zap.String("newIPv4Pool", req.NewPolicy.IPv4Pool),
				zap.String("oldIPv6Pool", smContext.PolicyData.IPv6Pool),
				zap.String("newIPv6Pool", req.NewPolicy.IPv6Pool),
			)

			return s.sendSessionRelease(ctx, smContext)
		}
	}

	// --- QoS/AMBR/DNS modification path ---
	oldQoS := smContext.PolicyData.QosData
	oldAmbr := smContext.PolicyData.Ambr

	hasQoSChange := false
	hasAmbrChange := false
	hasDNSChange := false

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

		oldDNS := ""
		if smContext.PolicyData.DNS != nil {
			oldDNS = smContext.PolicyData.DNS.String()
		}

		if req.NewPolicy.DNS != "" && oldDNS != req.NewPolicy.DNS {
			hasDNSChange = true
		}
	}

	newPolicy := smContext.PolicyData
	if (hasQoSChange || hasAmbrChange || hasDNSChange) && req.NewPolicy != nil {
		dns := smContext.PolicyData.DNS
		if hasDNSChange {
			dns = net.ParseIP(req.NewPolicy.DNS)
			if dns == nil {
				return fmt.Errorf("invalid DNS address %q in new policy", req.NewPolicy.DNS)
			}
		}

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
			DNS:          dns,
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
	//   - N1: PDU Session Modification Command (Session-AMBR, QoS flow
	//     descriptions, DNS server addresses via Extended PCO)
	//   - N2: PDU Session Resource Modify Request Transfer (PDU Session
	//     Aggregate Maximum Bit Rate, QoS Flow Add or Modify Request List)
	//
	// If the UE is in CM-IDLE, N1N2 delivery is skipped (per TS 23.502 §4.2.3.3
	// step 3b: the AMF may ignore the N2 SM information when the UE is not
	// reachable). The policy is still committed so that ActivateSmContext
	// returns updated QoS when the UE transitions back to CM-CONNECTED.
	if (hasAmbrChange || hasQoSChange || hasDNSChange) && req.NewPolicy != nil {
		if err := s.sendSessionModification(ctx, smContext, newPolicy, hasAmbrChange, hasQoSChange, hasDNSChange); err != nil {
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
	if (hasQoSChange || hasAmbrChange || hasDNSChange) && req.NewPolicy != nil {
		smContext.PolicyData = newPolicy
	}

	return nil
}

// sendSessionModification builds and sends N1+N2 messages for the network-
// requested PDU Session Modification (TS 23.502 §4.3.3.2).
//
// N1 (to UE): PDU Session Modification Command containing:
//   - Session-AMBR IE (when ambr changed, TS 24.501 §8.3.9.3)
//   - Authorized QoS flow descriptions IE (when 5QI/ARP changed, TS 24.501 §8.3.9.8)
//   - Extended PCO with DNS server address(es) (when DNS changed, TS 24.501 §6.3.2)
//
// N2 (to gNB): PDU Session Resource Modify Request Transfer containing:
//   - PDU Session Aggregate Maximum Bit Rate (when AMBR changed)
//   - QoS Flow Add or Modify Request List (when 5QI/ARP changed)
//
// When only DNS changes (no AMBR/QoS), only N1 is sent since DNS is a NAS-level
// parameter that does not affect the gNB.
func (s *SMF) sendSessionModification(ctx context.Context, smContext *SMContext, policy *Policy, hasAmbrChange, hasQoSChange, hasDNSChange bool) error {
	// Build N1 NAS message.
	var n1Ambr *models.Ambr
	if hasAmbrChange {
		n1Ambr = &policy.Ambr
	}

	var n1QoS *models.QosData
	if hasQoSChange {
		n1QoS = &policy.QosData
	}

	var n1DNS net.IP
	if hasDNSChange && policy.DNS != nil {
		n1DNS = policy.DNS
	}

	n1Msg, err := nas.BuildPDUSessionModificationCommand(smContext.PDUSessionID, n1Ambr, n1QoS, n1DNS)
	if err != nil {
		return fmt.Errorf("build PDU Session Modification Command (N1): %w", err)
	}

	// Build N2 NGAP message only when AMBR or QoS changed.
	// DNS is carried in the NAS Extended PCO and does not require N2 signaling.
	var n2Msg []byte

	if hasAmbrChange || hasQoSChange {
		var n2Ambr *models.Ambr
		if hasAmbrChange {
			n2Ambr = &policy.Ambr
		}

		var n2QoS *models.QosData
		if hasQoSChange {
			n2QoS = &policy.QosData
		}

		n2Msg, err = ngap.BuildPDUSessionResourceModifyRequestTransfer(n2Ambr, n2QoS)
		if err != nil {
			return fmt.Errorf("build PDU Session Resource Modify Request Transfer (N2): %w", err)
		}
	}

	// Deliver combined N1+N2 (or N1-only for DNS changes) via the AMF.
	// The AMF will send a PDUSessionResourceModifyRequest (TS 38.413 §9.2.1.5)
	// to the gNB, carrying the NAS PDU piggy-backed in the per-session modify item.
	if err := s.amf.ModifyN1N2(ctx, smContext.Supi, smContext.PDUSessionID, n1Msg, n2Msg); err != nil {
		return fmt.Errorf("transfer N1N2 message: %w", err)
	}

	// A network-requested modification uses PTI "no procedure transaction
	// identity assigned" (0) and awaits the UE's Modification Complete or
	// Command Reject (TS 24.501 §6.3.2, §7.3.1 a).
	smContext.MarkPTIInUse(0)

	// T3591 retransmits the command until the UE replies; on the final expiry
	// the procedure is aborted and the session stays PDU SESSION ACTIVE
	// (TS 24.501 §6.3.2.5). The committed PFCP/policy change is not rolled back.
	supi := smContext.Supi
	pduSessionID := smContext.PDUSessionID
	s.armRetransmit(smContext, s.t3591,
		func() error { return s.amf.ModifyN1N2(context.Background(), supi, pduSessionID, n1Msg, n2Msg) },
		func(sc *SMContext) {
			sc.ClearPTIInUse(0)
			logger.SmfLog.Warn("T3591 expired; PDU session modification aborted, session remains active",
				logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))
		})

	logger.SmfLog.Info("session modification N1+N2 sent",
		logger.SUPI(smContext.Supi.String()),
		logger.PDUSessionID(smContext.PDUSessionID),
		zap.Bool("ambrChange", hasAmbrChange),
		zap.Bool("qosChange", hasQoSChange),
		zap.Bool("dnsChange", hasDNSChange),
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

// sendSessionRelease performs the network-requested PDU session release
// (TS 23.502 §4.3.4.2, TS 24.501 §6.3.3) triggered by a slice (SST/SD) change,
// using cause #39 "reactivation requested" so the UE re-establishes on the
// correct slice (TS 24.501 table 11.4.2.1). Caller must hold smContext.Mutex.
func (s *SMF) sendSessionRelease(ctx context.Context, smContext *SMContext) error {
	return s.startRelease(ctx, smContext, 0, nasMessage.Cause5GSMReactivationRequested)
}
