package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/sctp",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runSCTPBasic,
	})
}

func runSCTPBasic(ctx context.Context, env scenarios.Env, _ any) error {
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

	fr, err := node.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		200*time.Millisecond,
	)
	if err != nil {
		return fmt.Errorf("wait for NG Setup Response: %w", err)
	}

	if err := testutil.ValidateSCTP(fr.Info, 60, 0); err != nil {
		return fmt.Errorf("SCTP validation: %w", err)
	}

	logger.Logger.Debug(
		"received SCTP frame",
		zap.Uint16("StreamIdentifier", fr.Info.Stream),
		zap.Uint32("PPID", fr.Info.PPID),
	)

	return nil
}
