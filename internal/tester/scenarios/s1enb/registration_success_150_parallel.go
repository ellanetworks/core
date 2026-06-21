// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

const (
	scaleParallelCount    = 150
	scaleParallelBaseIMSI = "001017271247000"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/registration_success_150_parallel",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBScaleParallel,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			subs := make([]scenarios.SubscriberSpec, scaleParallelCount)
			for i := range subs {
				subs[i] = scenarios.DefaultSubscriberWith(nthIMSI(scaleParallelBaseIMSI, i), "")
			}

			return scenarios.FixtureSpec{Subscribers: subs}
		},
	})
}

// runS1ENBScaleParallel attaches a batch of UEs concurrently on one eNB,
// verifying the MME completes every EPS attach and hands out a distinct GUTI
// under simultaneous load. The eNB sim demultiplexes downlink frames by
// eNB-UE-S1AP-ID, so each attach consumes only its own messages.
func runS1ENBScaleParallel(ctx context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	var (
		mu   sync.Mutex
		seen = make(map[uint32]string, scaleParallelCount)
	)

	eg, _ := errgroup.WithContext(ctx)

	for i := range scaleParallelCount {
		imsi := nthIMSI(scaleParallelBaseIMSI, i)

		eg.Go(func() error {
			ue := e.NewUE(imsi, k, opc)
			ue.RequestPDNType(env.PDUSessionType())

			res, err := e.Attach(ue, 30*time.Second)
			if err != nil {
				return fmt.Errorf("attach (imsi %s): %w", imsi, err)
			}

			if res.GUTI == nil {
				return fmt.Errorf("attach (imsi %s) completed without a GUTI", imsi)
			}

			mu.Lock()
			defer mu.Unlock()

			if prev, dup := seen[res.GUTI.MTMSI]; dup {
				return fmt.Errorf("MME reused M-TMSI %#x for %s and %s", res.GUTI.MTMSI, prev, imsi)
			}

			seen[res.GUTI.MTMSI] = imsi

			return nil
		})
	}

	return eg.Wait()
}
