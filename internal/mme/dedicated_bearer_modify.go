// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

var ErrInvalidModification = errors.New("the modification leaves the dedicated bearer without a valid TFT")

type radioOutcome uint8

const (
	radioNotNeeded radioOutcome = iota
	radioPending
	radioModified
	radioNotModified
)

type ueOutcome uint8

const (
	uePending ueOutcome = iota
	ueAccepted
	ueRejected
)

type dedicatedModification struct {
	mod     models.DedicatedBearerModification
	target  DedicatedBearerInfo
	radio   radioOutcome
	ue      ueOutcome
	reason  string
	realign bool
}

type modificationVerdict uint8

const (
	verdictPending modificationVerdict = iota
	verdictCommitted
	verdictFailed
	verdictRadioAhead
	verdictUEAhead
)

func (d *dedicatedModification) verdict() modificationVerdict {
	switch {
	case d.ue == ueAccepted && (d.radio == radioNotNeeded || d.radio == radioModified):
		return verdictCommitted
	case d.ue == ueAccepted && d.radio == radioNotModified:
		return verdictUEAhead
	case d.ue == ueRejected && d.radio == radioModified:
		return verdictRadioAhead
	case d.ue == ueRejected && d.radio != radioPending:
		return verdictFailed
	case d.ue == uePending && d.radio == radioNotModified:
		return verdictFailed
	default:
		return verdictPending
	}
}

func (d *dedicatedModification) awaitsRadio() bool {
	return d.ue != uePending && d.radio == radioPending
}

func modificationTFT(mod models.DedicatedBearerModification) (eps.TrafficFlowTemplate, bool) {
	switch mod.Operation {
	case models.TFTAddFilters:
		return models.EPSTFT(eps.TFTAddFilters, mod.Filters), true
	case models.TFTReplaceFilters:
		return models.EPSTFT(eps.TFTReplaceFilters, mod.Filters), true
	case models.TFTDeleteFilters:
		return eps.TrafficFlowTemplate{Operation: eps.TFTDeleteFilters, DeleteIdentifiers: slices.Clone(mod.DeleteIDs)}, true
	default:
		return eps.TrafficFlowTemplate{}, false
	}
}

func modifiedInfo(info DedicatedBearerInfo, mod models.DedicatedBearerModification) DedicatedBearerInfo {
	if mod.QoSChanged {
		info.MBR, info.GBR = mod.MBR, mod.GBR
	}

	if mod.ARP != nil {
		info.ARP = *mod.ARP
	}

	filters := slices.Clone(info.Filters)

	switch mod.Operation {
	case models.TFTAddFilters, models.TFTReplaceFilters:
		for _, f := range mod.Filters {
			filters = slices.DeleteFunc(filters, func(old models.SDFFilter) bool { return old.ID == f.ID })
			filters = append(filters, f)
		}
	case models.TFTDeleteFilters:
		filters = slices.DeleteFunc(filters, func(old models.SDFFilter) bool { return slices.Contains(mod.DeleteIDs, old.ID) })
	}

	info.Filters = filters

	return info
}

func checkModification(target DedicatedBearerInfo, mod models.DedicatedBearerModification) error {
	if !radioChange(mod) && mod.Operation == models.TFTNoChange {
		return fmt.Errorf("%w: it changes nothing", ErrInvalidModification)
	}

	if len(target.Filters) == 0 {
		return fmt.Errorf("%w: it empties the TFT (TS 24.301 §6.4.3.4, #41)", ErrInvalidModification)
	}

	if !slices.ContainsFunc(target.Filters, func(f models.SDFFilter) bool { return f.Direction != models.FilterDownlink }) {
		return fmt.Errorf("%w: no packet filter applies to the uplink (TS 24.301 §6.4.3.4, #44)", ErrInvalidModification)
	}

	return nil
}

func radioChange(mod models.DedicatedBearerModification) bool {
	return mod.QoSChanged || mod.ARP != nil
}

func arpOnlyModification(mod models.DedicatedBearerModification) bool {
	return mod.ARP != nil && !mod.QoSChanged && mod.Operation == models.TFTNoChange
}

func dedicatedModifyRequest(ebi uint8, target DedicatedBearerInfo, mod models.DedicatedBearerModification, useEPCO bool) ([]byte, error) {
	req := &eps.ModifyEPSBearerContextRequest{EPSBearerIdentity: eps.EPSBearerIdentity(ebi)}

	req.ProtocolConfigurationOptions, req.ExtendedProtocolConfigurationOptions = mappedFiveGSQoSOptions(mod.MappedFiveGSQoS, useEPCO)

	if radioChange(mod) {
		qos, err := dedicatedEPSQoS(&target)
		if err != nil {
			return nil, err
		}

		req.NewEPSQoS = &qos
	}

	if tft, ok := modificationTFT(mod); ok {
		req.TFT = &tft
	}

	return req.MarshalBinary()
}

func (m *MME) ModifyDedicatedBearer(ctx context.Context, imsi string, ebi uint8, mod models.DedicatedBearerModification) error {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return fmt.Errorf("no context for imsi %s", imsi)
	}

	if arpOnlyModification(mod) && m.commitIdleDedicatedARP(ctx, ue, ebi, mod) {
		return nil
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

	return m.startModification(ctx, ue, ueConn, ebi, mod, false)
}

func (m *MME) startModification(ctx context.Context, ue *UeContext, ueConn *UeConn, ebi uint8, mod models.DedicatedBearerModification, realign bool) error {
	if ueConn == nil {
		return ErrUENotReachable
	}

	ue.mu.Lock()

	p, b := ue.dedicatedLocked(ebi)
	if b == nil || p.SessionRef != mod.SessionRef || b.SgwFTEID.TEID != mod.SGWTEID {
		ue.mu.Unlock()
		return ErrNoDedicatedBearer
	}

	if b.Activating || b.Deactivating || b.modifying != nil {
		ue.mu.Unlock()
		return ErrBearerBusy
	}

	target := modifiedInfo(b.DedicatedBearerInfo, mod)
	if err := checkModification(target, mod); err != nil {
		ue.mu.Unlock()
		return err
	}

	plain, err := dedicatedModifyRequest(ebi, target, mod, ue.ueNetCap.SupportsEPCO())
	if err != nil {
		ue.mu.Unlock()
		return fmt.Errorf("build Modify EPS Bearer Context Request: %w", err)
	}

	pending := &dedicatedModification{mod: mod, target: target, realign: realign}
	if radioChange(mod) {
		pending.radio = radioPending
	}

	if !m.armDedicatedGuardLocked(ctx, ue, b, "Modify EPS Bearer Context Request", plain, func(ctx context.Context) {
		m.dedicatedUEAnswered(ctx, ue, b, pending, false, "T3486 expired")
	}) {
		ue.mu.Unlock()
		return ErrUENotReachable
	}

	b.modifying = pending
	ue.mu.Unlock()

	write := func(wire []byte) error {
		ueConn.SendDownlinkNASTransport(ctx, wire)
		return nil
	}

	if radioChange(mod) {
		write = func(wire []byte) error {
			return ueConn.SendERABModify(ctx, &s1ap.ERABModifyRequest{
				ERABToBeModified: []s1ap.ERABToBeModifiedItemBearerModReq{{
					ERABID: s1ap.ERABID(ebi),
					QoS:    DedicatedERABQoS(&target),
					NASPDU: s1ap.NASPDU(wire),
				}},
			})
		}
	}

	if err := ueConn.SendProtected(plain, eps.SHTIntegrityProtectedCiphered, write); err != nil {
		ue.mu.Lock()
		if b.modifying == pending {
			b.modifying = nil
			b.guard.Stop()
		}
		ue.mu.Unlock()

		ReportProtectFailure(ctx, ueConn, "Modify EPS Bearer Context Request", err)

		return err
	}

	ueConn.Log(ctx).Info("modifying dedicated EPS bearer", logger.ERABID(ebi), zap.Bool("qos", mod.QoSChanged), zap.Uint8("tft_operation", uint8(mod.Operation)))

	return nil
}

func (m *MME) commitIdleDedicatedARP(ctx context.Context, ue *UeContext, ebi uint8, mod models.DedicatedBearerModification) bool {
	if ue.EMMState() != EMMRegistered || ue.Conn() != nil {
		return false
	}

	ue.mu.Lock()

	p, b := ue.dedicatedLocked(ebi)
	if b == nil || p.SessionRef != mod.SessionRef || b.SgwFTEID.TEID != mod.SGWTEID || b.Activating || b.Deactivating || b.modifying != nil {
		ue.mu.Unlock()
		return false
	}

	b.ARP = *mod.ARP
	ref, teid := p.SessionRef, b.SgwFTEID.TEID
	ue.mu.Unlock()

	m.Session.DedicatedBearerModified(ctx, ref, teid, true)

	return true
}

func (m *MME) pendingModification(ue *UeContext, ebi uint8) (*DedicatedBearer, *dedicatedModification) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	_, b := ue.dedicatedLocked(ebi)
	if b == nil {
		return nil, nil
	}

	return b, b.modifying
}

func (m *MME) DedicatedBearerModifyAccepted(ctx context.Context, ue *UeContext, ebi uint8) bool {
	b, pending := m.pendingModification(ue, ebi)
	if pending == nil {
		return false
	}

	m.dedicatedUEAnswered(ctx, ue, b, pending, true, "")

	return true
}

func (m *MME) DedicatedBearerModifyRejected(ctx context.Context, ue *UeContext, ebi uint8, cause eps.ESMCause) bool {
	b, pending := m.pendingModification(ue, ebi)
	if pending == nil {
		return false
	}

	if cause != eps.ESMCauseInvalidEPSBearerIdentity {
		m.dedicatedUEAnswered(ctx, ue, b, pending, false, "UE rejected with "+cause.String())
		return true
	}

	ue.mu.Lock()
	if b.modifying == pending {
		b.modifying = nil
	}

	radioUp := b.radioUp
	ue.mu.Unlock()

	if ueConn := ue.Conn(); ueConn != nil && radioUp {
		m.sendDedicatedERABRelease(ctx, ueConn, ebi, nil)
	}

	m.releaseDedicated(ctx, ue, b, true)

	return true
}

func (m *MME) DedicatedRadioModified(ctx context.Context, ue *UeContext, ebi uint8, modified bool) bool {
	b, pending := m.pendingModification(ue, ebi)
	if b == nil {
		return false
	}

	if pending == nil {
		return true
	}

	outcome, reason := radioModified, ""
	if !modified {
		outcome, reason = radioNotModified, "eNB failed to modify the E-RAB"
	}

	m.settleRadio(ctx, ue, b, pending, outcome, reason)

	return true
}

func (m *MME) interruptRadioModifications(ctx context.Context, ue *UeContext) {
	type interrupted struct {
		b       *DedicatedBearer
		pending *dedicatedModification
	}

	var out []interrupted

	ue.mu.Lock()

	for _, p := range ue.Pdns {
		for _, b := range p.Dedicated {
			if b.modifying != nil && b.modifying.radio == radioPending {
				out = append(out, interrupted{b: b, pending: b.modifying})
			}
		}
	}
	ue.mu.Unlock()

	for _, i := range out {
		m.settleRadio(ctx, ue, i.b, i.pending, radioNotModified, "a handover interrupted the E-RAB modification")
	}
}

func (m *MME) dedicatedUEAnswered(ctx context.Context, ue *UeContext, b *DedicatedBearer, pending *dedicatedModification, accepted bool, reason string) {
	ue.mu.Lock()

	if b.modifying != pending || pending.ue != uePending {
		ue.mu.Unlock()
		return
	}

	pending.ue = ueRejected
	if accepted {
		pending.ue = ueAccepted
	}

	if pending.reason == "" {
		pending.reason = reason
	}

	if pending.awaitsRadio() {
		b.guard.ArmOnce(m.esmGuardCfg.ExpireTime, func() {
			m.settleRadio(context.Background(), ue, b, pending, radioNotModified, "no E-RAB Modify Response")
		})
	}
	ue.mu.Unlock()

	m.concludeDedicatedModification(ctx, ue, b, pending)
}

func (m *MME) settleRadio(ctx context.Context, ue *UeContext, b *DedicatedBearer, pending *dedicatedModification, outcome radioOutcome, reason string) {
	ue.mu.Lock()

	if b.modifying != pending || pending.radio != radioPending {
		ue.mu.Unlock()
		return
	}

	pending.radio = outcome

	if reason != "" && pending.reason == "" {
		pending.reason = reason
	}
	ue.mu.Unlock()

	m.concludeDedicatedModification(ctx, ue, b, pending)
}

func (m *MME) concludeDedicatedModification(ctx context.Context, ue *UeContext, b *DedicatedBearer, pending *dedicatedModification) {
	ue.mu.Lock()

	p, held := ue.dedicatedLocked(b.Ebi)

	verdict := pending.verdict()
	if held != b || b.modifying != pending || verdict == verdictPending {
		ue.mu.Unlock()
		return
	}

	b.guard.Stop()
	b.modifying = nil

	if verdict == verdictCommitted {
		b.DedicatedBearerInfo = pending.target
	}

	ref, teid, ebi := p.SessionRef, b.SgwFTEID.TEID, b.Ebi
	realign := models.DedicatedBearerModification{
		SessionRef: ref, SGWTEID: teid,
		QoSChanged: true, MBR: b.MBR, GBR: b.GBR,
	}

	if pending.mod.ARP != nil {
		realign.ARP = new(b.ARP)
	}
	ue.mu.Unlock()

	log := logger.From(ctx, logger.MmeLog).With(logger.SUPI(ue.Supi().String()), logger.ERABID(ebi))

	if pending.realign {
		if verdict != verdictCommitted {
			log.Warn("the E-RAB could not be realigned with the UE; deactivating the bearer (TS 24.301 §6.4.3.4)", zap.String("reason", pending.reason))
			m.DeactivateDedicated(ctx, ue, ebi)
		}

		return
	}

	switch verdict {
	case verdictCommitted:
		log.Info("dedicated EPS bearer modified")
		m.Session.DedicatedBearerModified(ctx, ref, teid, true)
	case verdictUEAhead:
		log.Warn("the UE accepted a modification the eNB did not apply; deactivating the bearer (TS 24.301 §6.4.3.4)", zap.String("reason", pending.reason))
		m.DeactivateDedicated(ctx, ue, ebi)
	case verdictRadioAhead:
		log.Warn("the UE rejected a modification the eNB applied; realigning the E-RAB (TS 24.301 §6.4.3.4)", zap.String("reason", pending.reason))
		m.Session.DedicatedBearerModified(ctx, ref, teid, false)

		if err := m.startModification(ctx, ue, ue.Conn(), ebi, realign, true); err != nil {
			log.Warn("E-RAB realignment not sent; deactivating the bearer", zap.Error(err))
			m.DeactivateDedicated(ctx, ue, ebi)
		}
	case verdictFailed:
		log.Warn("dedicated EPS bearer modification failed; keeping the previous configuration", zap.String("reason", pending.reason))
		m.Session.DedicatedBearerModified(ctx, ref, teid, false)
	}
}
