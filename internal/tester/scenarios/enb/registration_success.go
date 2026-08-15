// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/enb"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "enb/registration_success",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runEnbRegistrationSuccess(ctx, env, params)
		},
		Fixture: fixtureEnbRegistrationSuccess,
	})
}

func fixtureEnbRegistrationSuccess(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runEnbRegistrationSuccess(_ context.Context, env scenarios.Env, _ any) error {
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

	_, err = ngeNB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: fgs.PDUSessionTypeIPv4,
		GnodeB:         ngeNB.GnodeB,
		Msin:           scenarios.DefaultIMSI[5:],
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

	ngeNB.AddUE(int64(scenarios.DefaultRANUENGAPID), newUE)

	_, err = ngeNB.Register(newUE, int64(scenarios.DefaultRANUENGAPID), scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	err = ngeNB.Deregister(newUE, int64(scenarios.DefaultRANUENGAPID), releaseTimeout)
	if err != nil {
		return fmt.Errorf("deregistration procedure failed: %v", err)
	}

	return nil
}
