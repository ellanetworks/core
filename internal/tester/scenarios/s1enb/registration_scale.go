// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	scaleSequentialCount    = 50
	scaleSequentialBaseIMSI = "001017271246620"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/registration_success_50_sequential",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBScaleSequential,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			subs := make([]scenarios.SubscriberSpec, scaleSequentialCount)
			for i := range subs {
				subs[i] = scenarios.DefaultSubscriberWith(nthIMSI(scaleSequentialBaseIMSI, i), "")
			}

			return scenarios.FixtureSpec{Subscribers: subs}
		},
	})
}

// runS1ENBScaleSequential attaches a batch of UEs back-to-back on one eNB,
// verifying the MME completes every EPS attach and hands out a distinct GUTI.
func runS1ENBScaleSequential(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	seen := make(map[uint32]string, scaleSequentialCount)

	for i := range scaleSequentialCount {
		imsi := nthIMSI(scaleSequentialBaseIMSI, i)

		ue := e.NewUE(imsi, k, opc)
		ue.RequestPDNType(env.PDUSessionType())

		res, err := e.Attach(ue, 15*time.Second)
		if err != nil {
			return fmt.Errorf("attach %d/%d (imsi %s): %w", i+1, scaleSequentialCount, imsi, err)
		}

		if res.GUTI == nil {
			return fmt.Errorf("attach %d/%d (imsi %s) completed without a GUTI", i+1, scaleSequentialCount, imsi)
		}

		if prev, dup := seen[res.GUTI.MTMSI]; dup {
			return fmt.Errorf("MME reused M-TMSI %#x for %s and %s", res.GUTI.MTMSI, prev, imsi)
		}

		seen[res.GUTI.MTMSI] = imsi
	}

	return nil
}

func nthIMSI(base string, n int) string {
	v, err := strconv.ParseUint(base, 10, 64)
	if err != nil {
		return base
	}

	return fmt.Sprintf("%015d", v+uint64(n))
}
