// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// errPagingSkipped reports a deliberate no-op: the UE is already ECM-CONNECTED or already
// being paged. Entry points decide whether that is a success or a failure.
var errPagingSkipped = errors.New("paging skipped")

var causeErrorIndicationReceived = s1ap.Cause{Group: s1ap.CauseGroupTransport, Value: s1ap.CauseTransportResourceUnavailable}

// Page sends an S1AP Paging for an EMM-REGISTERED, ECM-IDLE UE so it re-establishes
// the S1 connection and buffered downlink data is delivered, within the UE's
// registered tracking area (TS 23.401 §5.3.4). The procedure is supervised and
// retransmitted up to a bound, then abandoned (T3413, TS 24.301 §5.6.2). A nil error
// covers a deliberate skip (already ECM-CONNECTED, or paging in progress); only a
// missing context or marshal failure is reported.
func (m *MME) NotifyDownlinkData(ctx context.Context, imsi string, ebi uint8, cause models.DownlinkDataNotificationCause) error {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return fmt.Errorf("paging: no context for imsi %s", imsi)
	}

	req := &MTRequest{Ebi: ebi}

	if cause == models.DownlinkDataErrorIndication && ue.Connected() {
		m.releaseForErrorIndication(ctx, ue, req)

		return nil
	}

	arm := func() bool { return ue.beginPaging(req) }

	if err := m.page(ctx, ue, arm); err != nil && !errors.Is(err, errPagingSkipped) {
		return err
	}

	return nil
}

func (m *MME) releaseForErrorIndication(ctx context.Context, ue *UeContext, req *MTRequest) {
	logger.From(ctx, logger.MmeLog).Info("Releasing S1 after a GTP-U Error Indication",
		logger.SUPI(ue.Supi().String()))

	ue.deferServiceRequest(req)

	m.ReleaseUEContext(ctx, ue, causeErrorIndicationReceived)
}

func (m *MME) ResumeDeferredServiceRequest(ctx context.Context, ue *UeContext) {
	req := ue.takeDeferredServiceRequest()
	if req == nil {
		return
	}

	arm := func() bool { return ue.beginPaging(req) }

	if err := m.page(ctx, ue, arm); err != nil && !errors.Is(err, errPagingSkipped) {
		logger.From(ctx, logger.MmeLog).Warn("could not page the UE after releasing S1 for a GTP-U Error Indication",
			logger.SUPI(ue.Supi().String()), zap.Error(err))
	}
}

func (m *MME) DropDeferredServiceRequest(ctx context.Context, ue *UeContext) {
	req := ue.takeDeferredServiceRequest()
	if req == nil || req.Ebi == 0 || m.Session == nil {
		return
	}

	imsi := ue.imsiOrEmpty()

	if err := m.Session.HandleEPSPagingFailure(ctx, imsi, req.Ebi, models.EPSPagingUENotResponding); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("could not report a downlink delivery failure for a UE released before it could be paged",
			logger.SUPIFromIMSI(imsi), zap.Uint8("ebi", req.Ebi), zap.Error(err))
	}
}

// page is the build-and-send path behind Page and PageAndRetryLPPa. arm runs immediately
// before the Paging is sent, because the UE may answer on another goroutine the moment it
// leaves the MME. No undo is needed: pageRadios reports send failures at the chokepoint.
func (m *MME) page(ctx context.Context, ue *UeContext, arm func() bool) error {
	m.mu.RLock()

	skip := ue.Connected() || ue.paging.guard.Active()
	imsi := ue.imsiOrEmpty()

	m.mu.RUnlock()

	if skip {
		return errPagingSkipped
	}

	paging, err := m.buildPaging(ctx, ue)
	if err != nil {
		return err
	}

	b, err := paging.Marshal()
	if err != nil {
		return fmt.Errorf("paging: marshal: %w", err)
	}

	if arm != nil && !arm() {
		return errPagingSkipped
	}

	m.pageRadios(ctx, ue, b)

	logger.From(ctx, logger.MmeLog).Info("Paging", logger.SUPIFromIMSI(imsi), zap.Uint32("m_tmsi", ue.Tmsi().Uint32()))

	m.armPaging(ctx, ue, b)

	return nil
}

// armPaging starts the paging supervision guard for a UE just paged. A guard
// already running (a paging procedure in progress) is left untouched.
func (m *MME) armPaging(ctx context.Context, ue *UeContext, pdu []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.paging.guard.Active() {
		return
	}

	link := trace.SpanContextFromContext(ctx)

	ue.paging.mu.Lock()
	ue.paging.attempt++
	pagingAttempt := ue.paging.attempt
	ue.paging.mu.Unlock()

	ue.paging.guard.ArmWith(m.pagingCfg,
		func(attempt int32) { m.retransmitPaging(link, ue, pdu, attempt) },
		func() { m.abandonPaging(link, ue, pagingAttempt) })
}

func (ue *UeContext) clearPaging() {
	ue.paging.guard.Stop()

	ue.paging.mu.Lock()
	ue.paging.pending = nil
	ue.paging.state = PagingIdle
	ue.paging.mu.Unlock()

	ue.ClearLPPaBuffered()
}

// retransmitPaging resends the Paging on each guard interval (T3413, TS 24.301
// §5.6.2), or stops the guard once the UE has answered (ECM-CONNECTED).
func (m *MME) retransmitPaging(link trace.SpanContext, ue *UeContext, pdu []byte, attempt int32) {
	m.mu.RLock()

	connected := ue.Connected()
	imsi := ue.imsiOrEmpty()

	m.mu.RUnlock()

	if connected {
		ue.paging.guard.Stop()
		return
	}

	ctx, span := guardSpan(link, "mme/paging_retransmit", "T3413 (Paging)", attempt)
	defer span.End()

	logger.From(ctx, logger.MmeLog).Info("paging unanswered, retransmitting",
		logger.SUPIFromIMSI(imsi), zap.Int32("attempt", attempt))
	m.pageRadios(ctx, ue, pdu)
}

// abandonPaging suppresses the anchor's downlink data notification so further
// downlink packets do not re-page an unreachable UE (TS 23.401 §5.3.4.3); the UE
// stays under mobile-reachable supervision until it returns or is implicitly detached.
func (m *MME) abandonPaging(link trace.SpanContext, ue *UeContext, attempt uint64) {
	m.mu.RLock()

	imsi := ue.imsiOrEmpty()

	m.mu.RUnlock()

	ctx, span := guardSpan(link, "mme/paging_abandon", "T3413 (Paging)", 0)
	defer span.End()

	dropped, abandoned := ue.PagingUnanswered(ctx, attempt, models.EPSPagingUENotResponding)
	if !abandoned {
		return
	}

	logger.From(ctx, logger.MmeLog).Info("paging unanswered, abandoning procedure", logger.SUPIFromIMSI(imsi))

	if m.Session == nil {
		return
	}

	for _, p := range m.SnapshotPDNs(ue) {
		if dropped != nil && dropped.Ebi == p.Ebi {
			continue
		}

		if err := m.Session.HandleEPSPagingFailure(ctx, imsi, p.Ebi, models.EPSPagingUENotResponding); err != nil {
			logger.MmeLog.Warn("failed to suppress downlink notification after paging failure",
				logger.SUPIFromIMSI(imsi), zap.Uint8("ebi", p.Ebi), zap.Error(err))
		}
	}
}

// buildPaging assembles the Paging message for a UE (TS 36.413). The TAI list is the
// UE's registration area.
func (m *MME) buildPaging(ctx context.Context, ue *UeContext) (*s1ap.Paging, error) {
	_, mmeCode, err := m.MmeIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("paging: %w", err)
	}

	taiList, err := areaToS1APTAIs(ue.RegistrationArea())
	if err != nil {
		return nil, fmt.Errorf("paging: %w", err)
	}

	// During a GUTI reallocation the UE still answers to the old M-TMSI until it
	// sends TAU Complete, so page with that one while it is pending.
	mtmsi := ue.Tmsi().Uint32()
	if ue.OldTmsi() != etsi.InvalidTMSI {
		mtmsi = ue.OldTmsi().Uint32()
	}

	return &s1ap.Paging{
		UEIdentityIndexValue: s1ap.Ptr(ueIdentityIndex(ue.imsiOrEmpty())),
		STMSI:                &s1ap.STMSI{MMEC: s1ap.MMECode(mmeCode), MTMSI: s1ap.MTMSI(mtmsi)},
		CNDomain:             s1ap.Ptr(s1ap.CNDomainPS),
		TAIList:              taiList,
		// Replay the eNB-reported paging capability so it can apply paging
		// optimisations (TS 36.413 §9.1.6.1); omitted when none was reported.
		UERadioCapabilityForPaging: ue.RadioCapabilityForPaging,
	}, nil
}

// ueIdentityIndex is the 10-bit UE Identity Index Value that selects the paging
// occasion: IMSI mod 1024 (TS 36.304).
func ueIdentityIndex(imsi string) uint16 {
	n, _ := strconv.ParseUint(imsi, 10, 64)

	return uint16(n % 1024)
}

// pageRadios writes a non-UE-associated Paging PDU to every connected eNB whose
// broadcast TAIs intersect the UE's registration area (TS 23.401 §5.3.4). Connections
// are snapshotted under the lock so the blocking writes happen without holding it.
func (m *MME) pageRadios(ctx context.Context, ue *UeContext, b []byte) {
	area := ue.RegistrationArea()

	m.mu.RLock()
	conns := make([]S1APWriter, 0, m.reg.CountConnected())

	for conn, radio := range m.reg.ByConn {
		if radioServesAnyLocked(radio, area) {
			conns = append(conns, conn)
		}
	}

	m.mu.RUnlock()

	for _, conn := range conns {
		_ = m.SendToRadio(ctx, conn, S1APProcedurePaging, b)
	}
}

// ServedTAIs is the network's served tracking areas, per
// OperatorConfig.ServedTAIs.
func (m *MME) ServedTAIs(ctx context.Context) ([]models.Tai, error) {
	o, err := m.Operator(ctx)
	if err != nil {
		return nil, err
	}

	return o.ServedTAIs()
}

// areaToS1APTAIs encodes a registration area as the S1AP TAI list carried in Paging.
// An empty area is rejected: it would page the UE nowhere.
func areaToS1APTAIs(area []models.Tai) ([]s1ap.TAI, error) {
	if len(area) == 0 {
		return nil, fmt.Errorf("empty registration area")
	}

	out := make([]s1ap.TAI, 0, len(area))

	for _, t := range area {
		if t.PlmnID == nil {
			return nil, fmt.Errorf("registration-area TAI with no PLMN")
		}

		plmnID, err := EncodePLMN(*t.PlmnID)
		if err != nil {
			return nil, fmt.Errorf("encode PLMN: %w", err)
		}

		tac, err := strconv.ParseUint(t.Tac, 16, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid TAC %q: %w", t.Tac, err)
		}

		out = append(out, s1ap.TAI{PLMNIdentity: plmnID, TAC: s1ap.TAC(tac)})
	}

	return out, nil
}

// radioServesAnyLocked reports whether the eNB broadcasts any of the given tracking
// areas. The caller holds m.mu.
func radioServesAnyLocked(radio *Radio, area []models.Tai) bool {
	for _, s := range radio.supportedTAIs {
		for _, t := range area {
			if s.Tai.Equal(t) {
				return true
			}
		}
	}

	return false
}

// PageAndRetryLPPa buffers an LPPa payload on an ECM-IDLE UE and pages it, for delivery
// when the UE answers (TS 23.273 §6.11.2 step 2). Unlike Page, a UE that needs no page is
// reported as an error so the LMF can fall back to a coarser method immediately.
func (m *MME) PageAndRetryLPPa(ctx context.Context, supi etsi.SUPI, measID int64, lppaPayload []byte) error {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("UE not found: %s", supi)
	}

	m.mu.RLock()

	handover := ue.handover

	m.mu.RUnlock()

	if handover != nil {
		return fmt.Errorf("temporary reject: handover ongoing")
	}

	if ue.EMMState() != EMMRegistered {
		return fmt.Errorf("UE is not in registered state")
	}

	arm := func() bool {
		if !ue.beginPaging(&MTRequest{}) {
			return false
		}

		ue.SetLPPaBuffered(measID, lppaPayload)

		return true
	}

	if err := m.page(ctx, ue, arm); err != nil {
		return fmt.Errorf("failed to page ECM-IDLE UE: %w", err)
	}

	logger.MmeLog.Info("LPPa message buffered, paging ECM-IDLE UE",
		logger.SUPI(ue.Supi().String()),
		zap.Int64("measurement_id", measID),
		zap.Int("lppa_len", len(lppaPayload)),
	)

	return nil
}

// CancelBufferedLPPa discards the LPPa payload buffered under measID, once the LMF stops
// waiting. Paging supervision outlives the LMF's timeout, so a UE answering late would
// otherwise be sent a request nobody awaits.
func (m *MME) CancelBufferedLPPa(supi etsi.SUPI, measID int64) {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return
	}

	ue.ClearLPPaBufferedIf(measID)
}
