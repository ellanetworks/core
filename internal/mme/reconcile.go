// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

var (
	ErrUENotReachable = errors.New("UE is not reachable for EPS bearer signalling")
	ErrBearerBusy     = errors.New("an ESM procedure is outstanding on the EPS bearer")
)

// ReconcileUE asks the SMF to re-evaluate every PDN connection of a UE against the
// current policy. Only a registered UE with an active S1 connection is signalled;
// an idle UE is signalled when it returns to ECM-CONNECTED (the ICS Response) or
// by the next backstop sweep.
func (m *MME) ReconcileUE(ctx context.Context, ue *UeContext) {
	if _, ready := m.ReconcileReady(ue); !ready {
		return
	}

	for _, p := range m.SnapshotPDNs(ue) {
		if err := m.Session.ReconcileSession(ctx, p.SessionRef); err != nil {
			logger.From(ctx, logger.MmeLog).Warn("session reconcile failed", zap.String("apn", p.Apn), zap.Error(err))
		}
	}
}

func (m *MME) ResumeBearerReconfigurationAfterHandover(ctx context.Context, ue *UeContext) {
	ue.mu.Lock()

	var interrupted []string

	for _, p := range ue.Pdns {
		if p.Modifying != nil {
			p.guard.Stop()
			p.clearModificationLocked()
			interrupted = append(interrupted, p.SessionRef)
		}
	}

	ue.mu.Unlock()

	for _, ref := range interrupted {
		m.Session.CommitEPSBearerModification(ctx, ref, false)
	}

	m.ReconcileUE(ctx, ue)
}

func (m *MME) ModifyEPSBearer(ctx context.Context, imsi string, ebi uint8, mod models.EPSBearerModification) error {
	if m.commitIdleARPChange(ctx, imsi, ebi, mod) {
		return nil
	}

	ue, ueConn, p, err := m.reconcilableBearer(ctx, imsi, ebi, func(p *PdnConnection) bool { return !arpOnly(p, mod) })
	if err != nil {
		return err
	}

	ctx = logger.Into(ctx, ueConn.LogFields()...)

	ueConn.Log(ctx).Info("policy/data-network changed; modifying EPS bearer in place", zap.String("apn", p.Apn),
		zap.Bool("dns_changed", mod.DNS.IsValid()), zap.Bool("session_ambr", mod.APNAMBR != nil), zap.Bool("qos", mod.QoS != nil))

	return m.modifyBearer(ctx, ue, ueConn, p, mod)
}

func (m *MME) ReactivateEPSBearer(ctx context.Context, imsi string, ebi uint8) error {
	ue, ueConn, p, err := m.reconcilableBearer(ctx, imsi, ebi, func(*PdnConnection) bool { return true })
	if err != nil {
		return err
	}

	ctx = logger.Into(ctx, ueConn.LogFields()...)

	if !ue.BearerReleaseOnly(p) {
		ueConn.Log(ctx).Info("policy/data-network changed on the last PDN connection; detaching for re-attach", zap.String("apn", p.Apn))
		m.sendNetworkDetach(ctx, ue, ueConn, eps.DetachTypeReattachRequired)

		return nil
	}

	ueConn.Log(ctx).Info("policy/data-network changed; reactivating EPS bearer", zap.String("apn", p.Apn))
	m.reactivateBearer(ctx, ue, p)

	return nil
}

func (m *MME) commitIdleARPChange(ctx context.Context, imsi string, ebi uint8, mod models.EPSBearerModification) bool {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok || ue.EMMState() != EMMRegistered || ue.Conn() != nil {
		return false
	}

	ue.mu.Lock()

	p := ue.Pdns[ebi]
	if p == nil || p.Deactivating || p.Modifying != nil || !arpOnly(p, mod) {
		ue.mu.Unlock()

		return false
	}

	p.Arp = mod.QoS.ARP
	ref := p.SessionRef
	ue.mu.Unlock()

	m.Session.CommitEPSBearerModification(ctx, ref, true)

	return true
}

func arpOnly(p *PdnConnection, mod models.EPSBearerModification) bool {
	return mod.QoS != nil && mod.QoS.QCI == p.Qci && mod.APNAMBR == nil && !mod.DNS.IsValid() && len(mod.MappedFiveGSQoS) == 0
}

// ue.active is freed concurrently by a release goroutine, and reconciliation is
// deferred while an S1 handover is in flight (an E-RAB Modify or Release would
// collide with the handover's bearer signalling, TS 36.413 §8.4.1.2); the next
// sweep re-converges the UE.
func (m *MME) reconcilableBearer(ctx context.Context, imsi string, ebi uint8, pageIfIdle func(*PdnConnection) bool) (*UeContext, *UeConn, *PdnConnection, error) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return nil, nil, nil, fmt.Errorf("no context for imsi %s", imsi)
	}

	p := m.LookupPDN(ue, ebi)
	if p == nil {
		return nil, nil, nil, fmt.Errorf("no EPS bearer %d for imsi %s", ebi, imsi)
	}

	ueConn, ready := m.ReconcileReady(ue)
	if !ready {
		if ue.EMMState() == EMMRegistered && ue.Conn() == nil && pageIfIdle(p) {
			m.pageForSignalling(ctx, ue)
		}

		return nil, nil, nil, ErrUENotReachable
	}

	if ueConn.ICS() != ICSCompleted {
		return nil, nil, nil, ErrUENotReachable
	}

	ue.mu.Lock()
	busy := p.Deactivating || p.Modifying != nil
	ue.mu.Unlock()

	if busy {
		return nil, nil, nil, ErrBearerBusy
	}

	return ue, ueConn, p, nil
}

func (m *MME) pageForSignalling(ctx context.Context, ue *UeContext) {
	arm := func() error { return ue.beginPaging(&MTRequest{Signalling: true}) }

	if err := m.page(ctx, ue, arm); err != nil && !errors.Is(err, errPagingSkipped) {
		logger.From(ctx, logger.MmeLog).Warn("could not page the UE for a bearer reconfiguration",
			logger.SUPI(ue.Supi().String()), zap.Error(err))
	}
}

// modifyBearer updates an active default bearer in place with a single MODIFY EPS
// BEARER CONTEXT REQUEST (TS 24.301 §6.4.2): a changed DNS server in the Protocol
// Configuration Options (TS 24.008 §10.5.6.3) and/or the per-APN Session-AMBR
// (§9.9.4.2). The new values are committed only when the UE accepts, so an aborted
// modification leaves the stored config stale for the backstop to retry.
func (m *MME) modifyBearer(ctx context.Context, ue *UeContext, ueConn *UeConn, p *PdnConnection, mod models.EPSBearerModification) error {
	req := &eps.ModifyEPSBearerContextRequest{
		EPSBearerIdentity: eps.EPSBearerIdentity(p.Ebi),
		PTI:               0,
	}

	if mod.QoS != nil {
		req.NewEPSQoS = &eps.EPSQoS{QCI: mod.QoS.QCI}
	}

	if mod.DNS.IsValid() || len(mod.MappedFiveGSQoS) > 0 {
		var (
			dnsServers  [][]byte
			ipv4LinkMTU uint16
		)

		if mod.DNS.IsValid() {
			dnsServers = nas.DNSServers(mod.DNS)

			if p.PdnType == eps.PDNTypeIPv4 || p.PdnType == eps.PDNTypeIPv4v6 {
				ipv4LinkMTU = mod.MTU
			}
		}

		pco := nas.NewProtocolConfigurationOptions(dnsServers, ipv4LinkMTU)
		pco.Containers = append(pco.Containers, mod.MappedFiveGSQoS...)

		// TS 24.301 §8.3.18.9 and §8.3.18.13
		if ue.UsesEPCO(p) {
			req.ExtendedProtocolConfigurationOptions = &pco
		} else {
			req.ProtocolConfigurationOptions = &pco
		}
	}

	if mod.APNAMBR != nil {
		apnAMBR, err := eps.APNAMBRFromKbps(mod.APNAMBR.Downlink.Bps()/1000, mod.APNAMBR.Uplink.Bps()/1000)
		if err != nil {
			return fmt.Errorf("encode APN-AMBR: %w", err)
		}

		req.APNAMBR = &apnAMBR
	}

	plain, err := req.MarshalBinary()
	if err != nil {
		return fmt.Errorf("build Modify EPS Bearer Context Request: %w", err)
	}

	ue.mu.Lock()

	if p.Deactivating || p.Modifying != nil {
		ue.mu.Unlock()

		return ErrBearerBusy
	}

	p.Modifying = &mod
	p.modifyAwaitingRadio = mod.QoS != nil
	ue.mu.Unlock()

	write := func(wire []byte) error {
		ueConn.SendDownlinkNASTransport(ctx, wire)

		return nil
	}

	if mod.QoS != nil {
		// A QCI/ARP change reconfigures the radio bearer, so the NAS message is
		// piggybacked in an S1AP E-RAB Modify Request (TS 36.413 §8.2.2).
		write = func(wire []byte) error {
			m.sendERABModify(ctx, ueConn, p, *mod.QoS, wire)

			return nil
		}
	}

	m.ArmESMGuardAbortOnly(ctx, ue, p, "Modify EPS Bearer Context Request", plain, eps.SHTIntegrityProtectedCiphered, func(ctx context.Context) {
		m.ConcludeBearerModification(ctx, ue, p, false)
	})

	if err := ueConn.SendProtected(plain, eps.SHTIntegrityProtectedCiphered, write); err != nil {
		m.StopESMGuard(p)

		ue.mu.Lock()
		p.clearModificationLocked()
		ue.mu.Unlock()

		ReportProtectFailure(ctx, ueConn, "Modify EPS Bearer Context Request", err)

		return err
	}

	return nil
}

// sendERABModify reconfigures the UE's default-bearer radio QoS with an S1AP
// E-RAB MODIFY REQUEST (TS 36.413 §8.2.2): the new E-RAB-level QoS (QCI, ARP) for
// the eNB, carrying the MODIFY EPS BEARER CONTEXT REQUEST piggybacked in the
// NAS-PDU for the UE. Completion is the NAS Modify Accept, not the E-RAB Modify
// Response, so this does not block on it.
func (m *MME) sendERABModify(ctx context.Context, ueConn *UeConn, p *PdnConnection, qos models.EPSBearerQoS, naspdu []byte) {
	req := &s1ap.ERABModifyRequest{
		ERABToBeModified: []s1ap.ERABToBeModifiedItemBearerModReq{{
			ERABID: s1ap.ERABID(p.Ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(qos.QCI),
				ARP: BearerARP(qos.ARP),
			},
			NASPDU: s1ap.NASPDU(naspdu),
		}},
	}

	if err := ueConn.SendERABModify(ctx, req); err != nil {
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
