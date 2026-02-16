package ue

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/core"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/ellanetworks/core/core-tester/internal/ue/sidf"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	GTPInterfaceNamePrefix  = "ellatester"
	GTPUPort                = 2152
	NumConnectivityParallel = 5
)

type Connectivity struct{}

func (Connectivity) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/connectivity",
		Summary: "UE connectivity test validating the connectivity of 5 UE's in parallel after registration, and after UE Context Release",
		Timeout: 45 * time.Second,
	}
}

func (t Connectivity) Run(ctx context.Context, env engine.Env) error {
	subs, err := buildSubscriberConfig(NumConnectivityParallel, testStartIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
	}

	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, core.EllaCoreConfig{
		Operator: core.OperatorConfig{
			ID: core.OperatorID{
				MCC: DefaultMCC,
				MNC: DefaultMNC,
			},
			Slice: core.OperatorSlice{
				SST: DefaultSST,
				SD:  DefaultSD,
			},
			Tracking: core.OperatorTracking{
				SupportedTACs: []string{DefaultTAC},
			},
		},
		DataNetworks: []core.DataNetworkConfig{
			{
				Name:   DefaultDNN,
				IPPool: "10.45.0.0/16",
				DNS:    "8.8.8.8",
				Mtu:    1500,
			},
		},
		Policies: []core.PolicyConfig{
			{
				Name:            DefaultPolicyName,
				BitrateUplink:   "100 Mbps",
				BitrateDownlink: "100 Mbps",
				Var5qi:          9,
				Arp:             15,
				DataNetworkName: DefaultDNN,
			},
		},
		Subscribers: subs,
	})

	err = ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Created EllaCore environment")

	gNodeB, err := gnb.Start(
		GNBID,
		DefaultMCC,
		DefaultMNC,
		DefaultSST,
		DefaultSD,
		DefaultDNN,
		DefaultTAC,
		"Ella-Core-Tester",
		env.Config.EllaCore.N2Address,
		env.Config.Gnb.N2Address,
		env.Config.Gnb.N3Address,
	)
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	eg := errgroup.Group{}

	for i := range NumConnectivityParallel {
		func() {
			eg.Go(func() error {
				ranUENGAPID := RANUENGAPID + int64(i)
				tunInterfaceName := fmt.Sprintf(GTPInterfaceNamePrefix+"%d", i)

				return runConnectivityTest(
					env,
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

	err = ellaCoreEnv.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not delete EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Deleted EllaCore environment")

	return nil
}

func runConnectivityTest(
	env engine.Env,
	ranUENGAPID int64,
	gNodeB *gnb.GnodeB,
	subscriber core.SubscriberConfig,
	tunInterfaceName string,
) error {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   PDUSessionID,
		PDUSessionType: PDUSessionType,
		Msin:           subscriber.Imsi[5:],
		K:              subscriber.Key,
		OpC:            subscriber.OPc,
		Amf:            "80000000000000000000000000000000",
		Sqn:            subscriber.SequenceNumber,
		Mcc:            DefaultMCC,
		Mnc:            DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: "0000",
		DNN:              DefaultDNN,
		Sst:              DefaultSST,
		Sd:               DefaultSD,
		IMEISV:           "3569380356438091",
		UeSecurityCapability: utils.GetUESecurityCapability(&utils.UeSecurityCapability{
			Integrity: utils.IntegrityAlgorithms{
				Nia2: true,
			},
			Ciphering: utils.CipheringAlgorithms{
				Nea0: true,
				Nea2: true,
			},
		}),
	})
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID: ranUENGAPID,
		UE:          newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed Initial Registration Procedure",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(ranUENGAPID)),
	)

	uePDUSession, err := newUE.WaitForPDUSession(5 * time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for PDU session: %v", err)
	}

	uePduSession := newUE.GetPDUSession()
	ueIP := uePduSession.UEIP + "/16"

	gnbPDUSession, err := gNodeB.WaitForPDUSession(ranUENGAPID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("could not get PDU Session for RAN UE NGAP ID %d: %v", ranUENGAPID, err)
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

	cmd := exec.CommandContext(context.TODO(), "ping", "-I", tunInterfaceName, env.Config.PingDestination, "-c", "3", "-W", "1")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping %s via %s failed after initial registration: %v\noutput:\n%s", env.Config.PingDestination, tunInterfaceName, err, string(out))
	}

	logger.Logger.Debug(
		"Ping successful",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", env.Config.PingDestination),
	)

	pduSessionStatus := [16]bool{}
	pduSessionStatus[PDUSessionID] = true

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

	cmd = exec.CommandContext(context.TODO(), "ping", "-I", tunInterfaceName, env.Config.PingDestination, "-c", "3", "-W", "1")

	out, err = cmd.CombinedOutput() // stdout + stderr
	if err == nil {
		return fmt.Errorf("ping %s via %s succeeded, but was expected to fail after UE Context Release\noutput:\n%s", env.Config.PingDestination, tunInterfaceName, string(out))
	}

	logger.Logger.Debug(
		"Ping failed as expected after UE Context Release",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", env.Config.PingDestination),
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

	pduSession := gNodeB.GetPDUSession(ranUENGAPID)

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP, // re-using the same UE IP, we may need to change this to fetch the IP from the Service Request response in the future
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

	cmd = exec.CommandContext(context.TODO(), "ping", "-I", tunInterfaceName, env.Config.PingDestination, "-c", "3", "-W", "1")

	out, err = cmd.CombinedOutput() // stdout + stderr
	if err != nil {
		return fmt.Errorf("ping %s via %s failed after service request: %v\noutput:\n%s", env.Config.PingDestination, tunInterfaceName, err, string(out))
	}

	logger.Logger.Debug(
		"Ping successful after Service Request",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", env.Config.PingDestination),
	)

	uplinkBytes, downlinkBytes, err := core.WaitForUsage(env.EllaCoreClient, subscriber.Imsi, 30*time.Second)
	if err != nil {
		return fmt.Errorf("error waiting for usage: %v", err)
	}

	logger.Logger.Debug(
		"Data usage detected",
		zap.String("IMSI", subscriber.Imsi),
		zap.Uint64("uplink bytes", uplinkBytes),
		zap.Uint64("downlink bytes", downlinkBytes),
	)

	// Cleanup
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
