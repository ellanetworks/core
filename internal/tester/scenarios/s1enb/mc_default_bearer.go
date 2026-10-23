// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const mcDefaultBearerIMSI = "001017271246912"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/mc_default_bearer",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBMCDefaultBearer,
		Fixture: func(env scenarios.Env) scenarios.FixtureSpec {
			return scenarios.MCFixture(env, mcDefaultBearerIMSI)
		},
	})
}

func runS1ENBMCDefaultBearer(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(mcDefaultBearerIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())
	ue.RequestPCSCFAddresses()

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	expect := familyExpect(env, scenarios.MCDNN, scenarios.MCUEIPv4Pool)
	expect.QCI = scenarios.MCPolicy5QI
	expect.ARP = scenarios.MCPolicyARP

	if err := assertAttach(res, expect); err != nil {
		return err
	}

	if res.PreemptCap != s1ap.PreemptionMayTrigger || res.PreemptVuln != s1ap.PreemptionNotPreemptable {
		return fmt.Errorf("default E-RAB pre-emption capability %v, vulnerability %v; want may trigger, not pre-emptable", res.PreemptCap, res.PreemptVuln)
	}

	return scenarios.CheckPCSCFAddresses(res.PCSCF, scenarios.ExpectedPCSCFAddresses(env))
}
