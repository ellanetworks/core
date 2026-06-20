// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"crypto/ecdh"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/registration_reject_invalid_home_network_public_key",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationRejectInvalidHomeNetworkPublicKey(ctx, env, params)
		},
		Fixture: fixtureRegistrationRejectInvalidHomeNetworkPublicKey,
	})
}

func fixtureRegistrationRejectInvalidHomeNetworkPublicKey(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runRegistrationRejectInvalidHomeNetworkPublicKey(_ context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	// This key will very likely not match Ella Core's randomly generated private key.
	key, err := hex.DecodeString("68863be1b86661a38a720217ec17170c5feda91e891cb3f53d4b74fbabb10247")
	if err != nil {
		return fmt.Errorf("invalid Home Network Public Key in configuration for Profile A: %w", err)
	}

	publicKey, err := ecdh.X25519().NewPublicKey(key)
	if err != nil {
		return fmt.Errorf("invalid Home Network Public Key in configuration for Profile A: %w", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: env.PDUSessionType(),
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
			PublicKeyID:      "1",
			PublicKey:        publicKey,
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

	gNodeB.AddUE(int64(scenarios.DefaultRANUENGAPID), newUE)

	err = newUE.SendRegistrationRequest(int64(scenarios.DefaultRANUENGAPID), nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		return fmt.Errorf("could not send Registration Request: %v", err)
	}

	msg, err := newUE.WaitForNASGMMMessage(nas.MsgTypeRegistrationReject, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive Registration Reject: %v", err)
	}

	err = validateRegistrationReject(msg, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
	if err != nil {
		return fmt.Errorf("NAS PDU validation failed: %v", err)
	}

	return nil
}
