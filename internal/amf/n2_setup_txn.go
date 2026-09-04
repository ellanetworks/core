// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"sync"

	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type N2SetupProcedure uint8

const (
	N2SetupInitialContext N2SetupProcedure = iota
	N2SetupPDUSession
	N2SetupHandover
)

func (p N2SetupProcedure) String() string {
	switch p {
	case N2SetupInitialContext:
		return "InitialContextSetup"
	case N2SetupPDUSession:
		return "PDUSessionResourceSetup"
	case N2SetupHandover:
		return "HandoverResourceAllocation"
	default:
		return "unknown"
	}
}

type n2SetupTxn struct {
	guard   *guard.Guard
	closing bool
}

type n2SetupTxns struct {
	mu   sync.Mutex
	open map[N2SetupProcedure]*n2SetupTxn
}

type N2Setup struct {
	conn *UeConn
	proc N2SetupProcedure
}

func (ueConn *UeConn) N2Setup(proc N2SetupProcedure) N2Setup {
	return N2Setup{conn: ueConn, proc: proc}
}

func (s N2Setup) ClaimSession(id uint8) bool {
	return len(s.Claim([]uint8{id})) == 1
}

func (s N2Setup) End() {
	s.conn.EndN2Setup(s.proc)
}

func (s N2Setup) Claim(ids []uint8) []uint8 {
	claimed := make([]uint8, 0, len(ids))

	for _, id := range ids {
		if s.conn.ClaimN2Session(s.proc, id) {
			claimed = append(claimed, id)
		}
	}

	if len(claimed) == 0 {
		return nil
	}

	s.conn.n2Setups.mu.Lock()
	defer s.conn.n2Setups.mu.Unlock()

	if s.conn.n2Setups.open == nil {
		s.conn.n2Setups.open = make(map[N2SetupProcedure]*n2SetupTxn)
	}

	if _, ok := s.conn.n2Setups.open[s.proc]; !ok {
		s.conn.n2Setups.open[s.proc] = &n2SetupTxn{guard: &guard.Guard{}}
	}

	return claimed
}

func (s N2Setup) Arm(cfg guard.TimerValue) {
	if !cfg.Enable {
		return
	}

	s.conn.n2Setups.mu.Lock()
	defer s.conn.n2Setups.mu.Unlock()

	txn, ok := s.conn.n2Setups.open[s.proc]
	if !ok {
		return
	}

	txn.guard.ArmOnce(cfg.ExpireTime, func() {
		s.conn.expireN2Setup(s.proc, txn)
	})
}

func (ueConn *UeConn) EndN2Setup(proc N2SetupProcedure) {
	ueConn.endN2SetupTxn(proc, nil)
}

func (ueConn *UeConn) expireN2Setup(proc N2SetupProcedure, txn *n2SetupTxn) {
	released := ueConn.endN2SetupTxn(proc, txn)
	if len(released) == 0 {
		return
	}

	ue := ueConn.UeContext()
	if ue == nil || ueConn.amf == nil || ueConn.amf.Session == nil {
		return
	}

	ctx := context.Background()

	for _, id := range released {
		smContext, ok := ue.SmContextFindByPDUSessionID(id)
		if !ok {
			continue
		}

		logger.From(ctx, ueConn.Log()).Warn("no answer to the PDU session resource setup; deactivating the session at the SMF",
			zap.Uint8("pdu_session_id", id), zap.Stringer("procedure", proc))

		if err := ueConn.amf.Session.DeactivateSmContext(ctx, smContext.Ref); err != nil {
			logger.From(ctx, ueConn.Log()).Error("could not deactivate a PDU session whose setup went unanswered",
				zap.Uint8("pdu_session_id", id), zap.Error(err))
		}
	}
}

func (ueConn *UeConn) endN2SetupTxn(proc N2SetupProcedure, want *n2SetupTxn) []uint8 {
	ueConn.n2Setups.mu.Lock()

	txn, ok := ueConn.n2Setups.open[proc]
	if !ok || txn.closing || (want != nil && txn != want) {
		ueConn.n2Setups.mu.Unlock()
		return nil
	}

	txn.closing = true

	ueConn.n2Setups.mu.Unlock()

	released := ueConn.releaseN2SetupTxn(proc, txn)

	ueConn.n2Setups.mu.Lock()

	if cur, ok := ueConn.n2Setups.open[proc]; ok && cur == txn {
		delete(ueConn.n2Setups.open, proc)
	}

	ueConn.n2Setups.mu.Unlock()

	return released
}

func (ueConn *UeConn) releaseN2SetupTxn(proc N2SetupProcedure, txn *n2SetupTxn) []uint8 {
	txn.guard.Stop()

	var released []uint8

	for _, id := range ueConn.pendingN2SessionsFor(proc) {
		if ueConn.releaseN2SessionIfPending(id) {
			released = append(released, id)
		}
	}

	return released
}

func (ueConn *UeConn) AbortN2Setups() {
	ueConn.n2Setups.mu.Lock()

	open := make(map[N2SetupProcedure]*n2SetupTxn, len(ueConn.n2Setups.open))

	for proc, txn := range ueConn.n2Setups.open {
		if txn.closing {
			continue
		}

		txn.closing = true
		open[proc] = txn
	}

	ueConn.n2Setups.mu.Unlock()

	for proc, txn := range open {
		ueConn.releaseN2SetupTxn(proc, txn)
	}

	ueConn.n2Setups.mu.Lock()

	for proc, txn := range open {
		if cur, ok := ueConn.n2Setups.open[proc]; ok && cur == txn {
			delete(ueConn.n2Setups.open, proc)
		}
	}

	ueConn.n2Setups.mu.Unlock()
}

func (ueConn *UeConn) N2SetupOpen(proc N2SetupProcedure) bool {
	ueConn.n2Setups.mu.Lock()
	defer ueConn.n2Setups.mu.Unlock()

	_, ok := ueConn.n2Setups.open[proc]

	return ok
}

func (ueConn *UeConn) RANHoldsUEContext() bool {
	return ueConn.ICS() != ICSNotStarted
}

func (ueConn *UeConn) ClaimN2Setup(carriesSessions bool) (N2SetupProcedure, bool) {
	if (ueConn.UeContextRequest || carriesSessions) && ueConn.ClaimICS() {
		return N2SetupInitialContext, true
	}

	return N2SetupPDUSession, false
}
