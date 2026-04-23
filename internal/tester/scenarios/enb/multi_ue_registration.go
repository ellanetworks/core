package enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

const (
	enbNumMultiUERegistration = 8
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "enb/multi_ue_registration",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runEnbMultiUERegistration(ctx, env, params)
		},
		Fixture: fixtureEnbMultiUERegistration,
	})
}

func fixtureEnbMultiUERegistration() scenarios.FixtureSpec {
	subs := make([]scenarios.SubscriberSpec, enbNumMultiUERegistration)
	for i := range enbNumMultiUERegistration {
		subs[i] = scenarios.DefaultSubscriberWith(enbIncrementIMSI(enbTestStartIMSI, i), "")
	}

	return scenarios.FixtureSpec{Subscribers: subs}
}

func runEnbMultiUERegistration(_ context.Context, env scenarios.Env, _ any) error {
	subs, err := buildEnbSubscribers(enbNumMultiUERegistration, enbTestStartIMSI)
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

	for i := range enbNumMultiUERegistration {
		func() {
			eg.Go(func() error {
				ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)

				return runEnbMultiUERegistrationTest(ranUENGAPID, ngeNB, subs[i])
			})
		}()
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("error during UE registrations: %v", err)
	}

	return nil
}

func runEnbMultiUERegistrationTest(
	ranUENGAPID int64,
	ngeNB *enb.NgeNB,
	subscriber enbSubscriber,
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
