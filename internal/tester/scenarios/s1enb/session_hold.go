// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const sessionHoldIMSI = "001017271246591"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/session_hold",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBSessionHold,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(sessionHoldIMSI, "")},
			}
		},
	})
}

// runS1ENBSessionHold attaches a single UE over S1AP, establishing the default
// EPS bearer (and its IP lease), and blocks until ctx is cancelled so external
// tests can observe the BGP route advertisement before tear-down. The 4G
// counterpart of gnb/session_hold. It honours the env IP family so the BGP suite
// can hold an IPv6 lease (and advertise its route) in IPv6 mode.
func runS1ENBSessionHold(ctx context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(sessionHoldIMSI, k, opc)

	ipv6 := env.IPFamily() == scenarios.IPv6Only
	if ipv6 {
		ue.RequestPDNType(eps.PDNTypeIPv6)
	}

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	ueIP := res.UEIPv4
	if ipv6 {
		ueIP = res.UEIPv6
	}

	logger.Logger.Info("session established, holding until cancelled",
		zap.String("IMSI", sessionHoldIMSI),
		zap.String("UE IP", ueIP),
	)

	<-ctx.Done()

	logger.Logger.Info("context cancelled, tearing down session", zap.String("IMSI", sessionHoldIMSI))

	if err := e.Detach(ue, res.MMEUES1APID, res.ENBUES1APID, 10*time.Second); err != nil {
		logger.Logger.Warn("detach failed during teardown", zap.Error(err))
	}

	return nil
}
