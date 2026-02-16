package ue

import (
	"context"
	"fmt"
	"net/netip"
	"reflect"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/core"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/validate"
	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/ellanetworks/core/core-tester/internal/ue/sidf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/security"
	"github.com/free5gc/ngap/ngapType"
)

const (
	RANUENGAPID    = 1
	GNBID          = "000008"
	PDUSessionID   = 1
	PDUSessionType = nasMessage.PDUSessionTypeIPv4
)

type RegistrationSuccess struct{}

func (RegistrationSuccess) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/registration_success",
		Summary: "UE registration success test validating the Registration Request and Authentication procedures",
		Timeout: 5 * time.Second,
	}
}

func (t RegistrationSuccess) Run(ctx context.Context, env engine.Env) error {
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

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
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

	gNodeB.AddUE(RANUENGAPID, newUE)

	err = runInitialRegistration(&InitialRegistrationOpts{
		RANUENGAPID:            RANUENGAPID,
		PDUSessionID:           PDUSessionID,
		ExpectedPDUSessionType: PDUSessionType,
		UE:                     newUE,
		GnodeB:                 gNodeB,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	// Cleanup
	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(RANUENGAPID),
		RANUENGAPID: RANUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	err = ellaCoreEnv.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not delete EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Deleted EllaCore environment")

	return nil
}

type InitialRegistrationOpts struct {
	RANUENGAPID            int64
	PDUSessionID           uint8
	ExpectedPDUSessionType uint8
	UE                     *ue.UE
	GnodeB                 *gnb.GnodeB
}

func runInitialRegistration(opts *InitialRegistrationOpts) error {
	err := opts.UE.SendRegistrationRequest(opts.RANUENGAPID, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		return fmt.Errorf("could not build Registration Request NAS PDU: %v", err)
	}

	nasMsg, err := opts.UE.WaitForNASGMMMessage(nas.MsgTypeAuthenticationRequest, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive Registration Accept: %v", err)
	}

	err = validateAuthenticationRequest(
		nasMsg,
	)
	if err != nil {
		return fmt.Errorf("NAS PDU validation failed: %v", err)
	}

	nasMsg, err = opts.UE.WaitForNASGMMMessage(nas.MsgTypeSecurityModeCommand, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive Registration Accept: %v", err)
	}

	err = validateSecurityModeCommand(
		nasMsg,
	)
	if err != nil {
		return fmt.Errorf("could not validate NAS PDU Security Mode Command: %v", err)
	}

	nasMsg, err = opts.UE.WaitForNASGMMMessage(nas.MsgTypeRegistrationAccept, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive Registration Accept: %v", err)
	}

	err = validate.RegistrationAccept(&validate.RegistrationAcceptOpts{
		NASMsg: nasMsg,
		UE:     opts.UE,
		Sst:    opts.GnodeB.SST,
		Sd:     opts.GnodeB.SD,
		Mcc:    opts.GnodeB.MCC,
		Mnc:    opts.GnodeB.MNC,
	})
	if err != nil {
		return fmt.Errorf("validation failed for registration accept: %v", err)
	}

	msg, err := opts.UE.WaitForNASGSMMessage(nas.MsgTypePDUSessionEstablishmentAccept, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	network, err := netip.ParsePrefix("10.45.0.0/16")
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	err = validate.PDUSessionEstablishmentAccept(msg, &validate.ExpectedPDUSessionEstablishmentAccept{
		PDUSessionID:               opts.PDUSessionID,
		PDUSessionType:             opts.ExpectedPDUSessionType,
		UeIPSubnet:                 network,
		Dnn:                        opts.GnodeB.DNN,
		Sst:                        opts.GnodeB.SST,
		Sd:                         opts.GnodeB.SD,
		MaximumBitRateUplinkMbps:   100,
		MaximumBitRateDownlinkMbps: 100,
		Qfi:                        1,
		FiveQI:                     9,
	})
	if err != nil {
		return fmt.Errorf("PDUSessionResourceSetupRequest validation failed: %v", err)
	}

	return nil
}

func validateAuthenticationRequest(nasMsg *nas.Message) error {
	if nasMsg == nil {
		return fmt.Errorf("NAS PDU is nil")
	}

	if nasMsg.GmmMessage == nil {
		return fmt.Errorf("NAS message is not a GMM message")
	}

	if nasMsg.GmmMessage.GetMessageType() != nas.MsgTypeAuthenticationRequest {
		return fmt.Errorf("NAS message type is not Authentication Request (%d), got (%d)", nas.MsgTypeAuthenticationRequest, nasMsg.GmmMessage.GetMessageType())
	}

	if nasMsg.AuthenticationRequest == nil {
		return fmt.Errorf("NAS Authentication Request message is nil")
	}

	if nasMsg.AuthenticationParameterRAND == nil {
		return fmt.Errorf("NAS Authentication Request RAND is nil")
	}

	if reflect.ValueOf(nasMsg.AuthenticationRequest.ExtendedProtocolDiscriminator).IsZero() {
		return fmt.Errorf("extended protocol is missing")
	}

	if nasMsg.AuthenticationRequest.GetExtendedProtocolDiscriminator() != 126 {
		return fmt.Errorf("extended protocol not the expected value")
	}

	if nasMsg.AuthenticationRequest.SpareHalfOctetAndSecurityHeaderType.GetSpareHalfOctet() != 0 {
		return fmt.Errorf("spare half octet not the expected value")
	}

	if nasMsg.AuthenticationRequest.GetSecurityHeaderType() != 0 {
		return fmt.Errorf("security header type not the expected value")
	}

	if reflect.ValueOf(nasMsg.AuthenticationRequest.AuthenticationRequestMessageIdentity).IsZero() {
		return fmt.Errorf("message type is missing")
	}

	if nasMsg.AuthenticationRequest.SpareHalfOctetAndNgksi.GetSpareHalfOctet() != 0 {
		return fmt.Errorf("spare half octet not the expected value")
	}

	if nasMsg.AuthenticationRequest.GetNasKeySetIdentifiler() == 7 {
		return fmt.Errorf("ngKSI not the expected value")
	}

	if reflect.ValueOf(nasMsg.AuthenticationRequest.ABBA).IsZero() {
		return fmt.Errorf("ABBA is missing")
	}

	if nasMsg.AuthenticationRequest.GetABBAContents() == nil {
		return fmt.Errorf("ABBA content is missing")
	}

	return nil
}

func validateSecurityModeCommand(nasMsg *nas.Message) error {
	if nasMsg == nil {
		return fmt.Errorf("NAS PDU is nil")
	}

	if nasMsg.GmmMessage == nil {
		return fmt.Errorf("NAS message is not a GMM message")
	}

	if nasMsg.GmmMessage.GetMessageType() != nas.MsgTypeSecurityModeCommand {
		return fmt.Errorf("NAS message type is not Security Mode Command (%d), got (%d)", nas.MsgTypeSecurityModeCommand, nasMsg.GmmMessage.GetMessageType())
	}

	if reflect.ValueOf(nasMsg.SecurityModeCommand.ExtendedProtocolDiscriminator).IsZero() {
		return fmt.Errorf("extended protocol is missing")
	}

	if nasMsg.SecurityModeCommand.GetExtendedProtocolDiscriminator() != 126 {
		return fmt.Errorf("extended protocol not the expected value")
	}

	if nasMsg.SecurityModeCommand.GetSecurityHeaderType() != 0 {
		return fmt.Errorf("security header type not the expected value")
	}

	if nasMsg.SecurityModeCommand.SpareHalfOctetAndSecurityHeaderType.GetSpareHalfOctet() != 0 {
		return fmt.Errorf("spare half octet not the expected value")
	}

	if reflect.ValueOf(nasMsg.SecurityModeCommand.SecurityModeCommandMessageIdentity).IsZero() {
		return fmt.Errorf("message type is missing")
	}

	if reflect.ValueOf(nasMsg.SecurityModeCommand.SelectedNASSecurityAlgorithms).IsZero() {
		return fmt.Errorf("nas security algorithms is missing")
	}

	if nasMsg.SecurityModeCommand.SpareHalfOctetAndNgksi.GetSpareHalfOctet() != 0 {
		return fmt.Errorf("spare half octet not the expected value")
	}

	if nasMsg.SecurityModeCommand.GetNasKeySetIdentifiler() == 7 {
		return fmt.Errorf("ngKSI not the expected value")
	}

	if reflect.ValueOf(nasMsg.SecurityModeCommand.ReplayedUESecurityCapabilities).IsZero() {
		return fmt.Errorf("replayed ue security capabilities is missing")
	}

	if nasMsg.IMEISVRequest == nil {
		return fmt.Errorf("imeisv request is missing")
	}

	if nasMsg.SelectedNASSecurityAlgorithms.GetTypeOfIntegrityProtectionAlgorithm() != security.AlgIntegrity128NIA2 {
		return fmt.Errorf("integrity protection algorithm not the expected value (got: %d)", nasMsg.SelectedNASSecurityAlgorithms.GetTypeOfIntegrityProtectionAlgorithm())
	}

	if nasMsg.SelectedNASSecurityAlgorithms.GetTypeOfCipheringAlgorithm() != security.AlgCiphering128NEA2 {
		return fmt.Errorf("ciphering algorithm not the expected value (got: %d)", nasMsg.SelectedNASSecurityAlgorithms.GetTypeOfCipheringAlgorithm())
	}

	return nil
}
