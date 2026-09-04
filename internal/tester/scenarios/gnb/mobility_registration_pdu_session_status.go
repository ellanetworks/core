// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
)

const mobilityRegStatusIMSI = "001017271246548"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/mobility_registration_pdu_session_status",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runMobilityRegistrationPDUSessionStatus(ctx, env, params)
		},
		Fixture: fixtureMobilityRegistrationPDUSessionStatus,
	})
}

func fixtureMobilityRegistrationPDUSessionStatus(env scenarios.Env) scenarios.FixtureSpec {
	spec := fixtureConnectivityMultiPDUSession(env)
	spec.Subscribers = []scenarios.SubscriberSpec{
		scenarios.DefaultSubscriberWith(mobilityRegStatusIMSI, "multi-pdu-profile"),
	}
	spec.AssertUsageForIMSIs = []string{mobilityRegStatusIMSI}

	return spec
}

func runMobilityRegistrationPDUSessionStatus(ctx context.Context, env scenarios.Env, _ any) error {
	const (
		dnn1 = scenarios.DefaultDNN
		dnn2 = "enterprise"

		slice2SST = int32(1)
		slice2SD  = "204060"

		keptPDUSessionID    uint8 = 1
		droppedPDUSessionID uint8 = 2
	)

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	updateRANUENGAPID := ranUENGAPID + 1
	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             dnn1,
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
		Slices: []gnb.SliceOpt{
			{Sst: scenarios.DefaultSST, Sd: scenarios.DefaultSD},
			{Sst: slice2SST, Sd: slice2SD},
		},
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %w", err)
	}

	defer gNodeB.Close()

	if _, err := gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, scenarios.NGSetupTimeout); err != nil {
		return fmt.Errorf("did not receive NG Setup Response: %w", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   keptPDUSessionID,
		PDUSessionType: fgs.PDUSessionType(env.PDUSessionType()),
		Msin:           mobilityRegStatusIMSI[5:],
		K:              scenarios.DefaultKey,
		OpC:            scenarios.DefaultOPC,
		Amf:            scenarios.DefaultAMF,
		Sqn:            scenarios.DefaultSequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              dnn1,
		Sst:              scenarios.DefaultSST,
		Sd:               scenarios.DefaultSD,
		IMEISV:           scenarios.DefaultIMEISV,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{Nia2: true},
			Ciphering: testutil.CipheringAlgorithms{Nea0: true, Nea2: true},
		}),
	})
	if err != nil {
		return fmt.Errorf("could not create UE: %w", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := gNodeB.Register(newUE, ranUENGAPID, keptPDUSessionID, registrationTimeout); err != nil {
		return fmt.Errorf("initial registration procedure failed: %w", err)
	}

	slice2Snssai := models.Snssai{Sst: slice2SST, Sd: slice2SD}
	if _, err := gNodeB.EstablishPDUSession(newUE, ranUENGAPID, droppedPDUSessionID, dnn2, slice2Snssai, registrationTimeout); err != nil {
		return fmt.Errorf("could not establish PDU session %d: %w", droppedPDUSessionID, err)
	}

	if err := awaitSessionCount(ctx, env, mobilityRegStatusIMSI, 2); err != nil {
		return fmt.Errorf("before the mobility registration update: %w", err)
	}

	sessions := []uint8{keptPDUSessionID, droppedPDUSessionID}
	if err := gNodeB.ReleaseContext(newUE, ranUENGAPID, sessions, gnb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("release the connection before the mobility registration update: %w", err)
	}

	var reported [16]bool

	reported[keptPDUSessionID] = true

	update, err := gNodeB.MobilityRegistrationUpdate(newUE, updateRANUENGAPID, keptPDUSessionID, &reported, registrationTimeout)
	if err != nil {
		return fmt.Errorf("mobility registration update reporting PDU session %d inactive: %w", droppedPDUSessionID, err)
	}

	if err := assertReportedPDUSessionStatus(update.PDUSessionStatus, keptPDUSessionID, droppedPDUSessionID); err != nil {
		return err
	}

	if err := awaitSessionCount(ctx, env, mobilityRegStatusIMSI, 1); err != nil {
		return fmt.Errorf("after the mobility registration update: %w", err)
	}

	tun := gtpInterfaceNamePrefix + "mrs0"

	ueIP := update.Session.UEIPv4 + env.UIPrefix()
	if env.IPFamily() == scenarios.IPv6Only {
		ueIP = update.Session.UEIPv6 + env.UIPrefix()
	}

	err = gNodeB.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           ueIP,
		UpfAddress:       update.Session.UpfAddress,
		TunInterfaceName: tun,
		ULTEID:           update.Session.ULTEID,
		DLTEID:           update.Session.DLTEID,
		MTU:              update.Session.MTU,
		QFI:              update.Session.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel for the surviving session: %w", err)
	}

	defer gNodeB.CloseTunnel(update.Session.DLTEID)

	if err := probe.Run(ctx, probe.ICMP, tun, env.PingDestination(), scenarios.DefaultProbePort, env.PingCommand() == "ping6"); err != nil {
		return fmt.Errorf("ping via %s (DNN %s) failed after the mobility registration update: %w", tun, dnn1, err)
	}

	return gNodeB.Deregister(newUE, updateRANUENGAPID, releaseTimeout)
}
