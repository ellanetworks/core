// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package inprocess_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/testenv"
	testerlogger "github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
	"github.com/ellanetworks/core/pkg/runtime/runtimetest"
	"github.com/spf13/pflag"
	"go.uber.org/zap/zapcore"
)

const scenarioTimeout = 3 * time.Minute

var initTesterLogger sync.Once

func TestScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("boots one core per scenario")
	}

	initTesterLogger.Do(func() { testerlogger.Init(zapcore.WarnLevel) })

	for i, name := range scenarios.ListTier(scenarios.TierInProcess) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runScenario(t, name, i)
		})
	}
}

func runScenario(t *testing.T, name string, index int) {
	sc, ok := scenarios.Get(name)
	if !ok {
		t.Fatalf("scenario %q not registered", name)
	}

	t0 := time.Now()
	core := runtimetest.Start(t, runtimetest.Options{})
	ctx := context.Background()
	tBoot := time.Since(t0)

	baseline := fixture.New(t, ctx, core.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	family := scenarios.IPv4Only
	if sc.OnlyFamily != nil {
		family = *sc.OnlyFamily
	}

	env := scenarios.Env{
		CoreN2Addresses: []string{core.N2Address},
		CoreS1Addresses: []string{core.S1Address},
		GNBs:            radios(index),
		APIAddress:      core.APIAddress,
		APIToken:        core.APIToken,
		Family:          &family,
	}

	var spec scenarios.FixtureSpec
	if sc.Fixture != nil {
		spec = sc.Fixture(env)
	}

	if len(spec.AssertUsageForIMSIs) > 0 {
		t.Fatalf("%s asserts on user-plane usage; it needs the datapath and belongs in the compose tier", name)
	}

	fixture.New(t, ctx, core.Client).Apply(spec)

	fs := pflag.NewFlagSet(name, pflag.ContinueOnError)
	params := sc.BindFlags(fs)

	args := append([]string{}, spec.ExtraArgs...)
	if fs.Lookup("ella-api-address") != nil {
		args = append(args, "--ella-api-address", core.APIAddress)
	}

	if fs.Lookup("ella-api-token") != nil {
		args = append(args, "--ella-api-token", core.APIToken)
	}

	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse scenario flags: %v", err)
	}

	tFixture := time.Since(t0) - tBoot
	t1 := time.Now()

	defer func() {
		t.Logf("TIMING boot=%.2f fixture=%.2f run=%.2f", tBoot.Seconds(), tFixture.Seconds(), time.Since(t1).Seconds())
	}()

	timeout := testenv.Timeout(scenarioTimeout)

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errc := make(chan error, 1)

	go func() { errc <- sc.Run(runCtx, env, params) }()

	select {
	case err := <-errc:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(timeout + 30*time.Second):
		t.Fatalf("%s did not return within %s", name, timeout)
	}
}

func radios(index int) []scenarios.GNB {
	block := fmt.Sprintf("127.%d.%d", 10+index/250, index%250)

	return []scenarios.GNB{
		{Name: "gnb1", N2Address: block + ".1", N3Address: block + ".1", N3Secondary: block + ".2"},
		{Name: "gnb2", N2Address: block + ".11", N3Address: block + ".11", N3Secondary: block + ".12"},
	}
}
