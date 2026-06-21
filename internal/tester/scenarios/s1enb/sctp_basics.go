// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const sctpBasicsIMSI = "001017271246609"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/sctp",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBSCTPBasics,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(sctpBasicsIMSI, "")},
			}
		},
	})
}

// runS1ENBSCTPBasics asserts the MME's S1AP SCTP transport parameters (TS 36.412
// §7): PPID 18 on every PDU, stream 0 for non-UE-associated signalling (S1 Setup
// Response), and stream 1 for UE-associated signalling (UE Context Release Command).
func runS1ENBSCTPBasics(_ context.Context, env scenarios.Env, _ any) error {
	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	if err := testutil.ValidateSCTP(e.S1SetupResponseInfo(), 18, 0); err != nil {
		return fmt.Errorf("S1 Setup Response SCTP: %w", err)
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	ue := e.NewUE(sctpBasicsIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if err := e.SendUEContextReleaseRequest(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity); err != nil {
		return fmt.Errorf("send UE Context Release Request: %w", err)
	}

	fr, err := e.WaitForMessage(res.ENBUES1APID, s1enb.Initiating, s1ap.ProcUEContextRelease, 10*time.Second)
	if err != nil {
		return fmt.Errorf("await UE Context Release Command: %w", err)
	}

	if err := testutil.ValidateSCTP(fr.Info, 18, 1); err != nil {
		return fmt.Errorf("UE Context Release Command SCTP: %w", err)
	}

	cmd, err := s1ap.ParseUEContextReleaseCommand(fr.Value)
	if err != nil {
		return fmt.Errorf("parse UE Context Release Command: %w", err)
	}

	return e.SendUEContextReleaseComplete(int64(cmd.UES1APIDs.MMEUES1APID), int64(cmd.UES1APIDs.ENBUES1APID))
}
