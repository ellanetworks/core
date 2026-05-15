package ue

import (
	"context"
	"fmt"
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
		Name: "ue/connectivity_expect_blocked_ipv6",
		BindFlags: func(fs *pflag.FlagSet) any {
			return bindConnectivityProbeFlags(fs)
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runConnectivityExpectBlockedIPv6(ctx, env, params.(*connectivityProbeParams))
		},
		Fixture: fixtureConnectivityExpectBlockedIPv6,
	})
}

func fixtureConnectivityExpectBlockedIPv6(env scenarios.Env) scenarios.FixtureSpec {
	subs := make([]scenarios.SubscriberSpec, numConnectivityParallel)

	for i := range numConnectivityParallel {
		imsi := incrementIMSI(ipv6StartIMSI, i)
		subs[i] = scenarios.DefaultSubscriberWith(imsi, "")
	}

	return scenarios.FixtureSpec{
		Subscribers: subs,
	}
}

func runConnectivityExpectBlockedIPv6(ctx context.Context, env scenarios.Env, params *connectivityProbeParams) error {
	protocol, err := parseConnectivityProbeProtocol(params.Protocol)
	if err != nil {
		return err
	}

	subs, err := buildSubscribers(numConnectivityParallel, ipv6StartIMSI)
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
				tunInterfaceName := fmt.Sprintf(gtpInterfaceNamePrefix+"v6%d", i)

				return runConnectivityExpectBlockedIPv6Test(
					ctx,
					ranUENGAPID,
					gNodeB,
					subs[i],
					tunInterfaceName,
					env.PDUSessionType(),
					protocol,
				)
			})
		}()
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("error during connectivity_expect_blocked_ipv6 test: %v", err)
	}

	return nil
}

func runConnectivityExpectBlockedIPv6Test(
	ctx context.Context,
	ranUENGAPID int64,
	gNodeB *gnb.GnodeB,
	sub subscriber,
	tunInterfaceName string,
	pduSessionType uint8,
	protocol connectivityProbeProtocol,
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
	ueIP := uePduSession.UEIPV6 + "/64"

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
		"Created GTP Tunnel for PDU Session (IPv6)",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("Interface", tunInterfaceName),
		zap.String("UE IP", ueIP),
	)

	if err := gnb.WaitForULAAddr(tunInterfaceName, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
		return fmt.Errorf("timeout waiting for ULA address on %s: %v", tunInterfaceName, err)
	}

	if err := runConnectivityProbe(ctx, protocol, tunInterfaceName, scenarios.DefaultPingDestinationV6, true); err == nil {
		return fmt.Errorf("%s probe to %s via %s succeeded, but was expected to fail (deny rule should be in force)", protocol, scenarios.DefaultPingDestinationV6, tunInterfaceName)
	}

	logger.Logger.Debug(
		"Probe failed as expected (IPv6, traffic blocked by network rule)",
		zap.String("protocol", string(protocol)),
		zap.String("interface", tunInterfaceName),
		zap.String("destination", scenarios.DefaultPingDestinationV6),
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
