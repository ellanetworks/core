package ue

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/tests/engine"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/core"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/validate"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type ConnectivityMultiplePoliciesPerProfile struct{}

func (ConnectivityMultiplePoliciesPerProfile) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/connectivity_multiple_policies_per_profile",
		Summary: "UE connectivity test with multiple policies on the same profile, each targeting a different data network",
		Timeout: 60 * time.Second,
	}
}

func (t ConnectivityMultiplePoliciesPerProfile) Run(ctx context.Context, env engine.Env) error {
	const (
		dnn2    = "enterprise"
		ipPool1 = "10.45.0.0/16"
		ipPool2 = "10.46.0.0/16"
	)

	subs := []core.SubscriberConfig{
		{
			Imsi:           "001017271246546",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			ProfileName:    DefaultProfileName,
		},
		{
			Imsi:           "001017271246547",
			Key:            DefaultKey,
			SequenceNumber: DefaultSequenceNumber,
			OPc:            DefaultOPC,
			ProfileName:    DefaultProfileName,
		},
	}

	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, core.EllaCoreConfig{
		Operator: core.OperatorConfig{
			ID: core.OperatorID{
				MCC: DefaultMCC,
				MNC: DefaultMNC,
			},
			Tracking: core.OperatorTracking{
				SupportedTACs: []string{DefaultTAC},
			},
		},
		Profiles: []core.ProfileConfig{
			{
				Name:           DefaultProfileName,
				UeAmbrUplink:   DefaultProfileUeAmbrUplink,
				UeAmbrDownlink: DefaultProfileUeAmbrDownlink,
			},
		},
		Slices: []core.SliceConfig{
			{
				Name: DefaultSliceName,
				SST:  DefaultSST,
				SD:   DefaultSD,
			},
		},
		DataNetworks: []core.DataNetworkConfig{
			{
				Name:   DefaultDNN,
				IPPool: ipPool1,
				DNS:    "8.8.8.8",
				Mtu:    1500,
			},
			{
				Name:   dnn2,
				IPPool: ipPool2,
				DNS:    "8.8.4.4",
				Mtu:    1500,
			},
		},
		Policies: []core.PolicyConfig{
			{
				Name:                DefaultPolicyName,
				ProfileName:         DefaultProfileName,
				SliceName:           DefaultSliceName,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
				DataNetworkName:     DefaultDNN,
			},
			{
				Name:                "enterprise",
				ProfileName:         DefaultProfileName,
				SliceName:           DefaultSliceName,
				SessionAmbrUplink:   "30 Mbps",
				SessionAmbrDownlink: "60 Mbps",
				Var5qi:              7,
				Arp:                 15,
				DataNetworkName:     dnn2,
			},
		},
		Subscribers: subs,
	})

	err := ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	defer func() {
		if delErr := ellaCoreEnv.Delete(ctx); delErr != nil {
			logger.Logger.Error("could not delete EllaCore environment", zap.Error(delErr))
		}
	}()

	logger.Logger.Debug("Created EllaCore environment")

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         GNBID,
		MCC:           DefaultMCC,
		MNC:           DefaultMNC,
		SST:           DefaultSST,
		SD:            DefaultSD,
		DNN:           DefaultDNN,
		TAC:           DefaultTAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: env.Config.EllaCore.N2Address,
		GnbN2Address:  env.Config.Gnb.N2Address,
		GnbN3Address:  env.Config.Gnb.N3Address,
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	dnns := []string{DefaultDNN, dnn2}
	expectedQoS := []*validate.ExpectedPDUSessionInformation{
		{FiveQi: 9, PriArp: 15, QFI: 1},
		{FiveQi: 7, PriArp: 15, QFI: 1},
	}

	eg := errgroup.Group{}

	for i := range subs {
		func() {
			eg.Go(func() error {
				ranUENGAPID := RANUENGAPID + int64(i)
				tunInterfaceName := fmt.Sprintf(GTPInterfaceNamePrefix+"%d", i)

				return runConnectivityTestWithDNN(
					ctx,
					env,
					ranUENGAPID,
					gNodeB,
					subs[i],
					tunInterfaceName,
					dnns[i],
					expectedQoS[i],
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

func runConnectivityTestWithDNN(
	ctx context.Context,
	env engine.Env,
	ranUENGAPID int64,
	gNodeB *gnb.GnodeB,
	subscriber core.SubscriberConfig,
	tunInterfaceName string,
	dnn string,
	expectedQoS *validate.ExpectedPDUSessionInformation,
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
		DNN:              dnn,
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
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: PDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed Initial Registration Procedure",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("DNN", dnn),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(ranUENGAPID)),
	)

	uePDUSession, err := newUE.WaitForPDUSession(PDUSessionID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for PDU session: %v", err)
	}

	uePduSession := newUE.GetPDUSession(PDUSessionID)
	ueIP := uePduSession.UEIP + "/16"

	gnbPDUSession, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(PDUSessionID), 5*time.Second)
	if err != nil {
		return fmt.Errorf("could not get PDU Session for RAN UE NGAP ID %d: %v", ranUENGAPID, err)
	}

	err = validate.PDUSessionInformation(gnbPDUSession, expectedQoS)
	if err != nil {
		return fmt.Errorf("NGAP QoS validation failed (DNN %s): %v", dnn, err)
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
		zap.String("DNN", dnn),
		zap.String("Interface", tunInterfaceName),
		zap.String("UE IP", ueIP),
		zap.String("UPF IP", gnbPDUSession.UpfAddress),
		zap.Uint32("UL TEID", gnbPDUSession.ULTeid),
		zap.Uint32("DL TEID", gnbPDUSession.DLTeid),
	)

	cmd := exec.CommandContext(ctx, "ping", "-I", tunInterfaceName, env.Config.PingDestination, "-c", "3", "-W", "1") // #nosec G204 -- ping is fixed; tunInterfaceName is internally derived; PingDestination is test config

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping %s via %s (DNN %s) failed after initial registration: %v\noutput:\n%s", env.Config.PingDestination, tunInterfaceName, dnn, err, string(out))
	}

	logger.Logger.Debug(
		"Ping successful",
		zap.String("DNN", dnn),
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

	cmd = exec.CommandContext(ctx, "ping", "-I", tunInterfaceName, env.Config.PingDestination, "-c", "3", "-W", "1") // #nosec G204 -- ping is fixed; tunInterfaceName is internally derived; PingDestination is test config

	out, err = cmd.CombinedOutput()
	if err == nil {
		return fmt.Errorf("ping %s via %s succeeded, but was expected to fail after UE Context Release\noutput:\n%s", env.Config.PingDestination, tunInterfaceName, string(out))
	}

	logger.Logger.Debug(
		"Ping failed as expected after UE Context Release",
		zap.String("DNN", dnn),
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
		zap.String("DNN", dnn),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(ranUENGAPID)),
	)

	err = gNodeB.CloseTunnel(gnbPDUSession.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close GTP tunnel: %v", err)
	}

	pduSession := gNodeB.GetPDUSession(ranUENGAPID, int64(PDUSessionID))

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
		zap.String("DNN", dnn),
		zap.String("Interface", tunInterfaceName),
		zap.String("UE IP", ueIP),
		zap.String("UPF IP", pduSession.UpfAddress),
		zap.Uint32("UL TEID", pduSession.ULTeid),
		zap.Uint32("DL TEID", pduSession.DLTeid),
	)

	cmd = exec.CommandContext(ctx, "ping", "-I", tunInterfaceName, env.Config.PingDestination, "-c", "3", "-W", "1") // #nosec G204 -- ping is fixed; tunInterfaceName is internally derived; PingDestination is test config

	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping %s via %s (DNN %s) failed after service request: %v\noutput:\n%s", env.Config.PingDestination, tunInterfaceName, dnn, err, string(out))
	}

	logger.Logger.Debug(
		"Ping successful after Service Request",
		zap.String("DNN", dnn),
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
		zap.String("DNN", dnn),
		zap.Uint64("uplink bytes", uplinkBytes),
		zap.Uint64("downlink bytes", downlinkBytes),
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
