// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type subscriber struct {
	IMSI           string
	Key            string
	OPc            string
	SequenceNumber string
	ProfileName    string
}

func incrementIMSI(base string, offset int) string {
	if len(base) < etsi.MinIMSIDigits || len(base) > etsi.MaxIMSIDigits {
		panic(fmt.Sprintf("incrementIMSI: base must be %d to %d digits", etsi.MinIMSIDigits, etsi.MaxIMSIDigits))
	}

	var n uint64

	for _, ch := range base {
		if ch < '0' || ch > '9' {
			panic("incrementIMSI: base must be numeric")
		}

		n = n*10 + uint64(ch-'0')
	}

	n += uint64(offset)

	out := make([]byte, len(base))
	for i := len(base) - 1; i >= 0; i-- {
		out[i] = byte('0' + n%10)
		n /= 10
	}

	return string(out)
}

func buildSubscribers(numSubscribers int, startIMSI string) ([]subscriber, error) {
	subs := make([]subscriber, 0, numSubscribers)

	for i := range numSubscribers {
		intBaseIMSI, err := strconv.Atoi(startIMSI)
		if err != nil {
			return nil, fmt.Errorf("failed to convert base IMSI to int: %v", err)
		}

		newIMSI := intBaseIMSI + i
		imsi := fmt.Sprintf("%0*d", len(startIMSI), newIMSI)

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

func runInitialRegistration(opts *initialRegistrationOpts) error {
	err := opts.UE.SendRegistrationRequest(opts.RANUENGAPID, uint8(fgs.RegistrationTypeInitial))
	if err != nil {
		return fmt.Errorf("could not build Registration Request NAS PDU: %v", err)
	}

	nasMsg, err := opts.UE.WaitForNASGMMMessage(uint8(fgs.MsgAuthenticationRequest), 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive Authentication Request: %v", err)
	}

	err = validateAuthenticationRequest(nasMsg)
	if err != nil {
		return fmt.Errorf("NAS PDU validation failed: %v", err)
	}

	nasMsg, err = opts.UE.WaitForNASGMMMessage(uint8(fgs.MsgSecurityModeCommand), 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive Security Mode Command: %v", err)
	}

	err = validateSecurityModeCommand(nasMsg)
	if err != nil {
		return fmt.Errorf("could not validate NAS PDU Security Mode Command: %v", err)
	}

	nasMsg, err = opts.UE.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), 1*time.Second)
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

	msg, err := opts.UE.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionEstablishmentAccept), 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive PDU Session Establishment Accept: %v", err)
	}

	network, err := netip.ParsePrefix("10.45.0.0/16")
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	err = validate.PDUSessionEstablishmentAccept(msg, &validate.ExpectedPDUSessionEstablishmentAccept{
		PDUSessionID:               fgs.PDUSessionID(opts.PDUSessionID),
		PDUSessionType:             fgs.PDUSessionType(opts.ExpectedPDUSessionType),
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

func validateAuthenticationRequest(plain []byte) error {
	req, err := testutil.ExpectNAS[*fgs.AuthenticationRequest](plain)
	if err != nil {
		return err
	}

	if req.RAND == nil {
		return fmt.Errorf("NAS Authentication Request RAND is nil")
	}

	if !req.NgKSI.Available() {
		return fmt.Errorf("ngKSI not the expected value")
	}

	if len(req.ABBA) == 0 {
		return fmt.Errorf("ABBA is missing")
	}

	return nil
}

func validateSecurityModeCommand(plain []byte) error {
	smc, err := testutil.ExpectNAS[*fgs.SecurityModeCommand](plain)
	if err != nil {
		return err
	}

	if !smc.NgKSI.Available() {
		return fmt.Errorf("ngKSI not the expected value")
	}

	if smc.ReplayedUESecurityCapability.Equal(fgs.UESecurityCapability{}) {
		return fmt.Errorf("replayed ue security capabilities is missing")
	}

	if smc.IMEISVRequested == nil {
		return fmt.Errorf("imeisv request is missing")
	}

	if smc.IntegrityAlgorithm != nas.IntegrityAES {
		return fmt.Errorf("integrity protection algorithm not the expected value (got: %d)", smc.IntegrityAlgorithm)
	}

	if smc.CipheringAlgorithm != nas.CipheringAES {
		return fmt.Errorf("ciphering algorithm not the expected value (got: %d)", smc.CipheringAlgorithm)
	}

	return nil
}

func validateRegistrationReject(plain []byte, cause uint8) error {
	rej, err := testutil.ExpectNAS[*fgs.RegistrationReject](plain)
	if err != nil {
		return err
	}

	if rej.Cause != fgs.GMMCause(cause) {
		return fmt.Errorf("NAS Registration Reject Cause is not Unknown UE (%x), received (%x)", cause, rej.Cause)
	}

	return nil
}

func newUEWithDNN(gNodeB *gnb.GnodeB, sub subscriber, dnn string, pduSessionType uint8) (*ue.UE, error) {
	return ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: fgs.PDUSessionType(pduSessionType),
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

func newDefaultUE(gNodeB *gnb.GnodeB, msin, k, opc, sqn string, pduSessionType uint8) (*ue.UE, error) {
	return ue.NewUE(defaultUEOpts(gNodeB, msin, k, opc, sqn, pduSessionType))
}

// newSignallingOnlyUE builds a UE that registers without establishing its
// default PDU session, the way a handset registers when it has nothing to send
// yet.
func newSignallingOnlyUE(gNodeB *gnb.GnodeB, msin, k, opc, sqn string, pduSessionType uint8) (*ue.UE, error) {
	opts := defaultUEOpts(gNodeB, msin, k, opc, sqn, pduSessionType)
	opts.NoAutoPDUSession = true

	return ue.NewUE(opts)
}

func defaultUEOpts(gNodeB *gnb.GnodeB, msin, k, opc, sqn string, pduSessionType uint8) *ue.UEOpts {
	return &ue.UEOpts{
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: fgs.PDUSessionType(pduSessionType),
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
	}
}

func ueRegistrationTest(ranUENGAPID int64, gNodeB *gnb.GnodeB, sub subscriber, dnn string, exp *validate.ExpectedPDUSessionEstablishmentAccept, pduSessionType uint8) error {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: fgs.PDUSessionType(pduSessionType),
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

	registration, err := gNodeB.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("initial registration procedure failed for subscriber %v: %v", newUE.UeSecurity.Msin, err)
	}

	err = validate.PDUSessionEstablishmentAccept(registration.Session.Accept, exp)
	if err != nil {
		return fmt.Errorf("PDUSessionResourceSetupRequest validation failed: %v", err)
	}

	err = gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout)
	if err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
