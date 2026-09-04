// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/guard"
)

func newTxnTestConn() *UeConn {
	conn := &UeConn{}
	conn.ue.Store(NewUeContext())

	return conn
}

func TestEndN2SetupTxnOnlyClosesTheTransactionItNames(t *testing.T) {
	conn := newTxnTestConn()

	first := conn.N2Setup(N2SetupPDUSession).Claim([]uint8{1})
	if len(first) != 1 {
		t.Fatalf("claimed %v, want PDU session 1", first)
	}

	conn.n2Setups.mu.Lock()
	stale := conn.n2Setups.open[N2SetupPDUSession]
	conn.n2Setups.mu.Unlock()

	conn.EndN2Setup(N2SetupPDUSession)

	if len(conn.N2Setup(N2SetupPDUSession).Claim([]uint8{2})) != 1 {
		t.Fatal("could not open a second transaction")
	}

	conn.endN2SetupTxn(N2SetupPDUSession, stale)

	if !conn.N2SetupOpen(N2SetupPDUSession) {
		t.Error("a guard left over from an ended transaction closed the transaction that replaced it")
	}
}

func TestArmN2SetupDoesNotOrphanAGuardAcrossEnd(t *testing.T) {
	cfg := guard.TimerValue{Enable: true, ExpireTime: 5 * time.Millisecond}

	for range 60 {
		conn := newTxnTestConn()
		conn.N2Setup(N2SetupPDUSession).Claim([]uint8{1})

		var wg sync.WaitGroup

		wg.Add(2)

		go func() {
			defer wg.Done()

			conn.N2Setup(N2SetupPDUSession).Arm(cfg)
		}()

		go func() {
			defer wg.Done()

			conn.EndN2Setup(N2SetupPDUSession)
		}()

		wg.Wait()

		conn.N2Setup(N2SetupPDUSession).Claim([]uint8{2})

		time.Sleep(12 * time.Millisecond)

		if !conn.N2SetupOpen(N2SetupPDUSession) {
			t.Fatal("an orphaned guard from the previous transaction closed the new one")
		}
	}
}

func TestAbortN2SetupsReleasesItsOwnClaims(t *testing.T) {
	conn := newTxnTestConn()
	ue := conn.UeContext()

	if err := ue.CreateSmContext(1, "ref-1", nil, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if err := ue.CreateSmContext(2, "ref-2", nil, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if got := conn.N2Setup(N2SetupInitialContext).Claim([]uint8{1}); len(got) != 1 {
		t.Fatalf("claimed %v, want PDU session 1", got)
	}

	if got := conn.N2Setup(N2SetupPDUSession).Claim([]uint8{2}); len(got) != 1 {
		t.Fatalf("claimed %v, want PDU session 2", got)
	}

	conn.AbortN2Setups()

	for _, id := range []uint8{1, 2} {
		if !conn.ClaimN2Session(N2SetupPDUSession, id) {
			t.Errorf("PDU session %d is still claimed after the transactions were aborted", id)
		}
	}
}

func TestAbortN2SetupsLeavesConfirmedSessionsAlone(t *testing.T) {
	conn := newTxnTestConn()
	ue := conn.UeContext()

	if err := ue.CreateSmContext(1, "ref-1", nil, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if got := conn.N2Setup(N2SetupPDUSession).Claim([]uint8{1}); len(got) != 1 {
		t.Fatalf("claimed %v, want PDU session 1", got)
	}

	conn.SetN2SessionActive(1)
	conn.AbortN2Setups()

	if conn.ClaimN2Session(N2SetupPDUSession, 1) {
		t.Error("a session the NG-RAN node confirmed must not be released by aborting a transaction")
	}
}

func TestClaimN2Setup(t *testing.T) {
	for _, tc := range []struct {
		name             string
		ueContextRequest bool
		carriesSessions  bool
		ics              ICSState
		wantProc         N2SetupProcedure
		wantInitial      bool
	}{
		{
			name:            "no UE Context Request IE, sessions to set up: the NG-RAN node needs the context first",
			carriesSessions: true,
			wantProc:        N2SetupInitialContext,
			wantInitial:     true,
		},
		{
			name:             "UE Context Request IE, nothing to carry",
			ueContextRequest: true,
			wantProc:         N2SetupInitialContext,
			wantInitial:      true,
		},
		{
			name:     "signalling only, no UE Context Request IE: no context is set up",
			wantProc: N2SetupPDUSession,
		},
		{
			name:            "context already set up: the standalone procedure carries the sessions",
			carriesSessions: true,
			ics:             ICSCompleted,
			wantProc:        N2SetupPDUSession,
		},
		{
			name:            "context setup already sent: the NG-RAN node holds it from that message on",
			carriesSessions: true,
			ics:             ICSPending,
			wantProc:        N2SetupPDUSession,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := newTxnTestConn()
			conn.UeContextRequest = tc.ueContextRequest
			conn.ics.Store(int32(tc.ics))

			proc, initial := conn.ClaimN2Setup(tc.carriesSessions)

			if proc != tc.wantProc {
				t.Errorf("procedure = %v, want %v", proc, tc.wantProc)
			}

			if initial != tc.wantInitial {
				t.Errorf("claimed the initial context setup = %v, want %v", initial, tc.wantInitial)
			}
		})
	}
}

func TestClaimN2SetupHandsTheInitialContextSetupToOneCaller(t *testing.T) {
	conn := newTxnTestConn()

	if _, initial := conn.ClaimN2Setup(true); !initial {
		t.Fatal("the first caller did not claim the initial context setup")
	}

	proc, initial := conn.ClaimN2Setup(true)
	if initial {
		t.Error("a second caller claimed the same initial context setup")
	}

	if proc != N2SetupPDUSession {
		t.Errorf("procedure = %v, want %v", proc, N2SetupPDUSession)
	}
}
