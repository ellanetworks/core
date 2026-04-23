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
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	gtpInterfaceNamePrefix  = "ellatester"
	numConnectivityParallel = 5
	connectivityStartIMSI   = "001017271246546"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/connectivity",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runConnectivity(ctx, env, params)
		},
	})
}

func runConnectivity(ctx context.Context, env scenarios.Env, _ any) error {
	subs, err := buildSubscribers(numConnectivityParallel, connectivityStartIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
	}

	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         scenarios.DefaultGNBID,
		MCC:           scenarios.DefaultMCC,
		MNC:           scenarios.DefaultMNC,
		SST:           scenarios.DefaultSST,
		SD:            scenarios.DefaultSD,
		DNN:           scenarios.DefaultDNN,
		TAC:           scenarios.DefaultTAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: env.FirstCore(),
		GnbN2Address:  g.N2Address,
		GnbN3Address:  g.N3Address,
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

				return runConnectivityTest(
					ctx,
					ranUENGAPID,
					gNodeB,
					subs[i],
					tunInterfaceName,
				)
			})
		}()
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("error during connectivity test: %v", err)
	}

	return nil
}

func runConnectivityTest(
	ctx context.Context,
	ranUENGAPID int64,
	gNodeB *gnb.GnodeB,
	sub subscriber,
	tunInterfaceName string,
) error {
	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber)
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	ueAmbr := gNodeB.GetUEAmbr(ranUENGAPID)

	err = validate.UEAmbr(ueAmbr, &validate.ExpectedUEAmbr{
		UplinkBps:   100_000_000,
		DownlinkBps: 100_000_000,
	})
	if err != nil {
		return fmt.Errorf("UE AMBR validation failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed Initial Registration Procedure",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(ranUENGAPID)),
	)

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

	err = validate.PDUSessionInformation(gnbPDUSession, &validate.ExpectedPDUSessionInformation{
		FiveQi: 9,
		PriArp: 15,
		QFI:    1,
	})
	if err != nil {
		return fmt.Errorf("NGAP QoS validation failed: %v", err)
	}

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              uePduSession.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel (name: %s, DL TEID: %d): %v", tunInterfaceName, gnbPDUSession.DLTeid, err)
	}

	logger.GnbLogger.Debug(
		"Created GTP Tunnel for PDU Session",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("Interface", tunInterfaceName),
		zap.String("UE IP", ueIP),
		zap.String("UPF IP", gnbPDUSession.UpfAddress),
		zap.Uint32("UL TEID", gnbPDUSession.ULTeid),
		zap.Uint32("DL TEID", gnbPDUSession.DLTeid),
	)

	cmd := exec.CommandContext(ctx, "ping", "-I", tunInterfaceName, scenarios.DefaultPingDestination, "-c", "3", "-W", "1") // #nosec G204

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping %s via %s failed after initial registration: %v\noutput:\n%s", scenarios.DefaultPingDestination, tunInterfaceName, err, string(out))
	}

	logger.Logger.Debug(
		"Ping successful",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", scenarios.DefaultPingDestination),
	)

	pduSessionStatus := [16]bool{}
	pduSessionStatus[scenarios.DefaultPDUSessionID] = true

	err = procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionStatus,
	})
	if err != nil {
		return fmt.Errorf("UEContextReleaseProcedure failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed UE Context Release Procedure",
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(ranUENGAPID)),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	cmd = exec.CommandContext(ctx, "ping", "-I", tunInterfaceName, scenarios.DefaultPingDestination, "-c", "3", "-W", "1") // #nosec G204

	out, err = cmd.CombinedOutput()
	if err == nil {
		return fmt.Errorf("ping %s via %s succeeded, but was expected to fail after UE Context Release\noutput:\n%s", scenarios.DefaultPingDestination, tunInterfaceName, string(out))
	}

	logger.Logger.Debug(
		"Ping failed as expected after UE Context Release",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", scenarios.DefaultPingDestination),
	)

	err = procedure.ServiceRequest(&procedure.ServiceRequestOpts{
		PDUSessionStatus: pduSessionStatus,
		RANUENGAPID:      ranUENGAPID,
		UE:               newUE,
	})
	if err != nil {
		return fmt.Errorf("service request procedure failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed Service Request Procedure",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(ranUENGAPID)),
	)

	err = gNodeB.CloseTunnel(gnbPDUSession.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close GTP tunnel: %v", err)
	}

	pduSession := gNodeB.GetPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID))

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            pduSession.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULteid:           pduSession.ULTeid,
		DLteid:           pduSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              uePduSession.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel after service request (name: %s, DL TEID: %d): %v", tunInterfaceName, pduSession.DLTeid, err)
	}

	logger.GnbLogger.Debug(
		"Created GTP Tunnel for PDU Session after Service Request",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("Interface", tunInterfaceName),
		zap.String("UE IP", ueIP),
		zap.String("UPF IP", pduSession.UpfAddress),
		zap.Uint32("UL TEID", pduSession.ULTeid),
		zap.Uint32("DL TEID", pduSession.DLTeid),
	)

	cmd = exec.CommandContext(ctx, "ping", "-I", tunInterfaceName, scenarios.DefaultPingDestination, "-c", "3", "-W", "1") // #nosec G204

	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping %s via %s failed after service request: %v\noutput:\n%s", scenarios.DefaultPingDestination, tunInterfaceName, err, string(out))
	}

	logger.Logger.Debug(
		"Ping successful after Service Request",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", scenarios.DefaultPingDestination),
	)

	// NOTE: client-side usage assertion skipped; integration test will verify.
	logger.Logger.Debug("client-side usage assertion skipped; integration test will verify",
		zap.String("IMSI", sub.IMSI),
	)

	err = gNodeB.CloseTunnel(pduSession.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close GTP tunnel: %v", err)
	}

	logger.Logger.Debug(
		"Closed GTP tunnel",
		zap.String("interface", tunInterfaceName),
	)

	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
