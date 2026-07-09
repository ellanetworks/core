// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// Page sends an S1AP Paging for an EMM-REGISTERED, ECM-IDLE UE so it
// re-establishes the S1 connection and buffered downlink data is delivered
// (TS 23.401 §5.3.4: page within the registered tracking area). Paging reaches
// every eNB serving the network's tracking area — every connected eNB under a
// single served TAC. The procedure is supervised and retransmitted up to a bound,
// then abandoned (T3413,
// TS 24.301 §5.6.2). A nil error covers a deliberate skip (already ECM-CONNECTED,
// or paging in progress); only a missing context or marshal failure is reported.
func (m *MME) Page(ctx context.Context, imsi string) error {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return fmt.Errorf("paging: no context for imsi %s", imsi)
	}

	m.mu.RLock()

	skip := ue.Connected() || ue.pagingTimer.Active()

	m.mu.RUnlock()

	if skip {
		return nil
	}

	paging, err := m.buildPaging(ue)
	if err != nil {
		return err
	}

	b, err := paging.Marshal()
	if err != nil {
		return fmt.Errorf("paging: marshal: %w", err)
	}

	m.pageRadios(ctx, ue, b)

	logger.From(ctx, logger.MmeLog).Info("Paging", zap.String("imsi", imsi), zap.Uint32("m-tmsi", ue.Tmsi().Uint32()))

	m.armPaging(ue, b)

	return nil
}

// armPaging starts the paging supervision guard for a UE just paged. A guard
// already running (a paging procedure in progress) is left untouched.
func (m *MME) armPaging(ue *UeContext, pdu []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.pagingTimer.Active() {
		return
	}

	ue.pagingTimer.ArmWith(m.pagingCfg,
		func(attempt int32) { m.retransmitPaging(ue, pdu, attempt) },
		func() { m.abandonPaging(ue) })
}

// stopPagingLocked cancels paging supervision. The guard invalidates any
// in-flight callback. The caller holds m.mu.
func (m *MME) stopPagingLocked(ue *UeContext) {
	ue.pagingTimer.Stop()
}

// retransmitPaging resends the Paging on each guard interval (T3413, TS 24.301
// §5.6.2), or stops the guard once the UE has answered (ECM-CONNECTED).
func (m *MME) retransmitPaging(ue *UeContext, pdu []byte, attempt int32) {
	m.mu.RLock()

	connected := ue.Connected()
	imsi := ue.imsiOrEmpty()

	m.mu.RUnlock()

	if connected {
		ue.pagingTimer.Stop()
		return
	}

	logger.MmeLog.Info("paging unanswered, retransmitting",
		zap.String("imsi", imsi), zap.Int32("attempt", attempt))
	m.pageRadios(context.Background(), ue, pdu)
}

// abandonPaging runs once the retransmission budget is exhausted (TS 24.301
// §5.6.2; TS 23.401 §5.3.4.3). The buffered downlink data remains at the anchor
// and the UE stays under mobile-reachable supervision until it returns or is
// implicitly detached.
func (m *MME) abandonPaging(ue *UeContext) {
	m.mu.RLock()

	imsi := ue.imsiOrEmpty()

	m.mu.RUnlock()

	logger.MmeLog.Info("paging unanswered, abandoning procedure", zap.String("imsi", imsi))
}

// buildPaging assembles the Paging message for a UE (TS 36.413). The TAI list is the
// UE's registration area, so the network pages within the area it registered the UE in.
func (m *MME) buildPaging(ue *UeContext) (*s1ap.Paging, error) {
	_, mmeCode := m.MmeIdentity()

	taiList, err := areaToS1APTAIs(ue.RegistrationArea())
	if err != nil {
		return nil, fmt.Errorf("paging: %w", err)
	}

	// During a GUTI reallocation the UE still answers to the old M-TMSI until it
	// sends TAU Complete, so page with that one while it is pending.
	mtmsi := ue.Tmsi().Uint32()
	if ue.OldTmsi().Uint32() != 0 {
		mtmsi = ue.OldTmsi().Uint32()
	}

	return &s1ap.Paging{
		UEIdentityIndexValue: ueIdentityIndex(ue.imsiOrEmpty()),
		STMSI:                s1ap.STMSI{MMEC: mmeCode, MTMSI: mtmsi},
		CNDomain:             s1ap.CNDomainPS,
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
// broadcast TAIs intersect the UE's registration area (TS 23.401 §5.3.4: page within
// the registered tracking area). An eNB that broadcasts only tracking areas outside
// that area is skipped; under a single served TAC this reaches every eNB. Connections
// are snapshotted under the lock so the blocking writes happen without holding it.
func (m *MME) pageRadios(ctx context.Context, ue *UeContext, b []byte) {
	area := ue.RegistrationArea()

	m.mu.RLock()
	conns := make([]S1APWriter, 0, len(m.radios))

	for conn, radio := range m.radios {
		if radioServesAnyLocked(radio, area) {
			conns = append(conns, conn)
		}
	}

	m.mu.RUnlock()

	for _, conn := range conns {
		m.SendS1APConn(ctx, conn, S1APProcedurePaging, b)
	}
}

// ServedTAIs is the network's served tracking areas: the operator PLMN paired with
// each served TAC. The MME registers every UE in this area, so it is the UE's
// registration area (TS 23.401 §5.3.4).
func (m *MME) ServedTAIs(ctx context.Context) ([]models.Tai, error) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		return nil, fmt.Errorf("operator PLMN: %w", err)
	}

	tacs, err := m.OperatorTACs(ctx)
	if err != nil {
		return nil, fmt.Errorf("operator TACs: %w", err)
	}

	out := make([]models.Tai, 0, len(tacs))

	for _, tac := range tacs {
		p := plmn
		out = append(out, models.Tai{PlmnID: &p, Tac: fmt.Sprintf("%06x", tac)})
	}

	return out, nil
}

// areaToS1APTAIs encodes a registration area as the S1AP TAI list carried in Paging
// (and rejects an empty area, which would page the UE nowhere).
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
