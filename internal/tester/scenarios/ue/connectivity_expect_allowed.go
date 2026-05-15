package ue

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/connectivity_expect_allowed",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runConnectivityExpectAllowed(ctx, env, params)
		},
		Fixture: fixtureConnectivityExpectAllowed,
	})
}

func fixtureConnectivityExpectAllowed(env scenarios.Env) scenarios.FixtureSpec {
	subs := make([]scenarios.SubscriberSpec, numConnectivityParallel)

	for i := range numConnectivityParallel {
		imsi := incrementIMSI(connectivityStartIMSI, i)
		subs[i] = scenarios.DefaultSubscriberWith(imsi, "")
	}

	return scenarios.FixtureSpec{
		Subscribers: subs,
	}
}

func runConnectivityExpectAllowed(ctx context.Context, env scenarios.Env, _ any) error {
	subs, err := buildSubscribers(numConnectivityParallel, connectivityStartIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
	}

	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	eg := errgroup.Group{}

	for i := range numConnectivityParallel {
		func() {
			eg.Go(func() error {
				ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)
				tunInterfaceName := fmt.Sprintf(gtpInterfaceNamePrefix+"%d", i)

				return runConnectivityExpectAllowedTest(
					ctx,
					ranUENGAPID,
					gNodeB,
					subs[i],
					tunInterfaceName,
					env.PingDestination(),
					env.PDUSessionType(),
				)
			})
		}()
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("error during connectivity_expect_allowed test: %v", err)
	}

	return nil
}

func runConnectivityExpectAllowedTest(
	ctx context.Context,
	ranUENGAPID int64,
	gNodeB *gnb.GnodeB,
	sub subscriber,
	tunInterfaceName string,
	pingDestination string,
	pduSessionType uint8,
) error {
	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, pduSessionType)
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	uePDUSession, err := newUE.WaitForPDUSession(scenarios.DefaultPDUSessionID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for PDU session: %v", err)
	}

	uePduSession := newUE.GetPDUSession(scenarios.DefaultPDUSessionID)
	ueIP := uePduSession.UEIP + "/16"

	gnbPDUSession, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID), 5*time.Second)
	if err != nil {
		return fmt.Errorf("could not get PDU Session for RAN UE NGAP ID %d: %v", ranUENGAPID, err)
	}

	if _, err := gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              uePduSession.QFI,
	}); err != nil {
		return fmt.Errorf("could not create GTP tunnel (name: %s, DL TEID: %d): %v", tunInterfaceName, gnbPDUSession.DLTeid, err)
	}

	logger.GnbLogger.Debug(
		"Created GTP Tunnel for PDU Session",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("Interface", tunInterfaceName),
		zap.String("UE IP", ueIP),
	)

	cmd := exec.CommandContext(ctx, "ping", "-I", tunInterfaceName, pingDestination, "-c", "3", "-W", "1") // #nosec G204

	out, pingErr := cmd.CombinedOutput()
	if pingErr != nil {
		return fmt.Errorf("ping %s via %s failed, but was expected to succeed: %v\noutput:\n%s", pingDestination, tunInterfaceName, pingErr, string(out))
	}

	logger.Logger.Debug(
		"Ping succeeded as expected",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", pingDestination),
	)

	if err := gNodeB.CloseTunnel(gnbPDUSession.DLTeid); err != nil {
		return fmt.Errorf("could not close GTP tunnel: %v", err)
	}

	if err := procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
	}); err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
