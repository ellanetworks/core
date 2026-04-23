package enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

const defaultEnbID = "000008"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "enb/ng_setup",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runNgSetup,
	})
}

func runNgSetup(_ context.Context, env scenarios.Env, _ any) error {
	g := env.FirstGNB()

	node, err := enb.Start(
		defaultEnbID,
		scenarios.DefaultMCC,
		scenarios.DefaultMNC,
		scenarios.DefaultSST,
		scenarios.DefaultSD,
		scenarios.DefaultDNN,
		scenarios.DefaultTAC,
		"Ella-Core-Tester-ENB",
		env.FirstCore(),
		g.N2Address,
		"",
	)
	if err != nil {
		return fmt.Errorf("start ng-eNB: %w", err)
	}

	defer node.Close()

	if _, err := node.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		1*time.Second,
	); err != nil {
		return fmt.Errorf("wait NGSetupResponse: %w", err)
	}

	return nil
}
