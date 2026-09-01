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
)

func (p N2SetupProcedure) String() string {
	switch p {
	case N2SetupInitialContext:
		return "InitialContextSetup"
	case N2SetupPDUSession:
		return "PDUSessionResourceSetup"
	default:
		return "unknown"
	}
}

type n2SetupTxn struct {
	sessions []uint8
	guard    *guard.Guard
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

func (s N2Setup) Claim(ids []uint8) []uint8 {
	return s.conn.ClaimN2Setup(s.proc, ids)
}

func (s N2Setup) ClaimSession(id uint8) bool {
	return s.conn.ClaimN2SetupSession(s.proc, id)
}

func (s N2Setup) Arm(cfg guard.TimerValue) {
	s.conn.ArmN2Setup(s.proc, cfg)
}

func (s N2Setup) End() {
	s.conn.EndN2Setup(s.proc)
}

func (ueConn *UeConn) ClaimN2Setup(proc N2SetupProcedure, ids []uint8) []uint8 {
	ue := ueConn.UeContext()
	if ue == nil {
		return nil
	}

	claimed := make([]uint8, 0, len(ids))

	for _, id := range ids {
		if ue.ClaimSmContextN2(id) {
			claimed = append(claimed, id)
		}
	}

	if len(claimed) == 0 {
		return nil
	}

	ueConn.n2Setups.mu.Lock()
	defer ueConn.n2Setups.mu.Unlock()

	if ueConn.n2Setups.open == nil {
		ueConn.n2Setups.open = make(map[N2SetupProcedure]*n2SetupTxn)
	}

	txn, ok := ueConn.n2Setups.open[proc]
	if !ok {
		txn = &n2SetupTxn{guard: &guard.Guard{}}
		ueConn.n2Setups.open[proc] = txn
	}

	txn.sessions = append(txn.sessions, claimed...)

	return claimed
}

func (ueConn *UeConn) ClaimN2SetupSession(proc N2SetupProcedure, id uint8) bool {
	return len(ueConn.ClaimN2Setup(proc, []uint8{id})) == 1
}

func (ueConn *UeConn) ArmN2Setup(proc N2SetupProcedure, cfg guard.TimerValue) {
	if !cfg.Enable {
		return
	}

	ueConn.n2Setups.mu.Lock()
	defer ueConn.n2Setups.mu.Unlock()

	txn, ok := ueConn.n2Setups.open[proc]
	if !ok {
		return
	}

	txn.guard.ArmOnce(cfg.ExpireTime, func() {
		ueConn.expireN2Setup(proc, txn)
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
	if ok && (want == nil || txn == want) {
		delete(ueConn.n2Setups.open, proc)
	} else {
		ok = false
	}

	ueConn.n2Setups.mu.Unlock()

	if !ok {
		return nil
	}

	txn.guard.Stop()

	ue := ueConn.UeContext()
	if ue == nil {
		return nil
	}

	released := make([]uint8, 0, len(txn.sessions))

	for _, id := range txn.sessions {
		if ue.ReleaseSmContextN2IfPending(id) {
			released = append(released, id)
		}
	}

	return released
}

func (ueConn *UeConn) AbortN2Setups() {
	ueConn.n2Setups.mu.Lock()
	open := ueConn.n2Setups.open
	ueConn.n2Setups.open = nil
	ueConn.n2Setups.mu.Unlock()

	ue := ueConn.UeContext()

	for _, txn := range open {
		txn.guard.Stop()

		if ue == nil {
			continue
		}

		for _, id := range txn.sessions {
			ue.ReleaseSmContextN2IfPending(id)
		}
	}
}

func (ueConn *UeConn) N2SetupOpen(proc N2SetupProcedure) bool {
	ueConn.n2Setups.mu.Lock()
	defer ueConn.n2Setups.mu.Unlock()

	_, ok := ueConn.n2Setups.open[proc]

	return ok
}
