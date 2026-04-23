package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
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
		GnbID:         fmt.Sprintf("%06x", 1),
		MCC:           scenarios.DefaultMCC,
		MNC:           scenarios.DefaultMNC,
		SST:           scenarios.DefaultSST,
		SD:            scenarios.DefaultSD,
		DNN:           scenarios.DefaultDNN,
		TAC:           scenarios.DefaultTAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: env.FirstCore(),
		GnbN2Address:  g.N2Address,
		GnbN3Address:  "0.0.0.0",
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

	if err := node.SendNGReset(&gnb.NGResetOpts{
		Cause: &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc: &ngapType.CauseMisc{
				Value: ngapType.CauseMiscPresentUnspecified,
			},
		},
		ResetAll: true,
	}); err != nil {
		return fmt.Errorf("send NGReset: %w", err)
	}

	logger.Logger.Debug("sent NGReset", zap.String("Cause", "unspecified"), zap.Bool("ResetAll", true))

	frame, err := node.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGResetAcknowledge,
		200*time.Millisecond,
	)
	if err != nil {
		return fmt.Errorf("wait NGResetAcknowledge: %w", err)
	}

	if err := testutil.ValidateSCTP(frame.Info, 60, 0); err != nil {
		return fmt.Errorf("SCTP validation: %w", err)
	}

	pdu, err := ngap.Decoder(frame.Data)
	if err != nil {
		return fmt.Errorf("decode NGAP: %w", err)
	}

	if pdu.SuccessfulOutcome == nil {
		return fmt.Errorf("NGAP PDU is not SuccessfulOutcome")
	}

	if pdu.SuccessfulOutcome.ProcedureCode.Value != ngapType.ProcedureCodeNGReset {
		return fmt.Errorf("NGAP ProcedureCode is not NGReset (%d)", ngapType.ProcedureCodeNGReset)
	}

	if pdu.SuccessfulOutcome.Value.NGResetAcknowledge == nil {
		return fmt.Errorf("NGResetAcknowledge is nil")
	}

	return nil
}
