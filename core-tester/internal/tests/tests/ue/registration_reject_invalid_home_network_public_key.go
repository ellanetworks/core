package ue

import (
	"context"
	"crypto/ecdh"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/core"
	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/ellanetworks/core/core-tester/internal/ue/sidf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

type RegistrationRejectInvalidHomeNetworkPublicKey struct{}

func (RegistrationRejectInvalidHomeNetworkPublicKey) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/registration_reject_invalid_home_network_public_key",
		Summary: "UE registration test validating the Registration Request and Authentication procedures with wrong key associated to Profile A SUCI protection",
		Timeout: 5 * time.Second,
	}
}

func (t RegistrationRejectInvalidHomeNetworkPublicKey) Run(ctx context.Context, env engine.Env) error {
	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, getDefaultEllaCoreConfig())

	err := ellaCoreEnv.Create(ctx)
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

	// This key will (very very very likely) not match Ella Core's randomly generated private key
	key, err := hex.DecodeString("68863be1b86661a38a720217ec17170c5feda91e891cb3f53d4b74fbabb10247")
	if err != nil {
		return fmt.Errorf("invalid Home Network Public Key in configuration for Profile A: %w", err)
	}

	publicKey, err := ecdh.X25519().NewPublicKey(key)
	if err != nil {
		return fmt.Errorf("invalid Home Network Public Key in configuration for Profile A: %w", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		PDUSessionID:   PDUSessionID,
		PDUSessionType: PDUSessionType,
		GnodeB:         gNodeB,
		Msin:           DefaultIMSI[5:],
		K:              DefaultKey,
		OpC:            DefaultOPC,
		Amf:            "80000000000000000000000000000000",
		Sqn:            DefaultSequenceNumber,
		Mcc:            DefaultMCC,
		Mnc:            DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.ProfileAScheme,
			PublicKeyID:      "1",
			PublicKey:        publicKey,
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

	gNodeB.AddUE(RANUENGAPID, newUE)

	err = newUE.SendRegistrationRequest(RANUENGAPID, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		return fmt.Errorf("could not send Registration Request: %v", err)
	}

	msg, err := newUE.WaitForNASGMMMessage(nas.MsgTypeRegistrationReject, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive Authentication Reject: %v", err)
	}

	err = validateRegistrationReject(msg, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
	if err != nil {
		return fmt.Errorf("NAS PDU validation failed: %v", err)
	}

	err = ellaCoreEnv.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not delete EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Deleted EllaCore environment")

	return nil
}
