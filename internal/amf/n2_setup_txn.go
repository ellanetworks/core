// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"sync"

	"github.com/ellanetworks/core/internal/guard"
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
	ueConn.n2Setups.mu.Lock()
	txn, ok := ueConn.n2Setups.open[proc]
	ueConn.n2Setups.mu.Unlock()

	if !ok || !cfg.Enable {
		return
	}

	txn.guard.ArmOnce(cfg.ExpireTime, func() {
		ueConn.EndN2Setup(proc)
	})
}

func (ueConn *UeConn) EndN2Setup(proc N2SetupProcedure) {
	ueConn.n2Setups.mu.Lock()
	txn, ok := ueConn.n2Setups.open[proc]
	delete(ueConn.n2Setups.open, proc)
	ueConn.n2Setups.mu.Unlock()

	if !ok {
		return
	}

	txn.guard.Stop()

	ue := ueConn.UeContext()
	if ue == nil {
		return
	}

	for _, id := range txn.sessions {
		ue.ReleaseSmContextN2IfPending(id)
	}
}

func (ueConn *UeConn) AbortN2Setups() {
	ueConn.n2Setups.mu.Lock()
	open := ueConn.n2Setups.open
	ueConn.n2Setups.open = nil
	ueConn.n2Setups.mu.Unlock()

	for _, txn := range open {
		txn.guard.Stop()
	}
}

func (ueConn *UeConn) N2SetupOpen(proc N2SetupProcedure) bool {
	ueConn.n2Setups.mu.Lock()
	defer ueConn.n2Setups.mu.Unlock()

	_, ok := ueConn.n2Setups.open[proc]

	return ok
}
