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

const macFailureIMSI = "001017271246602"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/authentication/mac_failure",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBAuthenticationMACFailure,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(macFailureIMSI, "")},
			}
		},
	})
}

func runS1ENBAuthenticationMACFailure(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	// Flip the key so the derived RES cannot match the network's XRES.
	k[0] ^= 0xff

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(macFailureIMSI, k, opc)

	if err := e.AttachExpectAuthReject(ue, 15*time.Second); err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	return nil
}
