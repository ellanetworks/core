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
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/ngap/configuration_update",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runRANConfigurationUpdate,
	})
}

// runRANConfigurationUpdate exercises both outcomes of TS 38.413 §8.7.2: an
// update the AMF can take into use is acknowledged, and one whose Supported TA
// List names no served TAC is refused with a Failure.
func runRANConfigurationUpdate(_ context.Context, env scenarios.Env, _ any) error {
	g := env.FirstGNB()

	node, err := gnb.Start(&gnb.StartOpts{
		GnbID:           fmt.Sprintf("%06x", 1),
		MCC:             scenarios.DefaultMCC,
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

	if _, err := node.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		200*time.Millisecond,
	); err != nil {
		return fmt.Errorf("wait NGSetupResponse: %w", err)
	}

	// A name-only update carries no Supported TA List, which §8.7.2.2 leaves
	// unchanged, so the AMF has nothing to refuse.
	if err := node.SendRANConfigurationUpdate(&gnb.RANConfigurationUpdateOpts{Name: "Ella-Core-Tester-renamed"}); err != nil {
		return fmt.Errorf("send name-only RANConfigurationUpdate: %w", err)
	}

	frame, err := node.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentRANConfigurationUpdateAcknowledge,
		200*time.Millisecond,
	)
	if err != nil {
		return fmt.Errorf("wait RANConfigurationUpdateAcknowledge: %w", err)
	}

	if err := testutil.ValidateSCTP(frame.Info, 60, 0); err != nil {
		return fmt.Errorf("SCTP validation: %w", err)
	}

	// An update whose Supported TA List names a TAC the AMF does not serve is
	// refused (§8.7.2.3).
	if err := node.SendRANConfigurationUpdate(&gnb.RANConfigurationUpdateOpts{
		Mcc: scenarios.DefaultMCC,
		Mnc: scenarios.DefaultMNC,
		Tac: "ffffff",
		Sst: scenarios.DefaultSST,
		Sd:  scenarios.DefaultSD,
	}); err != nil {
		return fmt.Errorf("send unserved-TAC RANConfigurationUpdate: %w", err)
	}

	if _, err := node.WaitForMessage(
		ngapType.NGAPPDUPresentUnsuccessfulOutcome,
		ngapType.UnsuccessfulOutcomePresentRANConfigurationUpdateFailure,
		200*time.Millisecond,
	); err != nil {
		return fmt.Errorf("wait RANConfigurationUpdateFailure: %w", err)
	}

	return nil
}
