package enb

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/enb"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	enbGTPInterfaceNamePrefix  = "ellatesterenb"
	enbNumConnectivityParallel = 5
	enbTestStartIMSI           = "001017271246546"
)

// enbSubscriber captures the fields the connectivity scenario needs when
// building per-UE inputs. It mirrors the old SubscriberConfig subset.
type enbSubscriber struct {
	IMSI           string
	Key            string
	OPc            string
	SequenceNumber string
}

func buildEnbSubscribers(numSubscribers int, startIMSI string) ([]enbSubscriber, error) {
	subs := make([]enbSubscriber, 0, numSubscribers)

	for i := range numSubscribers {
		intBaseIMSI, err := strconv.Atoi(startIMSI)
		if err != nil {
			return nil, fmt.Errorf("failed to convert base IMSI to int: %v", err)
		}

		newIMSI := intBaseIMSI + i
		imsi := fmt.Sprintf("%015d", newIMSI)

		subs = append(subs, enbSubscriber{
			IMSI:           imsi,
			Key:            scenarios.DefaultKey,
			OPc:            scenarios.DefaultOPC,
			SequenceNumber: scenarios.DefaultSequenceNumber,
		})
	}

	return subs, nil
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "enb/connectivity",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runEnbConnectivity(ctx, env, params)
		},
	})
}

func runEnbConnectivity(ctx context.Context, env scenarios.Env, _ any) error {
	subs, err := buildEnbSubscribers(enbNumConnectivityParallel, enbTestStartIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
	}

	g := env.FirstGNB()

	ngeNB, err := enb.Start(
		scenarios.DefaultGNBID,
		scenarios.DefaultMCC,
		scenarios.DefaultMNC,
		scenarios.DefaultSST,
		scenarios.DefaultSD,
		scenarios.DefaultDNN,
		scenarios.DefaultTAC,
		"Ella-Core-Tester-ENB",
		env.FirstCore(),
		g.N2Address,
		g.N3Address,
	)
	if err != nil {
		return fmt.Errorf("error starting eNB: %v", err)
	}

	defer ngeNB.Close()

	_, err = ngeNB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	eg := errgroup.Group{}

	for i := range enbNumConnectivityParallel {
		func() {
			eg.Go(func() error {
				ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)
				tunInterfaceName := fmt.Sprintf(enbGTPInterfaceNamePrefix+"%d", i)

				return runEnbConnectivityTest(
					ctx,
					ranUENGAPID,
					ngeNB,
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

func runEnbConnectivityTest(
	ctx context.Context,
	ranUENGAPID int64,
	ngeNB *enb.NgeNB,
	subscriber enbSubscriber,
	tunInterfaceName string,
) error {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         ngeNB.GnodeB,
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: nasMessage.PDUSessionTypeIPv4,
		Msin:           subscriber.IMSI[5:],
		K:              subscriber.Key,
		OpC:            subscriber.OPc,
		Amf:            scenarios.DefaultAMF,
		Sqn:            subscriber.SequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              scenarios.DefaultDNN,
		Sst:              scenarios.DefaultSST,
		Sd:               scenarios.DefaultSD,
		IMEISV:           scenarios.DefaultIMEISV,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{
				Nia2: true,
			},
			Ciphering: testutil.CipheringAlgorithms{
				Nea0: true,
				Nea2: true,
			},
		}),
	})
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	ngeNB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed Initial Registration Procedure",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.Int64("AMF UE NGAP ID", ngeNB.GetAMFUENGAPID(ranUENGAPID)),
	)

	uePDUSession, err := newUE.WaitForPDUSession(scenarios.DefaultPDUSessionID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for PDU session: %v", err)
	}

	uePduSession := newUE.GetPDUSession(scenarios.DefaultPDUSessionID)
	ueIP := uePduSession.UEIP + "/16"

	gnbPDUSession, err := ngeNB.WaitForPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID), 5*time.Second)
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

	_, err = ngeNB.AddTunnel(&gnb.NewTunnelOpts{
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

	cmd := exec.CommandContext(ctx, "ping", "-I", tunInterfaceName, scenarios.DefaultPingDestination, "-c", "3", "-W", "1") // #nosec G204 -- ping is fixed; tunInterfaceName is internally derived; destination is test config

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
		AMFUENGAPID:   ngeNB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		GnodeB:        ngeNB.GnodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionStatus,
	})
	if err != nil {
		return fmt.Errorf("UE context release procedure failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed UE Context Release Procedure",
		zap.Int64("AMF UE NGAP ID", ngeNB.GetAMFUENGAPID(ranUENGAPID)),
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
		zap.Int64("AMF UE NGAP ID", ngeNB.GetAMFUENGAPID(ranUENGAPID)),
	)

	err = ngeNB.CloseTunnel(gnbPDUSession.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close GTP tunnel: %v", err)
	}

	pduSession := ngeNB.GetPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID))

	_, err = ngeNB.AddTunnel(&gnb.NewTunnelOpts{
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

	// NOTE: client-side usage assertion skipped; integration test will verify via its own client.
	logger.Logger.Debug("client-side usage assertion skipped; integration test will verify",
		zap.String("IMSI", subscriber.IMSI),
	)

	err = ngeNB.CloseTunnel(pduSession.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close GTP tunnel: %v", err)
	}

	logger.Logger.Debug(
		"Closed GTP tunnel",
		zap.String("interface", tunInterfaceName),
	)

	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: ngeNB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("deregistration procedure failed: %v", err)
	}

	return nil
}
