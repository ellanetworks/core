package ue

import (
	"context"
	"crypto/ecdh"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

type profileAParams struct {
	HomeNetworkPrivateKeyHex string
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "ue/registration_success_profile_a",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &profileAParams{}
			fs.StringVar(&p.HomeNetworkPrivateKeyHex, "home-network-private-key", "",
				"hex-encoded X25519 private key; must match the one provisioned on Core")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationSuccessProfileA(ctx, env, params.(*profileAParams))
		},
		Fixture: fixtureRegistrationSuccessProfileA,
	})
}

const profileAHomeNetworkPrivateKeyHex = "c53c22208b61860b06c62e5406a7b330c2b577aab3cd7cd2d3fa33ef6b3df3f6"

func fixtureRegistrationSuccessProfileA() scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		HomeNetworkKeys: []scenarios.HomeNetworkKeySpec{
			{
				KeyIdentifier: 4,
				Scheme:        "A",
				PrivateKey:    profileAHomeNetworkPrivateKeyHex,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
		ExtraArgs:   []string{"--home-network-private-key", profileAHomeNetworkPrivateKeyHex},
	}
}

func runRegistrationSuccessProfileA(_ context.Context, env scenarios.Env, params *profileAParams) error {
	if params.HomeNetworkPrivateKeyHex == "" {
		return fmt.Errorf("--home-network-private-key required; must match the key provisioned on Core")
	}

	privKeyBytes, err := hex.DecodeString(params.HomeNetworkPrivateKeyHex)
	if err != nil {
		return fmt.Errorf("decode private key: %w", err)
	}

	privateKey, err := ecdh.X25519().NewPrivateKey(privKeyBytes)
	if err != nil {
		return fmt.Errorf("construct X25519 private key: %w", err)
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
		return fmt.Errorf("start gNB: %w", err)
	}

	defer gNodeB.Close()

	if _, err := gNodeB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		200*time.Millisecond,
	); err != nil {
		return fmt.Errorf("wait NGSetupResponse: %w", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: PDUSessionType,
		GnodeB:         gNodeB,
		Msin:           scenarios.DefaultIMSI[5:],
		K:              scenarios.DefaultKey,
		OpC:            scenarios.DefaultOPC,
		Amf:            scenarios.DefaultAMF,
		Sqn:            scenarios.DefaultSequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.ProfileAScheme,
			PublicKeyID:      "4",
			PublicKey:        privateKey.PublicKey(),
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
		return fmt.Errorf("create UE: %w", err)
	}

	gNodeB.AddUE(int64(scenarios.DefaultRANUENGAPID), newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  int64(scenarios.DefaultRANUENGAPID),
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	if err := procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(int64(scenarios.DefaultRANUENGAPID)),
		RANUENGAPID: int64(scenarios.DefaultRANUENGAPID),
	}); err != nil {
		return fmt.Errorf("deregistration: %w", err)
	}

	return nil
}
