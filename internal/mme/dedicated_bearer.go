// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"

	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var (
	ErrNoFreeEBI           = errors.New("no EPS bearer identity is free")
	ErrNoDedicatedBearer   = errors.New("no such dedicated EPS bearer")
	ErrLinkedBearerUnknown = errors.New("the linked EPS bearer is not the session's default bearer")
)

type DedicatedBearerInfo struct {
	Ebi       uint8
	SgwFTEID  models.FTEID
	SgwN3IPv6 netip.Addr
	EnbFTEID  models.FTEID
	QCI       uint8
	ARP       models.Arp
	MBR       models.Ambr
	GBR       models.Ambr
	Filters   []models.SDFFilter

	Activating   bool
	Deactivating bool
}

type DedicatedBearer struct {
	DedicatedBearerInfo

	accepted          bool
	radioUp           bool
	deactivatePending bool
	modifying         *dedicatedModification
	mappedFiveGSQoS   []nas.PCOContainer

	guard guard.Guard
}

func (ue *UeContext) ebiInUseLocked(ebi uint8) bool {
	if _, ok := ue.Pdns[ebi]; ok {
		return true
	}

	_, b := ue.dedicatedLocked(ebi)

	return b != nil
}

func (ue *UeContext) dedicatedLocked(ebi uint8) (*PdnConnection, *DedicatedBearer) {
	for _, p := range ue.Pdns {
		if b, ok := p.Dedicated[ebi]; ok {
			return p, b
		}
	}

	return nil, nil
}

func (m *MME) LookupDedicated(ue *UeContext, ebi uint8) (*PdnConnection, *DedicatedBearer) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.dedicatedLocked(ebi)
}

func (m *MME) DedicatedActivating(ue *UeContext, ebi uint8) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	_, b := ue.dedicatedLocked(ebi)

	return b != nil && b.Activating
}

func (m *MME) SnapshotDedicated(ue *UeContext) []DedicatedBearerInfo {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	var out []DedicatedBearerInfo

	for _, p := range ue.Pdns {
		for _, ebi := range slices.Sorted(maps.Keys(p.Dedicated)) {
			out = append(out, p.Dedicated[ebi].DedicatedBearerInfo)
		}
	}

	return out
}

func DedicatedBearerARP(arp models.Arp) s1ap.AllocationAndRetentionPriority {
	a := s1ap.AllocationAndRetentionPriority{
		PriorityLevel:           uint8(arp.PriorityLevel),
		PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
		PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
	}

	if arp.PreemptCap == models.PreemptionCapabilityMayPreempt {
		a.PreemptionCapability = s1ap.PreemptionMayTrigger
	}

	if arp.PreemptVuln == models.PreemptionVulnerabilityPreemptable {
		a.PreemptionVulnerability = s1ap.PreemptionPreemptable
	}

	return a
}

func DedicatedERABQoS(b *DedicatedBearerInfo) s1ap.ERABLevelQoSParameters {
	gbr := &s1ap.GBRQosInformation{
		MaximumBitrateDL:    s1ap.BitRate(b.MBR.Downlink.Bps()),
		MaximumBitrateUL:    s1ap.BitRate(b.MBR.Uplink.Bps()),
		GuaranteedBitrateDL: s1ap.BitRate(b.GBR.Downlink.Bps()),
		GuaranteedBitrateUL: s1ap.BitRate(b.GBR.Uplink.Bps()),
	}

	if qos, err := dedicatedEPSQoS(b); err == nil {
		if r, ok := qos.GBRBitRates(); ok {
			gbr = &s1ap.GBRQosInformation{
				MaximumBitrateDL:    s1ap.BitRate(r.MaxDownlinkKbps * 1000),
				MaximumBitrateUL:    s1ap.BitRate(r.MaxUplinkKbps * 1000),
				GuaranteedBitrateDL: s1ap.BitRate(r.GuaranteedDownlinkKbps * 1000),
				GuaranteedBitrateUL: s1ap.BitRate(r.GuaranteedUplinkKbps * 1000),
			}
		}
	}

	return s1ap.ERABLevelQoSParameters{QCI: s1ap.QCI(b.QCI), ARP: DedicatedBearerARP(b.ARP), GBR: gbr}
}

func dedicatedEPSQoS(b *DedicatedBearerInfo) (eps.EPSQoS, error) {
	return eps.GBREPSQoS(b.QCI, eps.EPSQoSBitRates{
		MaxUplinkKbps:          b.MBR.Uplink.Kbps(),
		MaxDownlinkKbps:        b.MBR.Downlink.Kbps(),
		GuaranteedUplinkKbps:   b.GBR.Uplink.Kbps(),
		GuaranteedDownlinkKbps: b.GBR.Downlink.Kbps(),
	})
}

func dedicatedActivationRequest(linked uint8, b *DedicatedBearer, useEPCO bool) ([]byte, error) {
	qos, err := dedicatedEPSQoS(&b.DedicatedBearerInfo)
	if err != nil {
		return nil, err
	}

	req := &eps.ActivateDedicatedEPSBearerContextRequest{
		EPSBearerIdentity:       eps.EPSBearerIdentity(b.Ebi),
		LinkedEPSBearerIdentity: eps.EPSBearerIdentity(linked),
		EPSQoS:                  qos,
		TFT:                     models.EPSTFT(eps.TFTCreate, b.Filters),
	}

	req.ProtocolConfigurationOptions, req.ExtendedProtocolConfigurationOptions = mappedFiveGSQoSOptions(b.mappedFiveGSQoS, useEPCO)

	return req.MarshalBinary()
}

func mappedFiveGSQoSOptions(containers []nas.PCOContainer, useEPCO bool) (*nas.ProtocolConfigurationOptions, *nas.ProtocolConfigurationOptions) {
	if len(containers) == 0 {
		return nil, nil
	}

	pco := nas.ProtocolConfigurationOptions{ConfigProtocol: nas.PCOConfigProtocolPPP, Direction: nas.PCONetworkToMS, Containers: slices.Clone(containers)}

	switch {
	case useEPCO:
		return nil, &pco
	case pco.FitsUnextended():
		return &pco, nil
	default:
		logger.MmeLog.Warn("the 5GS QoS of a dedicated bearer does not fit the protocol configuration options of a UE without ePCO; the bearer stays in EPS")
		return nil, nil
	}
}

func (m *MME) ActivateDedicatedBearer(ctx context.Context, imsi string, req models.DedicatedBearerRequest) error {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return fmt.Errorf("no context for imsi %s", imsi)
	}

	p := m.LookupPDN(ue, req.LinkedEBI)
	if p == nil || p.SessionRef != req.SessionRef {
		return ErrLinkedBearerUnknown
	}

	ueConn, ready := m.ReconcileReady(ue)
	if !ready {
		if ue.EMMState() == EMMRegistered && ue.Conn() == nil {
			m.pageForSignalling(ctx, ue)
		}

		return ErrUENotReachable
	}

	if ueConn.ICS() != ICSCompleted {
		return ErrUENotReachable
	}

	ue.mu.Lock()

	if ue.Pdns[p.Ebi] != p {
		ue.mu.Unlock()
		return ErrLinkedBearerUnknown
	}

	if p.Deactivating {
		ue.mu.Unlock()
		return ErrBearerBusy
	}

	ebi := ue.allocateEBI()
	if ebi == 0 {
		ue.mu.Unlock()
		return ErrNoFreeEBI
	}

	b := &DedicatedBearer{DedicatedBearerInfo: DedicatedBearerInfo{
		Ebi: ebi, SgwFTEID: req.SGW, SgwN3IPv6: req.SGWN3IPv6,
		QCI: req.QCI, ARP: req.ARP, MBR: req.MBR, GBR: req.GBR, Filters: req.Filters,
		Activating: true,
	}, mappedFiveGSQoS: req.MappedFiveGSQoS}

	plain, setup, err := dedicatedActivation(p.Ebi, b, ue.ueNetCap.SupportsEPCO())
	if err != nil {
		ue.mu.Unlock()
		return err
	}

	if !m.armDedicatedGuardLocked(ctx, ue, b, "Activate Dedicated EPS Bearer Context Request", plain, func(ctx context.Context) {
		m.failDedicated(ctx, ue, b, "T3485 expired")
	}) {
		ue.mu.Unlock()
		return ErrUENotReachable
	}

	if p.Dedicated == nil {
		p.Dedicated = make(map[uint8]*DedicatedBearer)
	}

	p.Dedicated[ebi] = b
	ue.mu.Unlock()

	if err := sendDedicatedActivation(ctx, ueConn, plain, setup); err != nil {
		m.dropDedicated(ue, p, b)
		return err
	}

	ueConn.Log(ctx).Info("activating dedicated EPS bearer", zap.String("apn", p.Apn), logger.ERABID(ebi), zap.Uint8("qci", b.QCI))

	return nil
}

func dedicatedActivation(linked uint8, b *DedicatedBearer, useEPCO bool) ([]byte, *s1ap.ERABSetupRequest, error) {
	plain, err := dedicatedActivationRequest(linked, b, useEPCO)
	if err != nil {
		return nil, nil, fmt.Errorf("build Activate Dedicated EPS Bearer Context Request: %w", err)
	}

	sgwTLA, err := models.EncodeTransportLayerAddress(b.SgwFTEID.Addr, b.SgwN3IPv6)
	if err != nil {
		return nil, nil, fmt.Errorf("encode the S-GW transport layer address: %w", err)
	}

	return plain, &s1ap.ERABSetupRequest{
		ERABToBeSetup: []s1ap.ERABToBeSetupItemBearerSUReq{{
			ERABID:                s1ap.ERABID(b.Ebi),
			QoS:                   DedicatedERABQoS(&b.DedicatedBearerInfo),
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(b.SgwFTEID.TEID),
		}},
	}, nil
}

func sendDedicatedActivation(ctx context.Context, ueConn *UeConn, plain []byte, setup *s1ap.ERABSetupRequest) error {
	var writeErr error

	if err := ueConn.SendProtected(plain, eps.SHTIntegrityProtectedCiphered, func(wire []byte) error {
		setup.ERABToBeSetup[0].NASPDU = s1ap.NASPDU(wire)
		writeErr = ueConn.SendERABSetup(ctx, setup)

		return writeErr
	}); err != nil {
		if writeErr == nil {
			ReportProtectFailure(ctx, ueConn, "Activate Dedicated EPS Bearer Context Request", err)
		}

		return err
	}

	return nil
}

func (m *MME) armDedicatedGuardLocked(ctx context.Context, ue *UeContext, b *DedicatedBearer, name string, plain []byte, onAbort func(context.Context)) bool {
	if ue.Conn() == nil {
		return false
	}

	link := trace.SpanContextFromContext(ctx)

	b.guard.ArmWith(
		m.esmGuardCfg,
		func(attempt int32) {
			if c := ue.Conn(); c != nil {
				c.retransmitNASGuard(link, ue, name, plain, eps.SHTIntegrityProtectedCiphered, attempt)
			}
		},
		func() {
			if c := ue.Conn(); c != nil {
				c.expireNASGuard(link, ue, name, onAbort)
			}
		},
	)

	return true
}

func (m *MME) dropDedicated(ue *UeContext, p *PdnConnection, b *DedicatedBearer) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	b.guard.Stop()

	if p.Dedicated[b.Ebi] != b {
		return false
	}

	delete(p.Dedicated, b.Ebi)

	return true
}

func (m *MME) DedicatedBearerAccepted(ctx context.Context, ue *UeContext, ebi uint8) bool {
	ue.mu.Lock()

	p, b := ue.dedicatedLocked(ebi)
	if b == nil || !b.Activating {
		ue.mu.Unlock()
		return b != nil
	}

	b.accepted = true

	if b.radioUp {
		b.guard.Stop()
	} else {
		b.guard.ArmOnce(m.esmGuardCfg.ExpireTime, func() {
			m.failDedicated(context.Background(), ue, b, "no E-RAB Setup Response")
		})
	}
	ue.mu.Unlock()

	if !m.completeDedicated(ctx, ue, p, b) {
		m.DeactivateDedicated(ctx, ue, ebi)
	}

	return true
}

func (m *MME) dedicatedRadioUp(ctx context.Context, ue *UeContext, p *PdnConnection, b *DedicatedBearer, enb models.FTEID, radioReleasedOnRefusal bool) bool {
	ue.mu.Lock()
	b.EnbFTEID = enb
	b.radioUp = true
	activating := b.Activating
	ue.mu.Unlock()

	if activating {
		if m.completeDedicated(ctx, ue, p, b) {
			return true
		}
	} else if err := m.Session.DedicatedBearerMoved(ctx, p.SessionRef, b.SgwFTEID.TEID, enb); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("SMF refused the dedicated bearer's radio endpoint; releasing it", logger.SUPI(ue.Supi().String()), logger.ERABID(b.Ebi), zap.Error(err))
	} else {
		return true
	}

	ue.mu.Lock()
	b.deactivatePending = true

	if radioReleasedOnRefusal {
		b.EnbFTEID = models.FTEID{}
		b.radioUp = false
	}
	ue.mu.Unlock()

	return false
}

func (m *MME) DeactivatePendingDedicated(ctx context.Context, ue *UeContext) {
	var ebis []uint8

	ue.mu.Lock()

	for _, p := range ue.Pdns {
		for ebi, b := range p.Dedicated {
			if !b.deactivatePending {
				continue
			}

			b.deactivatePending = false

			ebis = append(ebis, ebi)
		}
	}
	ue.mu.Unlock()

	for _, ebi := range ebis {
		m.DeactivateDedicated(ctx, ue, ebi)
	}
}

func (m *MME) completeDedicated(ctx context.Context, ue *UeContext, p *PdnConnection, b *DedicatedBearer) bool {
	ue.mu.Lock()

	if !b.Activating || !b.accepted || !b.radioUp || p.Dedicated[b.Ebi] != b {
		ue.mu.Unlock()
		return true
	}

	b.Activating = false
	b.guard.Stop()
	ref, teid, ebi, enb := p.SessionRef, b.SgwFTEID.TEID, b.Ebi, b.EnbFTEID
	ue.mu.Unlock()

	logger.From(ctx, logger.MmeLog).Info("dedicated EPS bearer active", logger.SUPI(ue.Supi().String()), logger.ERABID(ebi), zap.Uint8("qci", b.QCI))

	if err := m.Session.DedicatedBearerActivated(ctx, ref, teid, ebi, enb); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("SMF refused the dedicated bearer; releasing it", logger.SUPI(ue.Supi().String()), logger.ERABID(ebi), zap.Error(err))
		return false
	}

	return true
}

func (m *MME) FailDedicatedBearer(ctx context.Context, ue *UeContext, ebi uint8, reason string) {
	if _, b := m.LookupDedicated(ue, ebi); b != nil {
		m.failDedicated(ctx, ue, b, reason)
	}
}

func (m *MME) failDedicated(ctx context.Context, ue *UeContext, b *DedicatedBearer, reason string) {
	ue.mu.Lock()

	p, held := ue.dedicatedLocked(b.Ebi)
	if held != b {
		ue.mu.Unlock()
		return
	}

	b.guard.Stop()
	delete(p.Dedicated, b.Ebi)

	activating, radioUp, accepted := b.Activating, b.radioUp, b.accepted
	if !activating {
		ue.localBearerDeactivation = true
	}

	ref, teid, ebi := p.SessionRef, b.SgwFTEID.TEID, b.Ebi
	ue.mu.Unlock()

	logger.From(ctx, logger.MmeLog).Info("dedicated EPS bearer failed", logger.SUPI(ue.Supi().String()), logger.ERABID(ebi), zap.String("reason", reason))

	if ueConn := ue.Conn(); ueConn != nil && activating {
		if accepted && !radioUp {
			m.sendDedicatedDeactivation(ctx, ueConn, ebi)
		} else if radioUp {
			m.sendDedicatedERABRelease(ctx, ueConn, ebi, nil)
		}
	}

	m.Session.DedicatedBearerReleased(ctx, ref, teid)
}

func (m *MME) DedicatedReleasedByRAN(ctx context.Context, ue *UeContext, ebi uint8) bool {
	ue.mu.Lock()

	p, b := ue.dedicatedLocked(ebi)
	if b == nil {
		ue.mu.Unlock()
		return false
	}

	b.guard.Stop()
	delete(p.Dedicated, ebi)

	ref, teid := p.SessionRef, b.SgwFTEID.TEID
	ue.mu.Unlock()

	logger.From(ctx, logger.MmeLog).Info("eNB released the dedicated EPS bearer", logger.SUPI(ue.Supi().String()), logger.ERABID(ebi))

	m.Session.DedicatedBearerReleased(ctx, ref, teid)

	return true
}

func (m *MME) DedicatedESMStatus(ctx context.Context, ue *UeContext, ebi uint8, cause eps.ESMCause) bool {
	_, b := m.LookupDedicated(ue, ebi)
	if b == nil {
		return false
	}

	ue.mu.Lock()
	deactivating, activating, radioUp, modifying := b.Deactivating, b.Activating, b.radioUp, b.modifying
	ue.mu.Unlock()

	switch {
	case deactivating:
		m.releaseDedicated(ctx, ue, b, false)
	case cause == eps.ESMCauseInvalidEPSBearerIdentity:
		if ueConn := ue.Conn(); ueConn != nil && radioUp {
			m.sendDedicatedERABRelease(ctx, ueConn, ebi, nil)
		}

		m.releaseDedicated(ctx, ue, b, true)
	case activating:
		m.failDedicated(ctx, ue, b, "ESM STATUS "+cause.String())
	case modifying != nil:
		m.dedicatedUEAnswered(ctx, ue, b, modifying, false, "ESM STATUS "+cause.String())
	}

	return true
}

func (m *MME) DeactivateDedicatedBearer(ctx context.Context, imsi string, ebi uint8, sgwTEID uint32) error {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return fmt.Errorf("no context for imsi %s", imsi)
	}

	if _, b := m.LookupDedicated(ue, ebi); b == nil || b.SgwFTEID.TEID != sgwTEID {
		return ErrNoDedicatedBearer
	}

	m.DeactivateDedicated(ctx, ue, ebi)

	return nil
}

func (m *MME) DeactivateDedicated(ctx context.Context, ue *UeContext, ebi uint8) {
	ueConn, ready := m.ReconcileReady(ue)
	if ready && ueConn.ICS() != ICSCompleted {
		ready = false
	}

	if !ready && m.handoverInProgress(ue) {
		ue.mu.Lock()
		if _, b := ue.dedicatedLocked(ebi); b != nil && !b.Deactivating {
			b.deactivatePending = true
		}
		ue.mu.Unlock()

		return
	}

	plain, err := (&eps.DeactivateEPSBearerContextRequest{
		EPSBearerIdentity: eps.EPSBearerIdentity(ebi),
		Cause:             eps.ESMCauseRegularDeactivation,
	}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Deactivate EPS Bearer Context Request", zap.Error(err))
		m.ReleaseDedicated(ctx, ue, ebi)

		return
	}

	ue.mu.Lock()

	p, b := ue.dedicatedLocked(ebi)
	if b == nil || b.Deactivating {
		ue.mu.Unlock()
		return
	}

	if !ready || (b.Activating && !b.accepted) {
		b.guard.Stop()
		delete(p.Dedicated, ebi)

		ue.localBearerDeactivation = true
		ref, teid, radioUp := p.SessionRef, b.SgwFTEID.TEID, b.radioUp
		ue.mu.Unlock()

		if ueConn := ue.Conn(); ueConn != nil && radioUp && ueConn.ICS() == ICSCompleted {
			m.sendDedicatedERABRelease(ctx, ueConn, ebi, nil)
		}

		m.Session.DedicatedBearerReleased(ctx, ref, teid)

		return
	}

	enbHolds := b.radioUp || b.Activating
	b.Deactivating = true
	b.Activating = false
	b.modifying = nil

	armed := m.armDedicatedGuardLocked(ctx, ue, b, "Deactivate EPS Bearer Context Request", plain, func(ctx context.Context) {
		m.releaseDedicated(ctx, ue, b, true)
	})
	ue.mu.Unlock()

	if !armed {
		m.releaseDedicated(ctx, ue, b, true)
		return
	}

	if err := ueConn.SendProtected(plain, eps.SHTIntegrityProtectedCiphered, func(wire []byte) error {
		if !enbHolds {
			ueConn.SendDownlinkNASTransport(ctx, wire)
			return nil
		}

		m.sendDedicatedERABRelease(ctx, ueConn, ebi, wire)

		return nil
	}); err != nil {
		ReportProtectFailure(ctx, ueConn, "Deactivate EPS Bearer Context Request", err)
		m.ReleaseDedicated(ctx, ue, ebi)

		return
	}

	ueConn.Log(ctx).Info("deactivating dedicated EPS bearer", logger.ERABID(ebi))
}

func (m *MME) ReleaseDedicated(ctx context.Context, ue *UeContext, ebi uint8) bool {
	_, b := m.LookupDedicated(ue, ebi)
	if b == nil {
		return false
	}

	return m.releaseDedicated(ctx, ue, b, false)
}

func (m *MME) releaseDedicated(ctx context.Context, ue *UeContext, b *DedicatedBearer, local bool) bool {
	ue.mu.Lock()

	p, held := ue.dedicatedLocked(b.Ebi)
	if held != b {
		ue.mu.Unlock()
		return false
	}

	b.guard.Stop()
	delete(p.Dedicated, b.Ebi)

	if local {
		ue.localBearerDeactivation = true
	}

	ref, teid := p.SessionRef, b.SgwFTEID.TEID
	ue.mu.Unlock()

	m.Session.DedicatedBearerReleased(ctx, ref, teid)

	return true
}

func (p *PdnConnection) takeDedicatedLocked() {
	for _, b := range p.Dedicated {
		b.guard.Stop()
	}

	p.Dedicated = nil
}

func (ue *UeContext) DedicatedEBIs(p *PdnConnection) []uint8 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return slices.Sorted(maps.Keys(p.Dedicated))
}

func (m *MME) sendDedicatedDeactivation(ctx context.Context, ueConn *UeConn, ebi uint8) {
	plain, err := (&eps.DeactivateEPSBearerContextRequest{
		EPSBearerIdentity: eps.EPSBearerIdentity(ebi),
		Cause:             eps.ESMCauseRegularDeactivation,
	}).MarshalBinary()
	if err != nil {
		return
	}

	if err := ueConn.SendProtected(plain, eps.SHTIntegrityProtectedCiphered, func(wire []byte) error {
		ueConn.SendDownlinkNASTransport(ctx, wire)
		return nil
	}); err != nil {
		ReportProtectFailure(ctx, ueConn, "Deactivate EPS Bearer Context Request", err)
	}
}

func (m *MME) sendDedicatedERABRelease(ctx context.Context, ueConn *UeConn, ebi uint8, naspdu []byte) {
	cmd := &s1ap.ERABReleaseCommand{
		ERABToBeReleased: []s1ap.ERABItem{{ERABID: s1ap.ERABID(ebi), Cause: CauseNASNormalRelease}},
		NASPDU:           s1ap.NASPDU(naspdu),
	}

	if err := ueConn.SendERABRelease(ctx, cmd); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to send E-RAB Release Command", logger.ERABID(ebi), zap.Error(err))
	}
}

func preservesGBRBearers(cause *s1ap.Cause) bool {
	if cause == nil || cause.Group != s1ap.CauseGroupRadioNetwork || cause.Extended {
		return false
	}

	switch cause.Value {
	case s1ap.CauseRadioNetworkUserInactivity, s1ap.CauseRadioNetworkInterRATRedirection, s1ap.CauseRadioNetworkCSFallbackTriggered:
		return true
	default:
		return false
	}
}

func (ue *UeContext) takeDedicatedOnReleaseLocked(releaseGBR bool) (lost, interrupted []dedicatedLoss) {
	for _, p := range ue.Pdns {
		for ebi, b := range p.Dedicated {
			hadRadio := b.radioUp
			b.EnbFTEID = models.FTEID{}
			b.radioUp = false

			drop := b.Activating || b.Deactivating || releaseGBR && hadRadio && models.GBRQCI(b.QCI)

			if pending := b.modifying; pending != nil {
				b.modifying = nil
				b.guard.Stop()

				accepted := pending.ue == ueAccepted
				if accepted {
					b.DedicatedBearerInfo = pending.target
				}

				if !drop && !pending.realign {
					interrupted = append(interrupted, dedicatedLoss{ref: p.SessionRef, teid: b.SgwFTEID.TEID, accepted: accepted})
				}
			}

			if !drop {
				continue
			}

			b.guard.Stop()
			delete(p.Dedicated, ebi)
			lost = append(lost, dedicatedLoss{ref: p.SessionRef, teid: b.SgwFTEID.TEID})
		}
	}

	if len(lost) > 0 {
		ue.localBearerDeactivation = true
	}

	return lost, interrupted
}

type dedicatedLoss struct {
	ref      string
	teid     uint32
	accepted bool
}

func (m *MME) reportDedicatedLosses(lost, interrupted []dedicatedLoss) {
	if len(lost) == 0 && len(interrupted) == 0 {
		return
	}

	go func() {
		for _, l := range lost {
			m.Session.DedicatedBearerReleased(context.Background(), l.ref, l.teid)
		}

		for _, l := range interrupted {
			m.Session.DedicatedBearerModified(context.Background(), l.ref, l.teid, l.accepted)
		}
	}()
}
