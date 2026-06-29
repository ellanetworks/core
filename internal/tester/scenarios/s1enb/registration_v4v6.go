// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
)

const v4v6IMSI = "001017271246605"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/registration/v4v6",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBRegistrationV4V6,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(v4v6IMSI, "")},
			}
		},
	})
}

func runS1ENBRegistrationV4V6(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(v4v6IMSI, k, opc)
	ue.RequestPDNType(eps.PDNTypeIPv4v6)

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	exp := defaultExpectedAttach()
	exp.PDNType = eps.PDNTypeIPv4v6

	return assertAttach(res, exp)
}
