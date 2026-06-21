// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"strconv"
	"time"

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
func (m *MME) Page(ctx context.Context, imsi string) error {
	ue := m.lookupUeByIMSI(imsi)
	if ue == nil {
		return fmt.Errorf("paging: no context for imsi %s", imsi)
	}

	m.mu.RLock()

	skip := ue.ecmState == ECMConnected || ue.pagingTimer != nil

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

// armPaging starts the paging supervision timer for a UE just paged. A timer
// already running (a paging procedure in progress) is left untouched.
func (m *MME) armPaging(ue *UeContext, pdu []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.pagingTimer != nil {
		return
	}

	ue.pagingPDU = pdu
	ue.pagingTries = 0
	gen := ue.pagingGen

	ue.pagingTimer = time.AfterFunc(m.pagingTimeout, func() {
		m.onPagingExpiry(ue, gen)
	})
}

// stopPagingLocked cancels paging supervision and bumps pagingGen so an
// in-flight callback becomes a no-op. The caller holds m.mu.
func (m *MME) stopPagingLocked(ue *UeContext) {
	ue.pagingGen++

	if ue.pagingTimer != nil {
		ue.pagingTimer.Stop()
		ue.pagingTimer = nil
	}

	ue.pagingPDU = nil
	ue.pagingTries = 0
}

// onPagingExpiry retransmits the Paging or, once the retransmission budget is
// exhausted, abandons the paging procedure (TS 24.301 §5.6.2; TS 23.401
// §5.3.4.3). The buffered downlink data remains at the anchor and the UE stays
// under mobile-reachable supervision until it returns or is implicitly detached.
func (m *MME) onPagingExpiry(ue *UeContext, gen uint64) {
	m.mu.Lock()

	if ue.pagingGen != gen {
		m.mu.Unlock()
		return
	}

	if ue.ecmState == ECMConnected {
		m.stopPagingLocked(ue)
		m.mu.Unlock()

		return
	}

	if ue.pagingTries >= m.pagingMaxRetransmit {
		mmeUEID, imsi := ue.MMEUES1APID, ue.imsi
		m.stopPagingLocked(ue)
		m.mu.Unlock()

		logger.MmeLog.Info("paging unanswered, abandoning procedure",
			zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("imsi", imsi))

		return
	}

	ue.pagingTries++
	pdu := ue.pagingPDU
	tries := ue.pagingTries

	ue.pagingTimer = time.AfterFunc(m.pagingTimeout, func() {
		m.onPagingExpiry(ue, gen)
	})

	m.mu.Unlock()

	logger.MmeLog.Info("paging unanswered, retransmitting",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi), zap.Int("attempt", tries))
	m.broadcastPaging(context.Background(), pdu)
}

// buildPaging assembles the Paging message for a UE: the S-TMSI paging identity,
// the IMSI-derived UE identity index, the PS domain, and the operator's tracking
// area (TS 36.413).
func (m *MME) buildPaging(ctx context.Context, ue *UeContext) (*s1ap.Paging, error) {
	plmn, err := m.operatorPLMN(ctx)
	if err != nil {
		return nil, fmt.Errorf("paging: operator PLMN: %w", err)
	}

	tac, err := m.operatorTAC(ctx)
	if err != nil {
		return nil, fmt.Errorf("paging: operator TAC: %w", err)
	}

	_, mmeCode := m.mmeIdentity()

	plmnID, err := encodePLMN(plmn)
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
		if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamNonUE}); err != nil {
			logger.MmeLog.Warn("failed to send Paging to eNB", zap.Error(err))
			continue
		}

		m.logNetworkEvent(ctx, conn, S1APProcedurePaging, logger.DirectionOutbound, b)
	}
}
