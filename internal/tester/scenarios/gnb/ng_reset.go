// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	ngaplib "github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/ngap/reset",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runNGReset,
	})
}

func runNGReset(_ context.Context, env scenarios.Env, _ any) error {
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
		gnb.Successful,
		ngaplib.ProcNGSetup,
		200*time.Millisecond,
	); err != nil {
		return fmt.Errorf("wait NGSetupResponse: %w", err)
	}

	if err := node.SendNGReset(&gnb.NGResetOpts{
		Cause: ngaplib.Ptr(ngaplib.Cause{
			Group: ngaplib.CauseGroupMisc, Value: ngaplib.CauseMiscUnspecified,
		}),
		ResetAll: true,
	}); err != nil {
		return fmt.Errorf("send NGReset: %w", err)
	}

	logger.Logger.Debug("sent NGReset", zap.String("Cause", "unspecified"), zap.Bool("ResetAll", true))

	frame, err := node.WaitForMessage(
		gnb.Successful,
		ngaplib.ProcNGReset,
		200*time.Millisecond,
	)
	if err != nil {
		return fmt.Errorf("wait NGResetAcknowledge: %w", err)
	}

	if err := testutil.ValidateSCTP(frame.Info, 60, 0); err != nil {
		return fmt.Errorf("SCTP validation: %w", err)
	}

	if _, err := ngaplib.ParseNGResetAcknowledge(frame.Value); err != nil {
		return fmt.Errorf("parse NGResetAcknowledge: %w", err)
	}

	return nil
}
