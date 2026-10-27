// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

var (
	imsSMSSender   = scenarios.IMSSubscriber("001018400000011", "+15558400011")
	imsSMSReceiver = scenarios.IMSSubscriber("001018400000012", "+15558400012")
)

type imsSMSParams struct {
	ServiceCentre string
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "ims-sms/5g_sms_over_ip",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &imsSMSParams{}
			fs.StringVar(&p.ServiceCentre, "service-centre", "+15550000000", "the SMSC's service centre address")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, raw any) error {
			p, _ := raw.(*imsSMSParams)
			return runIMSSMS(ctx, env, p.ServiceCentre)
		},
		Fixture: func(env scenarios.Env) scenarios.FixtureSpec {
			return scenarios.IMSFixture(env, imsSMSSender, imsSMSReceiver)
		},
	})
}

func runIMSSMS(ctx context.Context, env scenarios.Env, serviceCentre string) error {
	unprovision, err := scenarios.ProvisionIPSMGWService(ctx, env, scenarios.IMSProfileName)
	if err != nil {
		return err
	}

	defer unprovision()

	g, err := startGNB(env)
	if err != nil {
		return err
	}

	defer g.Close()

	endpoints, _, cleanup, err := attachIMSCallUEs(env, g, imsSMSSender, imsSMSReceiver)
	if err != nil {
		return err
	}

	defer cleanup()

	for _, transport := range scenarios.IMSTransports {
		if err := scenarios.RequireIMSSMS(ctx, endpoints[0], endpoints[1], transport, serviceCentre); err != nil {
			return err
		}
	}

	return nil
}
