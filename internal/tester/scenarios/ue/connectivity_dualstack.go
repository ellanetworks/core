package ue

import (
	"context"
	"fmt"
	"os/exec"
	"time"

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
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/connectivity_dualstack",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runConnectivityDualStack(ctx, env, params)
		},
		Fixture: fixtureConnectivityDualStack,
	})
}

func fixtureConnectivityDualStack() scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runConnectivityDualStack(ctx context.Context, env scenarios.Env, _ any) error {
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

	sub := subscriber{
		IMSI:           scenarios.DefaultIMSI,
		Key:            scenarios.DefaultKey,
		SequenceNumber: scenarios.DefaultSequenceNumber,
		OPc:            scenarios.DefaultOPC,
		ProfileName:    scenarios.DefaultProfileName,
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: nasMessage.PDUSessionTypeIPv4IPv6,
		Msin:           sub.IMSI[5:],
		K:              sub.Key,
		OpC:            sub.OPc,
		Amf:            scenarios.DefaultAMF,
		Sqn:            sub.SequenceNumber,
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

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
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

	amfUENGAPID := gNodeB.GetAMFUENGAPID(ranUENGAPID)

	uePDUSession := newUE.GetPDUSession(scenarios.DefaultPDUSessionID)

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

	logger.Logger.Debug(
		"Completed Initial Registration (Dual-Stack)",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	tunIPv4 := gtpInterfaceNamePrefix + "ds0"
	tunIPv6 := gtpInterfaceNamePrefix + "ds1"

	ueIPv4 := uePDUSession.UEIP + "/16"

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIPv4,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: tunIPv4,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              uePDUSession.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel for IPv4 (name: %s): %v", tunIPv4, err)
	}

	logger.GnbLogger.Debug("Created GTP tunnel for IPv4 (Dual-Stack)",
		zap.String("interface", tunIPv4),
		zap.String("UE IP", ueIPv4),
	)

	cmd := exec.CommandContext(ctx, "ping", "-I", tunIPv4, scenarios.DefaultPingDestination, "-c", "3", "-W", "1") // #nosec G204 -- test constants only, no user input

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping %s via %s (IPv4) failed: %v\noutput:\n%s", scenarios.DefaultPingDestination, tunIPv4, err, string(out))
	}

	logger.Logger.Debug("Ping successful on IPv4 (Dual-Stack)",
		zap.String("interface", tunIPv4),
		zap.String("destination", scenarios.DefaultPingDestination),
	)

	_, err = newUE.WaitForPDUSession(2, 5*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for PDU session 2 (IPv6): %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	uePDUSession2 := newUE.GetPDUSession(2)
	ueIPv6 := uePDUSession2.UEIP + "/48"

	gnbPDUSession2, err := gNodeB.WaitForPDUSession(ranUENGAPID, 2, 5*time.Second)
	if err != nil {
		return fmt.Errorf("could not get PDU Session 2 for RAN UE NGAP ID %d: %v", ranUENGAPID, err)
	}

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIPv6,
		UpfIP:            gnbPDUSession2.UpfAddress,
		TunInterfaceName: tunIPv6,
		ULteid:           gnbPDUSession2.ULTeid,
		DLteid:           gnbPDUSession2.DLTeid,
		MTU:              uePDUSession2.MTU,
		QFI:              uePDUSession2.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel for IPv6 (name: %s): %v", tunIPv6, err)
	}

	logger.GnbLogger.Debug("Created GTP tunnel for IPv6 (Dual-Stack)",
		zap.String("interface", tunIPv6),
		zap.String("UE IP", ueIPv6),
	)

	cmd = exec.CommandContext(ctx, "ping6", "-I", tunIPv6, scenarios.DefaultPingDestinationV6, "-c", "3", "-W", "1") // #nosec G204 -- test constants only, no user input

	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping6 %s via %s (IPv6) failed: %v\noutput:\n%s", scenarios.DefaultPingDestinationV6, tunIPv6, err, string(out))
	}

	logger.Logger.Debug("Ping6 successful on IPv6 (Dual-Stack)",
		zap.String("interface", tunIPv6),
		zap.String("destination", scenarios.DefaultPingDestinationV6),
	)

	err = gNodeB.CloseTunnel(gnbPDUSession.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close GTP tunnel for IPv4: %v", err)
	}

	err = gNodeB.CloseTunnel(gnbPDUSession2.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close GTP tunnel for IPv6: %v", err)
	}

	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: amfUENGAPID,
		RANUENGAPID: ranUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("deregistration failed: %v", err)
	}

	logger.Logger.Debug("Deregistered UE after dual-stack connectivity test")

	return nil
}
