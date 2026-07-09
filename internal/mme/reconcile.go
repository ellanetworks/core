// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net/netip"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// ReconcileDataNetwork re-evaluates every connected EPS bearer against the
// current subscription and data-network configuration. For a DNS-only change it
// updates the bearer in place with a MODIFY EPS BEARER CONTEXT REQUEST
// (TS 24.301 §6.4.2); for an IP-pool or MTU change — which the UE cannot adopt
// without a new address or link config — it deactivates the bearer with ESM
// cause #39 "reactivation requested" (TS 24.301 §6.4.4.2) so the UE
// re-establishes.
func (m *MME) ReconcileDataNetwork(ctx context.Context) {
	for _, ue := range m.ConnectedUEs() {
		m.ReconcileUE(ctx, ue)
	}
}

// ReconcileUE reconciles every PDN connection of a UE against the current
// data-network configuration. Only a registered UE with an active S1 connection
// is signalled; an idle UE is signalled when it returns to ECM-CONNECTED
// (reconcileBearer on the ICS Response) or by the next backstop sweep.
func (m *MME) ReconcileUE(ctx context.Context, ue *UeContext) {
	// ue.active is freed concurrently by a release goroutine, and reconciliation is
	// deferred while an S1 handover is in flight (an E-RAB Modify or Release would
	// collide with the handover's bearer signalling, TS 36.413 §8.4.1.2); the next
	// sweep re-converges the UE.
	if !m.ReconcileReady(ue) {
		return
	}

	for _, p := range m.SnapshotPDNs(ue) {
		m.reconcileBearer(ctx, ue, p)
	}
}

// ClearPendingModifyLocked clears a PDN connection's in-flight modification
// bookkeeping. The caller holds ue.mu.
func ClearPendingModifyLocked(p *PdnConnection) {
	p.Modifying = false
	p.PendingDNConfig = ""
	p.PendingSessAmbrDLBps = 0
	p.PendingSessAmbrULBps = 0
	p.PendingQCI = 0
	p.PendingARP = 0
}

// reconcileBearer reconciles a single PDN connection against its current policy
// and data-network configuration.
func (m *MME) reconcileBearer(ctx context.Context, ue *UeContext, p *PdnConnection) {
	// Snapshot the connection's mutable policy state under the lock so a NAS
	// handler or the NAS-guard timer does not mutate the in-flight flags or the
	// stored config while the reconciler reads them.
	ue.mu.Lock()

	busy := p.Deactivating || p.Modifying
	curDNConfig := p.DnConfig
	curSessAmbrDLBps, curSessAmbrULBps := p.SessAmbrDLBps, p.SessAmbrULBps
	curQCI, curARP := p.Qci, p.Arp

	ue.mu.Unlock()

	if busy {
		return
	}

	qos, err := ResolveQoSByAPN(ctx, m, ue.IMSI(), p.Apn)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("reconcile: failed to resolve QoS for APN",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Error(err))

		return
	}

	newFingerprint := qos.DnFingerprint()
	dnChanged := newFingerprint != curDNConfig

	ambrChanged := BitRateToBps(qos.SessAmbrDLStr) != curSessAmbrDLBps ||
		BitRateToBps(qos.SessAmbrULStr) != curSessAmbrULBps

	qosChanged := qos.QCI != curQCI || qos.ARP != curARP

	if !dnChanged && !ambrChanged && !qosChanged {
		return
	}

	// An IP-pool or MTU change cannot be adopted in place; reactivate so the UE
	// re-establishes (the new bearer also picks up the new QoS/Session-AMBR).
	if dnChanged && !dnsOnlyChange(curDNConfig, newFingerprint) {
		logger.From(ctx, ue.Conn().Log).Info("data-network configuration changed; reactivating EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))
		m.reactivateBearer(ctx, ue, p)

		return
	}

	logger.From(ctx, ue.Conn().Log).Info("policy/data-network changed; modifying EPS bearer in place",
		zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn),
		zap.Bool("dns", dnChanged), zap.Bool("session-ambr", ambrChanged), zap.Bool("qos", qosChanged))
	m.modifyBearer(ctx, ue, p, qos, dnChanged, ambrChanged, qosChanged)
}

// dnsOnlyChange reports whether the data-network fingerprint changed in the DNS
// field alone (IP pools and MTU unchanged), so the bearer can be modified in
// place without reactivation. A malformed stored fingerprint returns false,
// so the caller falls back to the safe reactivation path. The fingerprint is
// "ipv4pool|ipv6pool|dns|mtu" (EpsQoS.DnFingerprint).
func dnsOnlyChange(oldFingerprint, newFingerprint string) bool {
	o := strings.Split(oldFingerprint, "|")
	n := strings.Split(newFingerprint, "|")

	if len(o) != 4 || len(n) != 4 {
		return false
	}

	return o[2] != n[2] && o[0] == n[0] && o[1] == n[1] && o[3] == n[3]
}

// modifyBearer updates an active default bearer in place with a single MODIFY EPS
// BEARER CONTEXT REQUEST (TS 24.301 §6.4.2): a changed DNS server in the Protocol
// Configuration Options (TS 24.008 §10.5.6.3) and/or the per-APN Session-AMBR
// (§9.9.4.2). The new values are committed only when the UE accepts, so an aborted
// modification leaves the stored config stale for the backstop to retry. The
// Session-AMBR is also pushed to the UPF QER so the data plane enforces it.
func (m *MME) modifyBearer(ctx context.Context, ue *UeContext, p *PdnConnection, qos *EpsQoS, includeDNS, includeAMBR, includeQoS bool) {
	req := &eps.ModifyEPSBearerContextRequest{
		EPSBearerIdentity:            p.Ebi,
		ProcedureTransactionIdentity: 0,
	}

	if includeQoS {
		req.NewEPSQoS = eps.EPSQoS{QCI: qos.QCI}.Marshal()
	}

	var (
		dns      netip.Addr
		dnsValid bool
	)

	if includeDNS {
		var dnsServers [][]byte

		if parsed, err := netip.ParseAddr(qos.DNS); err == nil {
			dns, dnsValid = parsed, true

			if dns.Is4() {
				b := dns.As4()
				dnsServers = [][]byte{b[:]}
			} else {
				b := dns.As16()
				dnsServers = [][]byte{b[:]}
			}
		}

		var ipv4LinkMTU uint16
		if p.PdnType == eps.PDNTypeIPv4 || p.PdnType == eps.PDNTypeIPv4v6 {
			ipv4LinkMTU = qos.MTU
		}

		req.ProtocolConfigurationOptions = eps.BuildProtocolConfigurationOptions(dnsServers, ipv4LinkMTU)
	}

	if includeAMBR {
		// Update the UPF QER (the enforcement point) before signalling the AMBR, and
		// abort on failure: signalling anyway commits the new AMBR on UE-accept while
		// the UPF stays behind, and reconcile then sees no diff to retry.
		if err := m.Session.UpdateEPSSessionAMBR(ctx, ue.IMSI(), p.Ebi, qos.SessAmbrULStr, qos.SessAmbrDLStr); err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to update UPF Session-AMBR; deferring EPS bearer modification to the next reconcile",
				zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Error(err))

			return
		}

		req.APNAMBR = eps.APNAMBRFromBitsPerSecond(BitRateToBps(qos.SessAmbrDLStr), BitRateToBps(qos.SessAmbrULStr)).Marshal()
	}

	naspdu, err := ue.ProtectDownlinkMessage(req)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to protect Modify EPS Bearer Context Request", zap.Error(err))
		return
	}

	ue.mu.Lock()
	p.Modifying = true
	p.PendingDNConfig = qos.DnFingerprint()
	p.PendingSessAmbrDLBps = BitRateToBps(qos.SessAmbrDLStr)
	p.PendingSessAmbrULBps = BitRateToBps(qos.SessAmbrULStr)
	p.PendingQCI = qos.QCI
	p.PendingARP = qos.ARP

	if dnsValid {
		p.Dns = dns
	}
	ue.mu.Unlock()

	if includeQoS {
		// A QCI/ARP change reconfigures the radio bearer, so the NAS message is
		// piggybacked in an S1AP E-RAB Modify Request (TS 36.413 §8.2.2).
		m.sendERABModify(ctx, ue, p, qos, naspdu)
	} else {
		// DNS and/or Session-AMBR only: no radio change, so the NAS message is sent
		// standalone in a Downlink NAS Transport (TS 23.401 §5.4.3).
		ue.Conn().SendDownlinkNASTransport(ctx, naspdu)
	}

	m.ArmESMGuardAbortOnly(ue, p, "Modify EPS Bearer Context Request", naspdu, func() {
		ue.mu.Lock()
		if p := ue.defaultPDNLocked(); p != nil {
			ClearPendingModifyLocked(p)
		}
		ue.mu.Unlock()
	})
}

// sendERABModify reconfigures the UE's default-bearer radio QoS with an S1AP
// E-RAB MODIFY REQUEST (TS 36.413 §8.2.2): the new E-RAB-level QoS (QCI, ARP) for
// the eNB, carrying the MODIFY EPS BEARER CONTEXT REQUEST piggybacked in the
// NAS-PDU for the UE. Completion is the NAS Modify Accept, not the E-RAB Modify
// Response, so this does not block on it.
func (m *MME) sendERABModify(ctx context.Context, ue *UeContext, p *PdnConnection, qos *EpsQoS, naspdu []byte) {
	req := &s1ap.ERABModifyRequest{
		ERABToBeModified: []s1ap.ERABToBeModifiedItemBearerModReq{{
			ERABID: s1ap.ERABID(p.Ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(qos.QCI),
				ARP: s1ap.AllocationAndRetentionPriority{
					PriorityLevel:           qos.ARP,
					PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
					PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
				},
			},
			NASPDU: s1ap.NASPDU(naspdu),
		}},
	}

	if err := ue.Conn().SendERABModify(ctx, req); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to send E-RAB Modify Request", zap.Error(err))
		return
	}
}

// reactivateBearer asks the UE to re-establish its PDN connection by deactivating
// the default bearer with ESM cause #39 "reactivation requested" (TS 24.301
// §6.4.4.2). The request is guarded and retransmitted until the UE answers with
// DEACTIVATE EPS BEARER CONTEXT ACCEPT.
func (m *MME) reactivateBearer(ctx context.Context, ue *UeContext, p *PdnConnection) {
	m.DeactivateBearer(ctx, ue, p, eps.ESMCauseReactivationRequested, 0, false)
}
