// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const ctxReleaseIMSI = "001017271246603"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/context_release",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBContextRelease,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(ctxReleaseIMSI, "")},
			}
		},
	})
}

// runS1ENBContextRelease attaches a UE, has the eNB request an S1 release on
// inactivity, and validates the MME's UE CONTEXT RELEASE COMMAND: a radioNetwork
// user-inactivity Cause and the paired UE-S1AP-IDs (TS 36.413 §8.3.3).
func runS1ENBContextRelease(_ context.Context, env scenarios.Env, _ any) error {
	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	ue := e.NewUE(ctxReleaseIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if err := e.SendUEContextReleaseRequest(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity); err != nil {
		return fmt.Errorf("send UE Context Release Request: %w", err)
	}

	// A returned command already proves the procedure code (S1AP UEContextRelease).
	cmd, err := e.WaitForUEContextReleaseCommand(res.ENBUES1APID, 10*time.Second)
	if err != nil {
		return fmt.Errorf("await UE Context Release Command: %w", err)
	}

	if cmd.Cause != s1enb.CauseUserInactivity {
		return fmt.Errorf("UE Context Release Command cause = %+v, want radioNetwork user-inactivity %+v", cmd.Cause, s1enb.CauseUserInactivity)
	}

	if !cmd.UES1APIDs.Pair {
		return fmt.Errorf("UE Context Release Command carried a single UE-S1AP-ID, want the MME+eNB pair")
	}

	if got := int64(cmd.UES1APIDs.MMEUES1APID); got != res.MMEUES1APID {
		return fmt.Errorf("UE Context Release Command MME-UE-S1AP-ID = %d, want %d", got, res.MMEUES1APID)
	}

	if got := int64(cmd.UES1APIDs.ENBUES1APID); got != res.ENBUES1APID {
		return fmt.Errorf("UE Context Release Command eNB-UE-S1AP-ID = %d, want %d", got, res.ENBUES1APID)
	}

	return e.SendUEContextReleaseComplete(int64(cmd.UES1APIDs.MMEUES1APID), int64(cmd.UES1APIDs.ENBUES1APID))
}
