// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// Page sends an S1AP Paging for an EMM-REGISTERED, ECM-IDLE UE so it
// re-establishes the S1 connection (via a Service Request) and the buffered
// downlink data is delivered (TS 23.401). It pages every connected eNB:
// Ella Core is single-TAC, so all eNBs serve the paging tracking area. The
// paging procedure is supervised: if the UE does not respond, the Paging is
// retransmitted up to a bounded number of times, then abandoned (T3413,
// TS 24.301 §5.6.2). A repeated trigger while a paging procedure is already in
// progress is folded into it rather than restarting the supervision.
//
// A nil error means the paging was sent or deliberately skipped (the UE is
// already ECM-CONNECTED or a paging procedure is already running); only a
// missing UE context or a marshal failure is reported. The eNB broadcast itself
// is best-effort — a per-eNB write failure does not fail the call.
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

	paging, err := m.buildPaging(ctx, ue)
	if err != nil {
		return err
	}

	b, err := paging.Marshal()
	if err != nil {
		return fmt.Errorf("paging: marshal: %w", err)
	}

	m.broadcastPaging(ctx, b)

	logger.MmeLog.Info("Paging", zap.String("imsi", imsi), zap.Uint32("m-tmsi", ue.mtmsi))

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

	ue.pagingTimer.Arm(m.pagingTimeout, int32(m.pagingMaxRetransmit),
		func(attempt int32) { m.retransmitPaging(ue, pdu, attempt) },
		func() { m.abandonPaging(ue) })
}

// stopPagingLocked cancels paging supervision. The guard invalidates any
// in-flight callback. The caller holds m.mu.
func (m *MME) stopPagingLocked(ue *UeContext) {
	ue.pagingTimer.Stop()
}

// retransmitPaging resends the Paging on each guard interval (T3413, TS 24.301
// §5.6.2). If the UE has answered (ECM-CONNECTED) it stops the guard instead.
func (m *MME) retransmitPaging(ue *UeContext, pdu []byte, attempt int32) {
	m.mu.RLock()

	connected := ue.Connected()
	imsi := ue.imsi

	m.mu.RUnlock()

	if connected {
		ue.pagingTimer.Stop()
		return
	}

	logger.MmeLog.Info("paging unanswered, retransmitting",
		zap.String("imsi", imsi), zap.Int32("attempt", attempt))
	m.broadcastPaging(context.Background(), pdu)
}

// abandonPaging runs once the retransmission budget is exhausted (TS 24.301
// §5.6.2; TS 23.401 §5.3.4.3). The buffered downlink data remains at the anchor
// and the UE stays under mobile-reachable supervision until it returns or is
// implicitly detached.
func (m *MME) abandonPaging(ue *UeContext) {
	m.mu.RLock()

	imsi := ue.imsi

	m.mu.RUnlock()

	logger.MmeLog.Info("paging unanswered, abandoning procedure", zap.String("imsi", imsi))
}

// buildPaging assembles the Paging message for a UE: the S-TMSI paging identity,
// the IMSI-derived UE identity index, the PS domain, and the operator's tracking
// area (TS 36.413).
func (m *MME) buildPaging(ctx context.Context, ue *UeContext) (*s1ap.Paging, error) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		return nil, fmt.Errorf("paging: operator PLMN: %w", err)
	}

	tac, err := m.OperatorTAC(ctx)
	if err != nil {
		return nil, fmt.Errorf("paging: operator TAC: %w", err)
	}

	_, mmeCode := m.MmeIdentity()

	plmnID, err := EncodePLMN(plmn)
	if err != nil {
		return nil, fmt.Errorf("paging: %w", err)
	}

	// During a GUTI reallocation the UE still answers to the old M-TMSI until it
	// sends TAU Complete, so page with that one while it is pending.
	mtmsi := ue.mtmsi
	if ue.oldMTMSI != 0 {
		mtmsi = ue.oldMTMSI
	}

	return &s1ap.Paging{
		UEIdentityIndexValue: ueIdentityIndex(ue.imsi),
		STMSI:                s1ap.STMSI{MMEC: mmeCode, MTMSI: mtmsi},
		CNDomain:             s1ap.CNDomainPS,
		TAIList:              []s1ap.TAI{{PLMNIdentity: plmnID, TAC: s1ap.TAC(tac)}},
	}, nil
}

// ueIdentityIndex is the 10-bit UE Identity Index Value that selects the paging
// occasion: IMSI mod 1024 (TS 36.304).
func ueIdentityIndex(imsi string) uint16 {
	n, _ := strconv.ParseUint(imsi, 10, 64)

	return uint16(n % 1024)
}

// broadcastPaging writes a non-UE-associated Paging PDU to every connected eNB.
// Connections are snapshotted under the lock so the blocking writes happen
// without holding it.
func (m *MME) broadcastPaging(ctx context.Context, b []byte) {
	m.mu.RLock()
	conns := make([]*sctp.SCTPConn, 0, len(m.enbs))

	for conn := range m.enbs {
		conns = append(conns, conn)
	}

	m.mu.RUnlock()

	for _, conn := range conns {
		if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: S1apWirePPID, Stream: S1apStreamNonUE}); err != nil {
			logger.MmeLog.Warn("failed to send Paging to eNB", zap.Error(err))
			continue
		}

		m.LogNetworkEvent(ctx, conn, S1APProcedurePaging, logger.DirectionOutbound, b)
	}
}
