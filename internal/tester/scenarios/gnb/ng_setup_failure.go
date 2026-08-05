// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/ngap/setup_failure/unknown_plmn",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runNGSetupFailureUnknownPLMN,
	})
}

func runNGSetupFailureUnknownPLMN(_ context.Context, env scenarios.Env, _ any) error {
	g := env.FirstGNB()

	node, err := gnb.Start(&gnb.StartOpts{
		GnbID:           fmt.Sprintf("%06x", 1),
		MCC:             "002", // Unknown PLMN to trigger failure.
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    "0.0.0.0",
	})
	if err != nil {
		return fmt.Errorf("start gNB: %w", err)
	}

	defer node.Close()

	frame, err := node.WaitForMessage(
		gnb.Unsuccessful,
		ngap.ProcNGSetup,
		200*time.Millisecond,
	)
	if err != nil {
		return fmt.Errorf("wait NGSetupFailure: %w", err)
	}

	if err := testutil.ValidateSCTP(frame.Info, 60, 0); err != nil {
		return fmt.Errorf("SCTP validation: %w", err)
	}

	failure, err := ngap.ParseNGSetupFailure(frame.Value)
	if err != nil {
		return fmt.Errorf("parse NGSetupFailure: %w", err)
	}

	if failure.Cause == nil {
		return fmt.Errorf("NG Setup Failure carried no cause")
	}

	// TS 38.455 gives Misc one root value, unknown-PLMN-or-SNPN, at index 4
	// (TS 38.413 §9.3.1.2).
	want := ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscUnknownPLMNOrSNPN}
	if *failure.Cause != want {
		return fmt.Errorf("expected cause Misc unknown-PLMN-or-SNPN, got %v", failure.Cause)
	}

	return nil
}
