// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const incorrectGUTIIMSI = "001017271246603"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/registration/incorrect_guti",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBRegistrationIncorrectGUTI,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(incorrectGUTIIMSI, "")},
			}
		},
	})
}

// runS1ENBRegistrationIncorrectGUTI attaches presenting a GUTI the MME cannot
// resolve and verifies the MME requests the IMSI with an Identity Request, then
// completes the attach (TS 24.301 §5.4.4) — the 4G counterpart of
// gnb/registration/incorrect_guti.
func runS1ENBRegistrationIncorrectGUTI(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(incorrectGUTIIMSI, k, opc)
	ue.UseUnknownGUTI()

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if !res.IdentityRequested {
		return fmt.Errorf("MME did not request the IMSI for an unresolvable GUTI")
	}

	if res.GUTI == nil {
		return fmt.Errorf("attach completed without a GUTI")
	}

	return nil
}
