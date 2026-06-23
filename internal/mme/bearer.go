// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// defaultERABID is the EPS bearer identity of the default bearer (TS 24.301).
const defaultERABID byte = 5

// t3412PeriodicTAU is the periodic tracking area update timer advertised to the
// UE, GPRS Timer encoded (TS 24.008): unit decihours (bits 8-6 =
// 010), value 9 → 54 minutes, the T3412 default of TS 24.301.
const t3412PeriodicTAU uint8 = 0x49

// bearerStore is the subscription-data surface the MME needs to resolve a
// subscriber's default-bearer QoS. *db.Database satisfies it.
type bearerStore interface {
	GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error)
	GetProfileByID(ctx context.Context, id string) (*db.Profile, error)
	GetDefaultPolicyByProfile(ctx context.Context, profileID string) (*db.Policy, error)
	ListPoliciesByProfile(ctx context.Context, profileID string) ([]db.Policy, error)
	GetDataNetworkByID(ctx context.Context, id string) (*db.DataNetwork, error)
	GetOperator(ctx context.Context) (*db.Operator, error)
	// NodeID is the cluster node identity, used to make each HA node's MME Code
	// (and hence its GUMMEI) distinct.
	NodeID() int
}

// operatorPLMN returns the operator's serving PLMN (TS 23.003), the network's
// identity advertised in S1 Setup and used for K_ASME derivation and the TAI.
func (m *MME) operatorPLMN(ctx context.Context) (models.PlmnID, error) {
	op, err := m.bearer.GetOperator(ctx)
	if err != nil {
		return models.PlmnID{}, fmt.Errorf("get operator: %w", err)
	}

	return models.PlmnID{Mcc: op.Mcc, Mnc: op.Mnc}, nil
}

// defaultMMEGroupID is the fixed MME Group ID of the GUMMEI (TS 23.003).
// Ella Core is a single MME pool, so the group is constant; per-node identity
// comes from the MME Code.
const defaultMMEGroupID uint16 = 1

// mmeIdentity returns the GUMMEI components (TS 23.003): a fixed MME Group
// ID, and an MME Code derived from the cluster node ID so each HA node advertises
// a distinct GUMMEI and a UE's GUTI routes back to its owning node.
// The MME Code is 8 bits, so distinct codes hold for clusters up to 256 nodes;
// beyond that the low 8 bits could collide.
func (m *MME) mmeIdentity() (uint16, uint8) {
	return defaultMMEGroupID, uint8(m.bearer.NodeID() & 0xFF)
}

// operatorTACs returns the operator's supported Tracking Area Codes that are
// valid for E-UTRAN. A TAC is an OCTET STRING, so its configured form is hex
// (matching the AMF, which compares the gNB's hex-encoded TAC against this same
// operator config). The E-UTRAN TAC is 2 octets and the 5GS TAC 3 (TS 23.003), so
// a configured value above 16 bits is a 5GS-only TAC and is excluded here rather
// than narrowed, which would let it falsely match a 16-bit eNB TAC.
func (m *MME) operatorTACs(ctx context.Context) ([]uint16, error) {
	op, err := m.bearer.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("get operator: %w", err)
	}

	tacs, err := op.GetSupportedTacs()
	if err != nil {
		return nil, fmt.Errorf("get supported TACs: %w", err)
	}

	out := make([]uint16, 0, len(tacs))

	for _, t := range tacs {
		n, err := strconv.ParseUint(t, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid TAC %q: %w", t, err)
		}

		if n > math.MaxUint16 {
			continue
		}

		out = append(out, uint16(n))
	}

	return out, nil
}

// operatorTAC returns the operator's first supported Tracking Area Code.
func (m *MME) operatorTAC(ctx context.Context) (uint16, error) {
	tacs, err := m.operatorTACs(ctx)
	if err != nil {
		return 0, err
	}

	if len(tacs) == 0 {
		return 0, fmt.Errorf("operator has no supported TAC")
	}

	return tacs[0], nil
}

// s1apSecurityCapabilities maps a UE's EPS NAS algorithm support to the S1AP UE
// Security Capabilities the eNB selects AS algorithms from. The S1AP BIT STRING
// omits the EEA0/EIA0 (mandatory null-algorithm) bit, so the UE network
// capability octet is shifted left and placed in the high byte (TS 36.413
// §9.2.1.40, TS 33.401).
func s1apSecurityCapabilities(uecap eps.UENetworkCapability) s1ap.UESecurityCapabilities {
	return s1ap.UESecurityCapabilities{
		EncryptionAlgorithms:          uint16(uecap.EEA<<1) << 8,
		IntegrityProtectionAlgorithms: uint16(uecap.EIA<<1) << 8,
	}
}

// epsQoS is the default-bearer QoS resolved from a subscriber's profile/policy.
type epsQoS struct {
	PolicyID string // policy DB ID, so the UPF binds the session to its network rules
	QCI      byte
	ARP      byte // priority level (1-15)
	APN      string
	// AMBRDL/UL is the profile UE-AMBR (bits/s), signaled as the S1AP UE
	// Aggregate Maximum Bit Rate — the per-UE aggregate across all non-GBR bearers.
	AMBRDL uint64
	AMBRUL uint64
	// SessAmbr*Str is the policy per-APN Session-AMBR ("<n> <unit>"), enforced by
	// the UPF QER and signaled to the UE as the APN-AMBR (TS 24.301 §9.9.4.2,
	// §8.3.6.7). Distinct from the UE-AMBR above.
	SessAmbrULStr string
	SessAmbrDLStr string
	IPv4Pool      string // data-network pools; non-empty enables that IP family
	IPv6Pool      string
	DNS           string // data-network DNS server, advertised to the UE via PCO
	MTU           uint16
	Allow4G       bool // whether the subscriber's profile permits EPS/4G access
}

// resolveQoS maps the subscriber's profile → policy → data network to the EPS
// default-bearer QoS. With no S-NSSAI in 4G, the profile's first policy is the
// default bearer.
func (m *MME) resolveQoS(ctx context.Context, imsi string) (*epsQoS, error) {
	sub, err := m.bearer.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("get subscriber: %w", err)
	}

	profile, err := m.bearer.GetProfileByID(ctx, sub.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	// The 4G default bearer uses the profile's default data-network binding (the
	// default APN, TS 23.401).
	pol, err := m.bearer.GetDefaultPolicyByProfile(ctx, sub.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("get default policy: %w", err)
	}

	return m.qosForPolicy(ctx, profile, pol)
}

// ErrUnknownAPN reports that the subscriber's profile has no policy bound to a
// data network with the requested APN, so the PDN connection cannot be
// authorised (TS 24.301 ESM cause #27).
var ErrUnknownAPN = fmt.Errorf("mme: requested APN not in subscriber profile")

// resolveQoSByAPN resolves the EPS QoS for a UE-requested APN by finding the
// subscriber's profile policy whose data network carries that name. It returns
// ErrUnknownAPN when no policy matches, so an unauthorised PDN connectivity
// request is rejected (TS 24.301 §6.5.1.4).
func (m *MME) resolveQoSByAPN(ctx context.Context, imsi, apn string) (*epsQoS, error) {
	sub, err := m.bearer.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("get subscriber: %w", err)
	}

	profile, err := m.bearer.GetProfileByID(ctx, sub.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	policies, err := m.bearer.ListPoliciesByProfile(ctx, sub.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}

	for i := range policies {
		dn, err := m.bearer.GetDataNetworkByID(ctx, policies[i].DataNetworkID)
		if err != nil {
			return nil, fmt.Errorf("get data network: %w", err)
		}

		if dn.Name == apn {
			return m.qosForPolicyDN(profile, &policies[i], dn), nil
		}
	}

	return nil, ErrUnknownAPN
}

// qosForPolicy resolves the data network of a policy and assembles the EPS QoS.
func (m *MME) qosForPolicy(ctx context.Context, profile *db.Profile, pol *db.Policy) (*epsQoS, error) {
	dn, err := m.bearer.GetDataNetworkByID(ctx, pol.DataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("get data network: %w", err)
	}

	return m.qosForPolicyDN(profile, pol, dn), nil
}

// qosForPolicyDN assembles the EPS default-bearer QoS from a profile, policy, and
// its data network.
func (m *MME) qosForPolicyDN(profile *db.Profile, pol *db.Policy, dn *db.DataNetwork) *epsQoS {
	return &epsQoS{
		PolicyID:      pol.ID,
		QCI:           byte(pol.Var5qi), // 5QI↔QCI align for the standardized values
		ARP:           byte(pol.Arp),
		APN:           dn.Name,
		AMBRDL:        bitRateToBps(profile.UeAmbrDownlink),
		AMBRUL:        bitRateToBps(profile.UeAmbrUplink),
		SessAmbrULStr: pol.SessionAmbrUplink,
		SessAmbrDLStr: pol.SessionAmbrDownlink,
		IPv4Pool:      dn.IPv4Pool,
		IPv6Pool:      dn.IPv6Pool,
		DNS:           dn.DNS,
		MTU:           uint16(dn.MTU),
		Allow4G:       profile.Allow4G,
	}
}

// dnFingerprint summarises the data-network parameters delivered to the UE at
// bearer setup (IP pools, DNS, MTU). A change between attach and a later
// reconcile means the UE's bearer must be re-established to pick it up.
func (q *epsQoS) dnFingerprint() string {
	return fmt.Sprintf("%s|%s|%s|%d", q.IPv4Pool, q.IPv6Pool, q.DNS, q.MTU)
}

// bitRateToBps parses an "<n> <unit>" bitrate string (e.g. "1 Gbps") to bits/s.
func bitRateToBps(s string) uint64 {
	parts := strings.Split(s, " ")
	if len(parts) != 2 {
		return 0
	}

	n, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0
	}

	switch parts[1] {
	case "bps":
		return n
	case "Kbps":
		return n * 1_000
	case "Mbps":
		return n * 1_000_000
	case "Gbps":
		return n * 1_000_000_000
	case "Tbps":
		return n * 1_000_000_000_000
	default:
		return 0
	}
}

// activateDefaultBearer builds the Attach Accept (carrying the default-bearer
// activation and the UE IP) and sends it to the eNB inside an Initial Context
// Setup Request, with K_eNB and the UE security capabilities for AS security.
// resolveAttachQoS resolves the default-bearer QoS for an attaching UE. It honours
// a UE-requested APN (TS 24.301 §6.5.1.3) by selecting the policy bound to that data
// network, and falls back to the profile's default policy when no APN is requested.
func (m *MME) resolveAttachQoS(ctx context.Context, ue *UeContext) (*epsQoS, error) {
	if ue.requestedAPN != "" {
		return m.resolveQoSByAPN(ctx, ue.imsi, ue.requestedAPN)
	}

	return m.resolveQoS(ctx, ue.imsi)
}

func (m *MME) activateDefaultBearer(ctx context.Context, ue *UeContext) {
	qos, err := m.resolveAttachQoS(ctx, ue)
	if errors.Is(err, ErrUnknownAPN) {
		// The requested APN is not bound to any policy in the subscriber's profile
		// (TS 24.301 §6.5.1.4, ESM cause #27); the default bearer cannot be set up.
		logger.MmeLog.Info("attach rejected: requested APN not in subscriber profile",
			zap.String("imsi", ue.imsi), zap.String("apn", ue.requestedAPN))
		m.rejectAttach(ctx, ue, emmCauseESMFailure)

		return
	}

	if err != nil {
		logger.MmeLog.Error("failed to resolve subscriber QoS", zap.String("imsi", ue.imsi), zap.Error(err))
		return
	}

	// Subscriber access control (Core Network type restriction, TS 23.501):
	// if the profile does not permit 4G, reject the attach with EMM cause #7 "EPS
	// services not allowed" (TS 24.301).
	if !qos.Allow4G {
		logger.MmeLog.Info("attach rejected: 4G not allowed for subscriber",
			zap.String("imsi", ue.imsi))
		m.rejectAttach(ctx, ue, emmCauseEPSServicesNotAllowed)

		return
	}

	// Delegate the default-bearer session to the SMF+PGW-C anchor: it negotiates
	// the PDN type, allocates the UE address(es), programs the user plane, and
	// returns the S-GW S1-U F-TEID.
	bearer, err := m.session.CreateEPSSession(ctx, models.EPSBearerRequest{
		IMSI:              ue.imsi,
		EPSBearerIdentity: defaultERABID,
		PolicyID:          qos.PolicyID,
		APN:               qos.APN,
		AMBRUplink:        qos.SessAmbrULStr,
		AMBRDownlink:      qos.SessAmbrDLStr,
		IPv4Pool:          qos.IPv4Pool,
		IPv6Pool:          qos.IPv6Pool,
		DNS:               qos.DNS,
		MTU:               qos.MTU,
		RequestedPDNType:  ue.requestedPDNType,
	})
	if err != nil {
		// No PDN type the UE requested can be served (e.g. it asked for IPv6 on an
		// IPv4-only data network). The default bearer cannot be set up, so reject
		// the attach with EMM cause #19 "ESM failure" (TS 24.301).
		logger.MmeLog.Info("attach rejected: default bearer setup failed",
			zap.String("imsi", ue.imsi), zap.Error(err))
		m.rejectAttach(ctx, ue, emmCauseESMFailure)

		return
	}

	ue.ambrUplink = qos.SessAmbrULStr
	ue.ambrDownlink = qos.SessAmbrDLStr

	p := m.addDefaultPDN(ue)
	p.apn = qos.APN
	p.dnConfig = qos.dnFingerprint()
	p.sessAmbrDLBps = bitRateToBps(qos.SessAmbrDLStr)
	p.sessAmbrULBps = bitRateToBps(qos.SessAmbrULStr)
	p.qci = qos.QCI
	p.arp = qos.ARP
	p.pdnType = bearer.PDNType
	p.ueIP = bearer.IPv4
	p.ueIPv6Prefix = bearer.IPv6Prefix
	p.ueIPv6IID = bearer.IPv6IID
	p.dns = bearer.DNS
	p.esmCause = bearer.ESMCause
	p.sgwFTEID = bearer.SGW
	p.sgwN3IPv6 = bearer.SGWN3IPv6

	logger.MmeLog.Info("EPS default bearer established",
		zap.String("imsi", ue.imsi),
		zap.Uint8("pdn-type", p.pdnType),
		zap.String("dns", p.dns.String()),
		zap.Uint8("esm-cause", p.esmCause),
	)

	naspdu, err := m.buildProtectedAttachAccept(ctx, ue, qos)
	if err != nil {
		logger.MmeLog.Error("failed to build Attach Accept", zap.Error(err))
		return
	}

	// On Attach the MME deletes any stored UE Radio Capability and omits it from
	// the Initial Context Setup, so the eNB re-fetches it from the UE (TS 23.401).
	ue.radioCapability = nil

	m.sendInitialContextSetup(ctx, ue, qos, naspdu)

	// Guard the Attach Accept: if the UE does not send Attach Complete, the MME
	// retransmits it and ultimately releases the UE (T3450, TS 24.301).
	m.armNASGuard(ue, "Attach Accept", naspdu)
}

// sendInitialContextSetup establishes the UE's S1 context and default E-RAB at
// the eNB (TS 36.413), with K_eNB and the UE security capabilities for AS
// security. naspdu carries the Attach Accept on attach; it is nil on a Service
// Request, where the EPS bearer context already exists and only the radio and S1
// bearers are re-established.
func (m *MME) sendInitialContextSetup(ctx context.Context, ue *UeContext, qos *epsQoS, naspdu []byte) {
	// K_eNB is derived from the uplink NAS COUNT of the most recently received
	// uplink NAS message (the Security Mode Complete on attach, the Service
	// Request on reconnect), i.e. one less than the next-expected count
	// (TS 33.401).
	kenbCount := ue.ulCount
	if kenbCount > 0 {
		kenbCount--
	}

	kenb, err := deriveKeNB(ue.kasme, kenbCount)
	if err != nil {
		logger.MmeLog.Error("failed to derive K_eNB", zap.Error(err))
		return
	}

	// Seed the X2-handover key chain from this K_eNB: NH(NCC=1) is ready for the
	// first Path Switch (TS 33.401). Re-seeded here on every context
	// setup, so a Service Request that re-derives K_eNB restarts the chain.
	nh, err := deriveNH(ue.kasme, kenb[:])
	if err != nil {
		logger.MmeLog.Error("failed to derive NH", zap.Error(err))
		return
	}

	ue.nh = nh
	ue.ncc = 1

	uecap, err := eps.ParseUENetworkCapability(ue.ueNetCap)
	if err != nil {
		logger.MmeLog.Error("failed to parse UE network capability", zap.Error(err))
		return
	}

	p := m.defaultPDN(ue)
	if p == nil {
		logger.MmeLog.Error("Initial Context Setup with no active PDN", zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)))
		return
	}

	// The S1-U endpoint is advertised as the IPv4, IPv6, or dual-stack transport
	// layer address the N3 has (TS 36.413).
	sgwTLA, err := models.EncodeTransportLayerAddress(p.sgwFTEID.Addr, p.sgwN3IPv6)
	if err != nil {
		logger.MmeLog.Error("failed to encode S-GW transport layer address", zap.Error(err))
		return
	}

	ics := &s1ap.InitialContextSetupRequest{
		MMEUES1APID:               ue.MMEUES1APID,
		ENBUES1APID:               ue.ENBUES1APID,
		UEAggregateMaximumBitRate: s1ap.UEAggregateMaximumBitRate{DL: s1ap.BitRate(qos.AMBRDL), UL: s1ap.BitRate(qos.AMBRUL)},
		ERABToBeSetup: []s1ap.ERABToBeSetupItemCtxtSUReq{{
			ERABID: s1ap.ERABID(p.ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(qos.QCI),
				ARP: s1ap.AllocationAndRetentionPriority{
					PriorityLevel:           qos.ARP,
					PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
					PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
				},
			},
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.sgwFTEID.TEID),
			NASPDU:                s1ap.NASPDU(naspdu),
		}},
		// The eNB selects AS algorithms from these bitmaps.
		UESecurityCapabilities: s1apSecurityCapabilities(uecap),
		SecurityKey:            kenb,
		UERadioCapability:      ue.radioCapability,
	}

	b, err := ics.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Initial Context Setup Request", zap.Error(err))
		return
	}

	// The eNB derives the AS keys from K_eNB and applies the selected algorithms;
	// a mismatch with the UE's own derivation fails the RRC reconfiguration and
	// the eNB releases the UE (TS 33.401). Record the inputs so such a
	// failure can be told apart from a radio-side release.
	logger.MmeLog.Info("Initial Context Setup Request",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.Uint32("enb-ue-id", uint32(ue.ENBUES1APID)),
		zap.String("ue-ip", p.ueIP.String()),
		zap.Uint32("kenb-ul-count", kenbCount),
		zap.Uint8("eea", ue.eea),
		zap.Uint8("eia", ue.eia),
	)
	m.sendS1AP(ctx, ue, S1APProcedureInitialContextSetupRequest, b)
}

// buildProtectedAttachAccept assembles the Attach Accept (with the embedded
// Activate Default EPS Bearer Context Request) and protects it for the UE.
func (m *MME) buildProtectedAttachAccept(ctx context.Context, ue *UeContext, qos *epsQoS) ([]byte, error) {
	p := m.defaultPDN(ue)
	if p == nil {
		return nil, fmt.Errorf("attach accept with no active PDN")
	}

	pti := uint8(0)
	if pc, err := eps.ParsePDNConnectivityRequest(ue.esmContainer); err == nil {
		pti = pc.ProcedureTransactionIdentity
	}

	esm, err := buildActivateDefaultESM(p, qos, pti)
	if err != nil {
		return nil, err
	}

	plmn, err := m.operatorPLMN(ctx)
	if err != nil {
		return nil, err
	}

	tac, err := m.operatorTAC(ctx)
	if err != nil {
		return nil, err
	}

	taiList, err := eps.TAIList{MCC: plmn.Mcc, MNC: plmn.Mnc, TACs: []uint16{tac}}.Marshal()
	if err != nil {
		return nil, err
	}

	mmeGroupID, mmeCode := m.mmeIdentity()

	guti := m.assignGUTI(ue, plmn, mmeGroupID, mmeCode)

	accept := &eps.AttachAccept{
		EPSAttachResult:     eps.AttachResultEPS,
		T3412:               t3412PeriodicTAU,
		TAIList:             taiList,
		ESMMessageContainer: esm,
		GUTI:                &guti,
		// Advertise IMS voice over PS session (TS 24.301). Without it a
		// voice-centric UE concludes E-UTRAN cannot serve voice and leaves for
		// another RAT (TS 23.221).
		EPSNetworkFeatureSupport: &eps.EPSNetworkFeatureSupport{IMSVoPS: true},
	}

	// The MME has no SGs interface, so a combined EPS/IMSI attach succeeds for
	// EPS services only. EMM cause #18 makes the UE stop attempting CS
	// registration on this PLMN (TS 24.301).
	if ue.combinedAttach {
		cause := emmCauseCSDomainNotAvailable
		accept.EMMCause = &cause
	}

	plain, err := accept.Marshal()
	if err != nil {
		return nil, err
	}

	count, knasInt, knasEnc, eia, eea := ue.downlinkSecCtx()

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, uint8(count)),
		nascommon.DirectionDownlink, knasInt, knasEnc, integrityAlg(eia), cipherAlg(eea))
	if err != nil {
		return nil, err
	}

	return wire, nil
}

// onAttachComplete finalises the attach: the UE is EMM-REGISTERED with an active
// default bearer.
func (m *MME) onAttachComplete(ctx context.Context, ue *UeContext, plain []byte) {
	m.stopNASGuard(ue)

	if _, err := eps.ParseAttachComplete(plain); err != nil {
		logger.MmeLog.Warn("failed to decode Attach Complete", zap.Error(err))
		return
	}

	ue.emmState.store(EMMRegistered)

	metrics.RegistrationAttempt(metrics.RAT4G, attachTypeName(ue), metrics.ResultAccept)

	logger.MmeLog.Info("UE attached (EMM-REGISTERED)",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.String("imsi", ue.imsi),
	)

	m.sendNetworkName(ctx, ue)
}

// sendNetworkName provides the operator's network name to the UE in an EMM
// INFORMATION message (TS 24.301). The procedure is optional, so it is skipped
// when no service provider name is configured.
func (m *MME) sendNetworkName(ctx context.Context, ue *UeContext) {
	op, err := m.bearer.GetOperator(ctx)
	if err != nil {
		logger.MmeLog.Warn("failed to get operator for network name", zap.String("imsi", ue.imsi), zap.Error(err))
		return
	}

	if op.SpnFullName == "" && op.SpnShortName == "" {
		return
	}

	m.sendDownlinkProtected(ctx, ue, &eps.EMMInformation{
		FullNetworkName:  op.SpnFullName,
		ShortNetworkName: op.SpnShortName,
	})
}

// buildActivateDefaultESM assembles the ACTIVATE DEFAULT EPS BEARER CONTEXT
// REQUEST for a PDN connection (TS 24.301 §8.3.1): the negotiated PDN address,
// QoS, APN, and the PCO carrying the DNS server and (for IPv4-capable bearers)
// the IPv4 Link MTU. It is carried inside the Attach Accept for the default
// bearer and inside the E-RAB Setup Request for an additional PDN connection.
func buildActivateDefaultESM(p *pdnConnection, qos *epsQoS, pti uint8) ([]byte, error) {
	apn, err := eps.MarshalAPN(qos.APN)
	if err != nil {
		return nil, err
	}

	// PDN Address per the negotiated type (TS 24.301): IPv4 carries the
	// address; IPv6 carries the SLAAC interface identifier (the prefix reaches the
	// UE via Router Advertisement); IPv4v6 carries both.
	var pdnAddr eps.PDNAddress

	switch p.pdnType {
	case eps.PDNTypeIPv6:
		pdnAddr = eps.PDNAddress{PDNType: eps.PDNTypeIPv6, IPv6IID: p.ueIPv6IID}
	case eps.PDNTypeIPv4v6:
		pdnAddr = eps.PDNAddress{PDNType: eps.PDNTypeIPv4v6, IPv4: p.ueIP.As4(), IPv6IID: p.ueIPv6IID}
	default:
		pdnAddr = eps.PDNAddress{PDNType: eps.PDNTypeIPv4, IPv4: p.ueIP.As4()}
	}

	activate := &eps.ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity:            p.ebi,
		ProcedureTransactionIdentity: pti,
		EPSQoS:                       eps.EPSQoS{QCI: qos.QCI}.Marshal(),
		AccessPointName:              apn,
		PDNAddress:                   pdnAddr.Marshal(),
		// Signal the per-APN Session-AMBR so the UE can enforce its uplink share
		// (TS 24.301 §8.3.6.7; the P-GW/UPF also enforces both directions).
		APNAMBR: eps.APNAMBRFromBitsPerSecond(bitRateToBps(qos.SessAmbrDLStr), bitRateToBps(qos.SessAmbrULStr)).Marshal(),
	}

	// Advertise the DNS server and, for IPv4-capable bearers, the IPv4 Link MTU
	// to the UE in the PCO (TS 24.008). SLAAC carries no DNS, so the PCO is the
	// only way an IPv6 UE learns its resolver; the IPv6 link MTU is carried in the
	// Router Advertisement (there is no IPv6 PCO MTU container).
	var dnsServers [][]byte

	if p.dns.IsValid() {
		if p.dns.Is4() {
			b := p.dns.As4()
			dnsServers = [][]byte{b[:]}
		} else {
			b := p.dns.As16()
			dnsServers = [][]byte{b[:]}
		}
	}

	var ipv4LinkMTU uint16
	if p.pdnType == eps.PDNTypeIPv4 || p.pdnType == eps.PDNTypeIPv4v6 {
		ipv4LinkMTU = qos.MTU
	}

	activate.ProtocolConfigurationOptions = eps.BuildProtocolConfigurationOptions(dnsServers, ipv4LinkMTU)

	// On an IPv4v6→single-stack downgrade, tell the UE which family was allowed
	// (#50/#51) so it does not retry the other on this APN (TS 24.301).
	if p.esmCause != 0 {
		cause := p.esmCause
		activate.ESMCause = &cause
	}

	return activate.Marshal()
}

// sendS1AP writes a complete S1AP PDU to the UE's eNB association.
func (m *MME) sendS1AP(ctx context.Context, ue *UeContext, messageType S1APProcedure, b []byte) {
	conn, _, _ := m.s1Identity(ue)
	if conn == nil {
		return
	}

	m.sendS1APConn(ctx, conn, messageType, b)
}

// sendS1APConn writes a complete S1AP PDU to a specific eNB association, used
// when the target is not the UE's current conn (an in-flight S1 handover).
func (m *MME) sendS1APConn(ctx context.Context, conn nasWriter, messageType S1APProcedure, b []byte) {
	ctx, span := tracer.Start(ctx, "s1ap/send",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("s1ap.message_type", string(messageType)),
			attribute.Int("s1ap.message_size", len(b)),
			attribute.String("network.protocol.name", "s1ap"),
			attribute.String("network.transport", "sctp"),
		),
	)
	defer span.End()

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamUE}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send S1AP message")
		logger.MmeLog.Error("failed to send S1AP message", zap.String("message-type", string(messageType)), zap.Error(err))

		return
	}

	m.logOutboundS1AP(ctx, conn, messageType, b)
}
