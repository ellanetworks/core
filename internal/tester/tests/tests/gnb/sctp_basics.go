package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/tests/engine"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/core"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

type SCTPBasic struct{}

func (SCTPBasic) Meta() engine.Meta {
	return engine.Meta{
		ID:      "gnb/sctp",
		Summary: "SCTP connectivity test validating SCTP Stream Identifier and PPID for NGSetup procedure",
		Timeout: 2 * time.Second,
	}
}

func (t SCTPBasic) Run(ctx context.Context, env engine.Env) error {
	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, getDefaultEllaCoreConfig())

	err := ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Created EllaCore environment")

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         fmt.Sprintf("%06x", 1),
		MCC:           DefaultMCC,
		MNC:           DefaultMNC,
		SST:           DefaultSST,
		SD:            DefaultSD,
		DNN:           DefaultDNN,
		TAC:           DefaultTAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: env.Config.EllaCore.N2Address,
		GnbN2Address:  env.Config.Gnb.N2Address,
		GnbN3Address:  "0.0.0.0",
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	fr, err := gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	err = utils.ValidateSCTP(fr.Info, 60, 0)
	if err != nil {
		return fmt.Errorf("SCTP validation failed: %v", err)
	}

	logger.Logger.Debug(
		"Received SCTP frame",
		zap.Uint16("StreamIdentifier", fr.Info.Stream),
		zap.Uint32("PPID", fr.Info.PPID),
	)

	// Cleanup
	err = ellaCoreEnv.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not delete EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Deleted EllaCore environment")

	return nil
}
