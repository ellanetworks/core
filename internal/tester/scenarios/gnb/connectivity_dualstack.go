// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/connectivity_dualstack",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runConnectivityDualStack(ctx, env, params)
		},
		Fixture: fixtureConnectivityDualStack,
	})
}

func fixtureConnectivityDualStack(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runConnectivityDualStack(ctx context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

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
		PDUSessionType: fgs.PDUSessionTypeIPv4v6,
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

	registration, err := gNodeB.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout)
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

	session := registration.Session

	err = validate.PDUSessionInformation(session, &validate.ExpectedPDUSessionInformation{
		FiveQI: 9,
		ARP:    15,
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

	tunName := gtpInterfaceNamePrefix + "ds0"

	ueIPv4 := session.UEIPv4 + "/16"
	ueIPv6 := session.UEIPv6 + "/64"

	err = gNodeB.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           ueIPv4,
		UEIPv6:           ueIPv6,
		UpfAddress:       session.UpfAddress,
		TunInterfaceName: tunName,
		ULTEID:           session.ULTEID,
		DLTEID:           session.DLTEID,
		MTU:              session.MTU,
		QFI:              session.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel (name: %s): %v", tunName, err)
	}

	logger.GnbLogger.Debug("Created GTP tunnel for dual-stack (Dual-Stack)",
		zap.String("interface", tunName),
		zap.String("UE IPv4", ueIPv4),
		zap.String("UE IPv6", ueIPv6),
	)

	err = gnb.WaitForULAAddr(tunName, scenarios.DefaultUEIPv6Pool, 5*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for ULA address on %s: %v", tunName, err)
	}

	if err := probe.Run(ctx, probe.ICMP, tunName, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping via %s (IPv4) failed: %w", tunName, err)
	}

	logger.Logger.Debug("Ping successful on IPv4 (Dual-Stack)",
		zap.String("interface", tunName),
		zap.String("destination", scenarios.DefaultPingDestination),
	)

	if err := probe.Run(ctx, probe.ICMP, tunName, scenarios.DefaultPingDestinationV6, scenarios.DefaultProbePort, true); err != nil {
		return fmt.Errorf("ping6 via %s (IPv6) failed: %w", tunName, err)
	}

	logger.Logger.Debug("Ping6 successful on IPv6 (Dual-Stack)",
		zap.String("interface", tunName),
		zap.String("destination", scenarios.DefaultPingDestinationV6),
	)

	gNodeB.CloseTunnel(session.DLTEID)

	err = gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout)
	if err != nil {
		return fmt.Errorf("deregistration failed: %v", err)
	}

	logger.Logger.Debug("Deregistered UE after dual-stack connectivity test")

	return nil
}
