// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/s1_setup_failure/unknown_plmn",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBSetupFailureUnknownPLMN,
	})
}

// runS1ENBSetupFailureUnknownPLMN sends an S1 Setup Request advertising a PLMN
// the core does not serve and verifies the MME answers with an S1 SETUP FAILURE
// carrying cause Misc "unknown-PLMN" (TS 36.413 §8.7.3.4).
func runS1ENBSetupFailureUnknownPLMN(_ context.Context, env scenarios.Env, _ any) error {
	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	g := env.FirstGNB()

	e, err := s1enb.Start(&s1enb.StartOpts{
		ENBID:            uint32(enbID),
		MCC:              "002", // unknown PLMN to trigger the failure
		MNC:              scenarios.DefaultMNC,
		TAC:              scenarios.DefaultTAC,
		Name:             "Ella-Core-Tester-S1eNB",
		CoreS1MMEAddress: s1mme,
		ENBAddress:       g.N2Address,
		ENBN3Address:     g.N3Address,
		SkipS1SetupWait:  true,
	})
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	fail, err := e.WaitForS1SetupFailure(5 * time.Second)
	if err != nil {
		return fmt.Errorf("await S1 Setup Failure: %w", err)
	}

	// TS 36.413: Cause Misc "unknown-PLMN" is the sixth Misc root value.
	if fail.Cause.Group != s1ap.CauseGroupMisc || fail.Cause.Value != 5 {
		return fmt.Errorf("expected cause Misc unknown-PLMN, got group %d value %d", fail.Cause.Group, fail.Cause.Value)
	}

	return nil
}
