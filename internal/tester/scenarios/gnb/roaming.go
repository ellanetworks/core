// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"crypto/ecdh"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	roamerRegisteredMarker    = "ROAMER_REGISTERED"
	roamingNetworkDeregWait   = 6 * time.Minute
	roamingRegistrationWait   = 30 * time.Second
	roamingTunnelInterface    = "roamer5g0"
	roamingRANUENGAPIDDefault = 1
)

type roamingParams struct {
	IMSI                 string
	Key                  string
	OPc                  string
	SQN                  string
	MCC                  string
	MNC                  string
	HomeMCC              string
	HomeMNC              string
	GNBIndex             int
	Session              bool
	Ping                 bool
	PingDestination      string
	Deregister           bool
	ExpectReject         int
	ExpectAuthReject     bool
	AwaitNetworkDereg    bool
	ExpectDeregCause     int
	ExpectReregistration bool
	SD                   string
	PDUSessionType       string
	ExpectPDUSessionType string
	ExpectGSMCause       int
	SUCIPublicKey        string
	SUCIPublicKeyID      string
	NoS1Mode             bool
	ExpectSMCs           int
	Hold                 time.Duration
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "gnb/roaming/register",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &roamingParams{}
			fs.StringVar(&p.IMSI, "imsi", "", "IMSI of the roamer")
			fs.StringVar(&p.Key, "key", "", "K of the roamer (hex)")
			fs.StringVar(&p.OPc, "opc", "", "OPc of the roamer (hex)")
			fs.StringVar(&p.SQN, "sqn", scenarios.DefaultSequenceNumber, "SQN the USIM holds (hex, 6 octets)")
			fs.StringVar(&p.MCC, "mcc", scenarios.DefaultMCC, "MCC the gNB broadcasts (the visited PLMN)")
			fs.StringVar(&p.MNC, "mnc", scenarios.DefaultMNC, "MNC the gNB broadcasts (the visited PLMN)")
			fs.StringVar(&p.HomeMCC, "home-mcc", "001", "MCC of the roamer's home PLMN (the SUCI home network identifier)")
			fs.StringVar(&p.HomeMNC, "home-mnc", "01", "MNC of the roamer's home PLMN, with its length")
			fs.IntVar(&p.GNBIndex, "gnb-index", 0, "offset added to the gNB ID, so several testers can register through different gNBs")
			fs.BoolVar(&p.Session, "session", true, "establish the default PDU session after the registration")
			fs.BoolVar(&p.Ping, "ping", true, "ping through the default PDU session")
			fs.StringVar(&p.PingDestination, "ping-destination", scenarios.DefaultPingDestination, "address pinged through the PDU session")
			fs.BoolVar(&p.Deregister, "deregister", true, "deregister at the end; when false the UE stays registered and the gNB goes away")
			fs.IntVar(&p.ExpectReject, "expect-reject", 0, "expect a Registration Reject with this 5GMM cause")
			fs.BoolVar(&p.ExpectAuthReject, "expect-auth-reject", false, "expect an Authentication Reject")
			fs.BoolVar(&p.AwaitNetworkDereg, "await-network-deregistration", false, "after the marker, await a network-initiated deregistration")
			fs.IntVar(&p.ExpectDeregCause, "expect-dereg-cause", 0, "5GMM cause the network-initiated deregistration must carry (0: any)")
			fs.BoolVar(&p.ExpectReregistration, "expect-reregistration", false, "the network-initiated deregistration must ask the UE to register again")
			fs.StringVar(&p.SD, "sd", scenarios.DefaultSD, "SD of the slice the gNB supports and the UE requests (empty: no SD)")
			fs.StringVar(&p.PDUSessionType, "pdu-session-type", "ipv4", "PDU session type requested: ipv4, ipv6 or ipv4v6")
			fs.StringVar(&p.ExpectPDUSessionType, "expect-pdu-session-type", "", "PDU session type the session must be accepted with (empty: any)")
			fs.IntVar(&p.ExpectGSMCause, "expect-gsm-cause", -1, "5GSM cause the PDU Session Establishment Accept must carry (0: none, -1: any)")
			fs.StringVar(&p.SUCIPublicKey, "suci-public-key", "", "X25519 home network public key (hex); when set the SUCI is concealed with Profile A")
			fs.StringVar(&p.SUCIPublicKeyID, "suci-public-key-id", "1", "home network public key identifier of the Profile A SUCI")
			fs.BoolVar(&p.NoS1Mode, "no-s1-mode", false, "register as a UE without S1 mode: no S1 mode in the 5GMM capability and no S1 UE network capability")
			fs.IntVar(&p.ExpectSMCs, "expect-security-mode-commands", 0, "number of Security Mode Commands the registration must take (0: any)")
			fs.DurationVar(&p.Hold, "hold", 0, "stay registered this long after the marker before deregistering")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRoamingRegistration(ctx, env, params.(*roamingParams))
		},
	})
}

var roamerPDUSessionTypes = map[string]fgs.PDUSessionType{
	"ipv4":   fgs.PDUSessionTypeIPv4,
	"ipv6":   fgs.PDUSessionTypeIPv6,
	"ipv4v6": fgs.PDUSessionTypeIPv4v6,
}

func roamerSUCIKey(p *roamingParams) (sidf.HomeNetworkPublicKey, error) {
	if p.SUCIPublicKey == "" {
		return sidf.HomeNetworkPublicKey{ProtectionScheme: sidf.NullScheme, PublicKeyID: "0"}, nil
	}

	raw, err := hex.DecodeString(p.SUCIPublicKey)
	if err != nil {
		return sidf.HomeNetworkPublicKey{}, fmt.Errorf("--suci-public-key: %w", err)
	}

	key, err := ecdh.X25519().NewPublicKey(raw)
	if err != nil {
		return sidf.HomeNetworkPublicKey{}, fmt.Errorf("--suci-public-key: %w", err)
	}

	return sidf.HomeNetworkPublicKey{ProtectionScheme: sidf.ProfileAScheme, PublicKey: key, PublicKeyID: p.SUCIPublicKeyID}, nil
}

func roamingUE(gNodeB *gnb.GnodeB, p *roamingParams) (*ue.UE, error) {
	prefix := p.HomeMCC + p.HomeMNC
	if !strings.HasPrefix(p.IMSI, prefix) {
		return nil, fmt.Errorf("--imsi %s is not in the home PLMN %s-%s", p.IMSI, p.HomeMCC, p.HomeMNC)
	}

	sessionType, ok := roamerPDUSessionTypes[p.PDUSessionType]
	if !ok {
		return nil, fmt.Errorf("--pdu-session-type %q is not ipv4, ipv6 or ipv4v6", p.PDUSessionType)
	}

	if _, ok := roamerPDUSessionTypes[p.ExpectPDUSessionType]; p.ExpectPDUSessionType != "" && !ok {
		return nil, fmt.Errorf("--expect-pdu-session-type %q is not ipv4, ipv6 or ipv4v6", p.ExpectPDUSessionType)
	}

	suciKey, err := roamerSUCIKey(p)
	if err != nil {
		return nil, err
	}

	u, err := ue.NewUE(&ue.UEOpts{
		GnodeB:               gNodeB,
		PDUSessionID:         scenarios.DefaultPDUSessionID,
		PDUSessionType:       sessionType,
		Msin:                 p.IMSI[len(prefix):],
		K:                    p.Key,
		OpC:                  p.OPc,
		Amf:                  scenarios.DefaultAMF,
		Sqn:                  p.SQN,
		Mcc:                  p.HomeMCC,
		Mnc:                  p.HomeMNC,
		NoAutoPDUSession:     !p.Session,
		HomeNetworkPublicKey: suciKey,
		RoutingIndicator:     scenarios.DefaultRoutingIndicator,
		DNN:                  scenarios.DefaultDNN,
		Sst:                  scenarios.DefaultSST,
		Sd:                   p.SD,
		IMEISV:               scenarios.DefaultIMEISV,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{Nia2: true},
			Ciphering: testutil.CipheringAlgorithms{Nea0: true, Nea2: true},
		}),
		NoS1Mode: p.NoS1Mode,
	})
	if err != nil {
		return nil, err
	}

	u.SetAmfMccAndMnc(p.MCC, p.MNC)

	return u, nil
}

func runRoamingRegistration(ctx context.Context, env scenarios.Env, p *roamingParams) error {
	if p.IMSI == "" || p.Key == "" || p.OPc == "" {
		return fmt.Errorf("--imsi, --key and --opc are required")
	}

	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           fmt.Sprintf("%06x", 0x000001+p.GNBIndex),
		MCC:             p.MCC,
		MNC:             p.MNC,
		SST:             scenarios.DefaultSST,
		SD:              p.SD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            fmt.Sprintf("Ella-Core-Tester-roaming-%d", p.GNBIndex),
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start gNB: %w", err)
	}

	defer gNodeB.Close()

	if _, err := gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, scenarios.NGSetupTimeout); err != nil {
		return fmt.Errorf("await NG Setup Response: %w", err)
	}

	u, err := roamingUE(gNodeB, p)
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	ranID := int64(roamingRANUENGAPIDDefault)
	gNodeB.AddUE(ranID, u)

	if p.ExpectReject != 0 || p.ExpectAuthReject {
		return roamerRejected(u, ranID, p)
	}

	accept, session, err := registerRoamer(gNodeB, u, ranID, p)
	if err != nil {
		return err
	}

	if err := checkRoamingAccept(accept, p); err != nil {
		return err
	}

	if err := checkRoamerSecurityMode(u, p); err != nil {
		return err
	}

	if p.Session {
		if err := checkRoamerSession(session, p); err != nil {
			return err
		}
	}

	logger.Logger.Info("roamer registered", zap.String("imsi", p.IMSI), zap.String("ue_ipv4", session.UEIPv4))

	if p.Session && p.Ping {
		if err := roamerPing5G(ctx, gNodeB, session, p.PingDestination); err != nil {
			return err
		}
	}

	fmt.Println(roamerRegisteredMarker)

	_ = os.Stdout.Sync()

	if p.Hold > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(p.Hold):
		}
	}

	if p.AwaitNetworkDereg {
		return awaitRoamerDeregistration(u, p)
	}

	if !p.Deregister {
		return nil
	}

	return gNodeB.Deregister(u, ranID, releaseTimeout)
}

func roamerRejected(u *ue.UE, ranID int64, p *roamingParams) error {
	if err := u.SendRegistrationRequest(ranID, uint8(fgs.RegistrationTypeInitial)); err != nil {
		return fmt.Errorf("send Registration Request: %w", err)
	}

	if p.ExpectAuthReject {
		if _, err := u.WaitForNASGMMMessage(uint8(fgs.MsgAuthenticationReject), roamingRegistrationWait); err != nil {
			return fmt.Errorf("await Authentication Reject: %w", err)
		}

		logger.Logger.Info("roamer authentication rejected", zap.String("imsi", p.IMSI))

		return nil
	}

	msg, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationReject), roamingRegistrationWait)
	if err != nil {
		return fmt.Errorf("await Registration Reject: %w", err)
	}

	rej, err := testutil.ExpectNAS[*fgs.RegistrationReject](msg)
	if err != nil {
		return err
	}

	if int(rej.Cause) != p.ExpectReject {
		return fmt.Errorf("registration reject cause = %d, want %d", rej.Cause, p.ExpectReject)
	}

	logger.Logger.Info("roamer rejected", zap.String("imsi", p.IMSI), zap.Uint8("gmm_cause", uint8(rej.Cause)), zap.Bool("t3346", rej.T3346 != nil))

	return nil
}

func registerRoamer(gNodeB *gnb.GnodeB, u *ue.UE, ranID int64, p *roamingParams) (*fgs.RegistrationAccept, gnb.PDUSessionResult, error) {
	if p.Session {
		res, err := gNodeB.Register(u, ranID, scenarios.DefaultPDUSessionID, roamingRegistrationWait)
		if err != nil {
			return nil, gnb.PDUSessionResult{}, fmt.Errorf("register: %w", err)
		}

		return res.Accept, res.Session, nil
	}

	if err := u.SendRegistrationRequest(ranID, uint8(fgs.RegistrationTypeInitial)); err != nil {
		return nil, gnb.PDUSessionResult{}, fmt.Errorf("send Registration Request: %w", err)
	}

	msg, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), roamingRegistrationWait)
	if err != nil {
		return nil, gnb.PDUSessionResult{}, fmt.Errorf("await Registration Accept: %w", err)
	}

	accept, err := fgs.ParseRegistrationAccept(msg)
	if err != nil {
		return nil, gnb.PDUSessionResult{}, fmt.Errorf("parse Registration Accept: %w", err)
	}

	if _, err := u.WaitForNASGMMMessage(uint8(fgs.MsgConfigurationUpdateCommand), roamingRegistrationWait); err != nil {
		return nil, gnb.PDUSessionResult{}, fmt.Errorf("await Configuration Update Command: %w", err)
	}

	return accept, gnb.PDUSessionResult{}, nil
}

func checkRoamingAccept(accept *fgs.RegistrationAccept, p *roamingParams) error {
	if accept == nil {
		return fmt.Errorf("no Registration Accept")
	}

	if len(accept.AllowedNSSAI) == 0 {
		return fmt.Errorf("the Registration Accept allows no S-NSSAI")
	}

	for _, s := range accept.AllowedNSSAI {
		if s.MappedHPLMNSST == nil || *s.MappedHPLMNSST != s.SST {
			return fmt.Errorf("allowed S-NSSAI %+v carries no mapped HPLMN S-NSSAI", s)
		}
	}

	if len(accept.EquivalentPLMNs) != 1 || accept.EquivalentPLMNs[0].MCC != p.MCC || accept.EquivalentPLMNs[0].MNC != p.MNC {
		return fmt.Errorf("equivalent PLMNs %+v, want only the visited PLMN", accept.EquivalentPLMNs)
	}

	if accept.SMSAllowed {
		return fmt.Errorf("SMS over NAS allowed for a roamer")
	}

	if accept.NetworkFeatureSupport != nil && accept.NetworkFeatureSupport.IMSVoPS3GPP {
		return fmt.Errorf("IMS voice over PS indicated for a roamer")
	}

	return nil
}

func checkRoamerSecurityMode(u *ue.UE, p *roamingParams) error {
	smcs := u.ReceivedNASGMMCount(uint8(fgs.MsgSecurityModeCommand))

	logger.Logger.Info("roamer security mode", zap.Int("security_mode_commands", smcs), zap.Bool("eps_nas_algorithms", u.UeSecurity.EPSNASAlgorithms != nil))

	if p.ExpectSMCs != 0 && smcs != p.ExpectSMCs {
		return fmt.Errorf("the registration took %d Security Mode Commands, want %d", smcs, p.ExpectSMCs)
	}

	if p.NoS1Mode && u.UeSecurity.EPSNASAlgorithms != nil {
		return fmt.Errorf("EPS NAS security algorithms selected for a UE without S1 mode")
	}

	return nil
}

func checkRoamerSession(session gnb.PDUSessionResult, p *roamingParams) error {
	accept, err := fgs.ParsePDUSessionEstablishmentAccept(session.Accept)
	if err != nil {
		return fmt.Errorf("parse PDU Session Establishment Accept: %w", err)
	}

	logger.Logger.Info("roamer PDU session",
		zap.Stringer("pdu_session_type", accept.PDUSessionType), zap.Any("gsm_cause", accept.Cause),
		zap.String("ue_ipv4", session.UEIPv4), zap.String("ue_ipv6", session.UEIPv6))

	if want, ok := roamerPDUSessionTypes[p.ExpectPDUSessionType]; ok && accept.PDUSessionType != want {
		return fmt.Errorf("PDU session type = %s, want %s", accept.PDUSessionType, want)
	}

	switch {
	case p.ExpectGSMCause == 0 && accept.Cause != nil:
		return fmt.Errorf("PDU session accepted with 5GSM cause %d, want none", *accept.Cause)
	case p.ExpectGSMCause > 0 && (accept.Cause == nil || int(*accept.Cause) != p.ExpectGSMCause):
		return fmt.Errorf("PDU session 5GSM cause = %v, want #%d", accept.Cause, p.ExpectGSMCause)
	}

	return nil
}

func awaitRoamerDeregistration(u *ue.UE, p *roamingParams) error {
	msg, err := u.WaitForNASGMMMessage(uint8(fgs.MsgDeregistrationRequestUETerm), roamingNetworkDeregWait)
	if err != nil {
		return fmt.Errorf("await the network-initiated Deregistration Request: %w", err)
	}

	req, err := testutil.ExpectNAS[*fgs.DeregistrationRequestUETerminated](msg)
	if err != nil {
		return err
	}

	if p.ExpectDeregCause != 0 && (req.Cause == nil || int(*req.Cause) != p.ExpectDeregCause) {
		return fmt.Errorf("deregistration cause = %v, want %d", req.Cause, p.ExpectDeregCause)
	}

	if req.ReRegistrationRequired != p.ExpectReregistration {
		return fmt.Errorf("re-registration required = %v, want %v", req.ReRegistrationRequired, p.ExpectReregistration)
	}

	logger.Logger.Info("roamer deregistered by the network", zap.String("imsi", p.IMSI))

	return nil
}

func roamerPing5G(ctx context.Context, gNodeB *gnb.GnodeB, session gnb.PDUSessionResult, destination string) error {
	if session.UEIPv4 == "" {
		return fmt.Errorf("the PDU session has no IPv4 address")
	}

	if err := gNodeB.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           session.UEIPv4 + "/16",
		UpfAddress:       session.UpfAddress,
		TunInterfaceName: roamingTunnelInterface,
		ULTEID:           session.ULTEID,
		DLTEID:           session.DLTEID,
		MTU:              session.MTU,
		QFI:              session.QFI,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	defer gNodeB.CloseTunnel(session.DLTEID)

	awaitDownlinkReady()

	if err := probe.Run(ctx, probe.ICMP, roamingTunnelInterface, destination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping %s via %s failed: %w", destination, roamingTunnelInterface, err)
	}

	logger.Logger.Info("roamer data path verified", zap.String("destination", destination))

	return nil
}
