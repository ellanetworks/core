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

// ReconcileSmContext applies an OAM-initiated policy change to an active PDU
// session.
//
// QoS/AMBR/DNS changes use the network-requested PDU Session Modification
// procedure (TS 23.502): PFCP update plus N1+N2 to the UE and gNB.
//
// Slice (SST/SD), MTU, or IP pool changes release the session with cause #39
// "reactivation requested" (TS 24.501) so the UE re-establishes with the
// correct configuration; TS 23.501 does not address dynamic MTU adjustment,
// and IP pools have no in-place modification mechanism.
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

	// A network-requested modification or release is already outstanding
	// (T3591/T3592 running); re-firing resends the command and resets the
	// retransmission counter, or double-frees on release. Defer to the next
	// backstop sweep.
	if smContext.procedureTimer.Active() {
		logger.SmfLog.Debug("a network-requested procedure is in flight, skipping reconciliation",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)

		return nil
	}

	// UE in CM-IDLE (user plane buffering): do not touch any enforcement point
	// while it is unreachable. The change is applied atomically to UPF + RAN + UE
	// when the UE reactivates and the reconcile re-runs on the resulting Initial
	// Context / PDU Session Resource Setup Response.
	if !smContext.upConnectionActive() {
		logger.SmfLog.Debug("UE idle (user plane buffering), deferring reconciliation to reactivation",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)

		return nil
	}

	// Slice (SST/SD) change: stored Snssai matches no configured slice. Release
	// with cause #39 so the UE re-establishes on the new slice (TS 23.502).
	if req.Reason == models.ReconcileSliceMismatch {
		return s.sendSessionRelease(ctx, smContext)
	}

	// MTU or IP pool change: release for re-establishment. Zero/empty delta
	// values mean "unspecified" (unchanged), avoiding spurious releases on a
	// partial delta.
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

	// Send N1 (NAS) + N2 (NGAP) to the UE and gNB (TS 23.502).
	//
	// If the UE is in CM-IDLE, N1N2 delivery is skipped (TS 23.502: the AMF may
	// ignore N2 SM information when the UE is unreachable) and the policy is
	// committed immediately so ActivateSmContext returns updated QoS when the UE
	// returns to CM-CONNECTED. If the UE is connected, the commit is deferred until
	// the UE answers the modification.
	ueIdle := false

	if (hasAmbrChange || hasQoSChange || hasDNSChange) && req.NewPolicy != nil {
		if err := s.sendSessionModification(ctx, smContext, newPolicy, hasAmbrChange, hasQoSChange, hasDNSChange); err != nil {
			if errors.Is(err, ErrUENotReachable) {
				ueIdle = true

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

	if (hasQoSChange || hasAmbrChange || hasDNSChange) && req.NewPolicy != nil {
		if ueIdle {
			// No procedure to await: commit now. ActivateSmContext rebuilds the Setup
			// Transfer from PolicyData, so the UE gets updated QoS on reconnect.
			smContext.PolicyData = newPolicy
		} else {
			// UE connected: the modification command is outstanding. Commit only when
			// the UE answers PDU SESSION MODIFICATION COMPLETE (TS 24.501 §6.3.2.2); a
			// reject or T3591 abort discards this and keeps the previous configuration
			// (§6.3.2.5), which the backstop then re-attempts.
			smContext.pendingPolicy = newPolicy
		}
	}

	return nil
}

// sendSessionModification builds and sends N1+N2 for the network-requested PDU
// Session Modification (TS 23.502): the PDU Session Modification Command (N1, to
// UE) and the PDU Session Resource Modify Request Transfer (N2, to gNB), each
// carrying only the changed AMBR/QoS IEs (TS 24.501). A DNS-only change sends N1
// alone, since DNS travels in the NAS Extended PCO and does not affect the gNB.
func (s *SMF) sendSessionModification(ctx context.Context, smContext *SMContext, policy *Policy, hasAmbrChange, hasQoSChange, hasDNSChange bool) error {
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

	// DNS travels in the NAS Extended PCO and needs no N2 signaling.
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

	if err := s.amf.ModifyN1N2(ctx, smContext.Supi, smContext.PDUSessionID, n1Msg, n2Msg); err != nil {
		return fmt.Errorf("transfer N1N2 message: %w", err)
	}

	// A network-requested modification uses PTI "no procedure transaction
	// identity assigned" (0) and awaits the UE's Modification Complete or
	// Command Reject (TS 24.501).
	smContext.MarkPTIInUse(0)

	// T3591 retransmits the command until the UE replies; on the final expiry
	// the procedure is aborted and the session stays PDU SESSION ACTIVE
	// (TS 24.501). The committed PFCP/policy change is not rolled back.
	supi := smContext.Supi
	pduSessionID := smContext.PDUSessionID
	s.armRetransmit(smContext, s.t3591,
		func() error { return s.amf.ModifyN1N2(context.Background(), supi, pduSessionID, n1Msg, n2Msg) },
		func(sc *SMContext) {
			sc.ClearPTIInUse(0)
			// Discard the uncommitted policy: the UE never confirmed, so the session
			// keeps its previous configuration and the backstop re-attempts (TS 24.501
			// §6.3.2.5).
			sc.pendingPolicy = nil

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

// updatePFCPRules pushes the policy's QoS (QFI + session-AMBR) to the UPF data
// plane (TS 29.244).
func (s *SMF) updatePFCPRules(ctx context.Context, smContext *SMContext, policy *Policy) error {
	return s.applySessionQERs(ctx, smContext, policy.PolicyID, policy.QosData.QFI, policy.Ambr.Uplink, policy.Ambr.Downlink)
}

// applySessionQERs sets every distinct session QER (deduped across UL/DL) to the
// given QFI and AMBR-derived MBR, marks them for update, and sends a PFCP Session
// Modification. On a failed modify it restores each QER's prior MBR/QFI/state, so
// the cached rules never run ahead of the data plane.
//
// QER MBR is set to the session AMBR because this implementation supports a single
// QoS flow per session (non-GBR only): per TS 23.501 the session AMBR is the
// aggregate non-GBR limit, which with one flow equals the per-flow MBR. If
// multiple or GBR flows are ever supported, this must use per-flow MBR values.
//
// The caller holds smContext.Mutex.
func (s *SMF) applySessionQERs(ctx context.Context, smContext *SMContext, policyID string, qfi uint8, ambrUplink, ambrDownlink string) error {
	if smContext.PFCPContext == nil || smContext.PFCPContext.RemoteSEID == 0 {
		return fmt.Errorf("PFCP session not established")
	}

	if smContext.Tunnel == nil || smContext.Tunnel.DataPath == nil {
		return fmt.Errorf("data path not available")
	}

	dataPath := smContext.Tunnel.DataPath

	type qerSnapshot struct {
		qer   *QER
		mbr   *models.MBR
		qfi   uint8
		state RuleState
	}

	var (
		qerList   []*QER
		snapshots []qerSnapshot
	)

	for _, t := range []*GTPTunnel{dataPath.UpLinkTunnel, dataPath.DownLinkTunnel} {
		if t == nil || t.PDR == nil || t.PDR.QER == nil {
			continue
		}

		qer := t.PDR.QER

		listed := false

		for _, q := range qerList {
			if q.QERID == qer.QERID {
				listed = true

				break
			}
		}

		if listed {
			continue
		}

		snapshots = append(snapshots, qerSnapshot{qer: qer, mbr: qer.MBR, qfi: qer.QFI, state: qer.State})

		qer.QFI = qfi
		qer.MBR = &models.MBR{
			ULMBR: bitRateTokbps(ambrUplink),
			DLMBR: bitRateTokbps(ambrDownlink),
		}
		qer.State = RuleUpdate
		qerList = append(qerList, qer)
	}

	if len(qerList) == 0 {
		return fmt.Errorf("no QERs to update")
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.RemoteSEID,
		policyID,
		nil, nil, qerList,
	)); err != nil {
		for _, snap := range snapshots {
			snap.qer.MBR = snap.mbr
			snap.qer.QFI = snap.qfi
			snap.qer.State = snap.state
		}

		return fmt.Errorf("failed to modify PFCP session: %w", err)
	}

	return nil
}

// sendSessionRelease performs the network-requested PDU session release
// (TS 23.502, TS 24.501) with cause #39 "reactivation requested" so the UE
// re-establishes on the correct slice. Caller must hold smContext.Mutex.
func (s *SMF) sendSessionRelease(ctx context.Context, smContext *SMContext) error {
	return s.startRelease(ctx, smContext, 0, nasMessage.Cause5GSMReactivationRequested)
}
