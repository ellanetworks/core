package ue

import (
	"fmt"
	"net/netip"
	"reflect"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/security"
)

// PDUSessionType is the default PDU Session Type used across UE scenarios.
const PDUSessionType = nasMessage.PDUSessionTypeIPv4

// subscriber is a scenario-local subscriber stub used by multi-UE helpers.
type subscriber struct {
	IMSI           string
	Key            string
	OPc            string
	SequenceNumber string
	ProfileName    string
}

func buildSubscribers(numSubscribers int, startIMSI string) ([]subscriber, error) {
	subs := make([]subscriber, 0, numSubscribers)

	for i := range numSubscribers {
		intBaseIMSI, err := strconv.Atoi(startIMSI)
		if err != nil {
			return nil, fmt.Errorf("failed to convert base IMSI to int: %v", err)
		}

		newIMSI := intBaseIMSI + i
		imsi := fmt.Sprintf("%015d", newIMSI)

		subs = append(subs, subscriber{
			IMSI:           imsi,
			Key:            scenarios.DefaultKey,
			OPc:            scenarios.DefaultOPC,
			SequenceNumber: scenarios.DefaultSequenceNumber,
			ProfileName:    scenarios.DefaultProfileName,
		})
	}

	return subs, nil
}

type initialRegistrationOpts struct {
	RANUENGAPID            int64
	PDUSessionID           uint8
	ExpectedPDUSessionType uint8
	UE                     *ue.UE
	GnodeB                 *gnb.GnodeB
}

// runInitialRegistration walks a UE through the initial registration procedure
// and asserts key NAS message contents along the way. Used by the basic
// registration_success scenarios.
func runInitialRegistration(opts *initialRegistrationOpts) error {
	err := opts.UE.SendRegistrationRequest(opts.RANUENGAPID, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		return fmt.Errorf("could not build Registration Request NAS PDU: %v", err)
	}

	nasMsg, err := opts.UE.WaitForNASGMMMessage(nas.MsgTypeAuthenticationRequest, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive Authentication Request: %v", err)
	}

	err = validateAuthenticationRequest(nasMsg)
	if err != nil {
		return fmt.Errorf("NAS PDU validation failed: %v", err)
	}

	nasMsg, err = opts.UE.WaitForNASGMMMessage(nas.MsgTypeSecurityModeCommand, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive Security Mode Command: %v", err)
	}

	err = validateSecurityModeCommand(nasMsg)
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
		return fmt.Errorf("did not receive PDU Session Establishment Accept: %v", err)
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

// validateRegistrationReject asserts the NAS Registration Reject cause.
func validateRegistrationReject(msg *nas.Message, cause uint8) error {
	if msg == nil {
		return fmt.Errorf("NAS message is nil")
	}

	if msg.GmmMessage == nil {
		return fmt.Errorf("NAS message is not a GMM message")
	}

	if msg.GmmMessage.GetMessageType() != nas.MsgTypeRegistrationReject {
		return fmt.Errorf("NAS message type is not Registration Reject (%d), got (%d)", nas.MsgTypeRegistrationReject, msg.GmmMessage.GetMessageType())
	}

	if msg.RegistrationReject == nil {
		return fmt.Errorf("NAS Registration Reject message is nil")
	}

	if msg.RegistrationReject.GetCauseValue() != cause {
		return fmt.Errorf("NAS Registration Reject Cause is not Unknown UE (%x), received (%x)", cause, msg.RegistrationReject.GetCauseValue())
	}

	return nil
}

// newUEWithDNN builds a UE for the given subscriber, DNN, and default slice.
func newUEWithDNN(gNodeB *gnb.GnodeB, sub subscriber, dnn string) (*ue.UE, error) {
	return ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: PDUSessionType,
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
		DNN:              dnn,
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
}

// newDefaultUE builds a UE using the default test identity and the given
// subscriber sequence number and msin. Used by scenarios that just need a
// basic UE with the standard settings.
func newDefaultUE(gNodeB *gnb.GnodeB, msin, k, opc, sqn string) (*ue.UE, error) {
	return ue.NewUE(&ue.UEOpts{
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: PDUSessionType,
		GnodeB:         gNodeB,
		Msin:           msin,
		K:              k,
		OpC:            opc,
		Amf:            scenarios.DefaultAMF,
		Sqn:            sqn,
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
}

// ueRegistrationTest executes a single UE registration/deregistration using
// the subscriber's configured keys and asserts the PDU Session Establishment
// Accept against exp.
func ueRegistrationTest(ranUENGAPID int64, gNodeB *gnb.GnodeB, sub subscriber, dnn string, exp *validate.ExpectedPDUSessionEstablishmentAccept) error {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: PDUSessionType,
		Msin:           sub.IMSI[5:],
		K:              sub.Key,
		OpC:            sub.OPc,
		Amf:            scenarios.DefaultAMF,
		Sqn:            scenarios.DefaultSequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              dnn,
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

	gNodeB.AddUE(ranUENGAPID, newUE)

	pduSessAcceptMsg, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed for subscriber %v: %v", newUE.UeSecurity.Msin, err)
	}

	err = validate.PDUSessionEstablishmentAccept(pduSessAcceptMsg, exp)
	if err != nil {
		return fmt.Errorf("PDUSessionResourceSetupRequest validation failed: %v", err)
	}

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
