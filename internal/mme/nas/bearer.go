// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// t3412PeriodicTAU is the periodic tracking area update timer advertised to the
// UE, GPRS Timer encoded (TS 24.008): unit decihours (bits 8-6 =
// 010), value 9 → 54 minutes, the T3412 default of TS 24.301.
const t3412PeriodicTAU uint8 = 0x49

// activateDefaultBearer builds the Attach Accept (carrying the default-bearer
// activation and the UE IP) and sends it to the eNB inside an Initial Context
// Setup Request, with K_eNB and the UE security capabilities for AS security.
func activateDefaultBearer(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	qos, err := mme.ResolveAttachQoS(m, ctx, ue)
	if errors.Is(err, mme.ErrUnknownAPN) {
		// The requested APN is not bound to any policy in the subscriber's profile
		// (TS 24.301 §6.5.1.4, ESM cause #27); the default bearer cannot be set up.
		logger.MmeLog.Info("attach rejected: requested APN not in subscriber profile",
			zap.String("imsi", ue.IMSI()), zap.String("apn", ue.RequestedAPN))
		rejectAttach(m, ctx, ue, mme.EmmCauseESMFailure)

		return
	}

	if err != nil {
		logger.MmeLog.Error("failed to resolve subscriber QoS", zap.String("imsi", ue.IMSI()), zap.Error(err))
		return
	}

	// Subscriber access control (Core Network type restriction, TS 23.501):
	// if the profile does not permit 4G, reject the attach with EMM cause #7 "EPS
	// services not allowed" (TS 24.301).
	if !qos.Allow4G {
		logger.MmeLog.Info("attach rejected: 4G not allowed for subscriber",
			zap.String("imsi", ue.IMSI()))
		rejectAttach(m, ctx, ue, mme.EmmCauseEPSServicesNotAllowed)

		return
	}

	// Delegate the default-bearer session to the SMF+PGW-C anchor: it negotiates
	// the PDN type, allocates the UE address(es), programs the user plane, and
	// returns the S-GW S1-U F-TEID.
	bearer, err := m.Session.CreateEPSSession(ctx, models.EPSBearerRequest{
		IMSI:              ue.IMSI(),
		EPSBearerIdentity: mme.DefaultERABID,
		PolicyID:          qos.PolicyID,
		APN:               qos.APN,
		AMBRUplink:        qos.SessAmbrULStr,
		AMBRDownlink:      qos.SessAmbrDLStr,
		IPv4Pool:          qos.IPv4Pool,
		IPv6Pool:          qos.IPv6Pool,
		DNS:               qos.DNS,
		MTU:               qos.MTU,
		RequestedPDNType:  ue.RequestedPDNType,
	})
	if err != nil {
		// No PDN type the UE requested can be served (e.g. it asked for IPv6 on an
		// IPv4-only data network). The default bearer cannot be set up, so reject
		// the attach with EMM cause #19 "ESM failure" (TS 24.301).
		logger.MmeLog.Info("attach rejected: default bearer setup failed",
			zap.String("imsi", ue.IMSI()), zap.Error(err))
		rejectAttach(m, ctx, ue, mme.EmmCauseESMFailure)

		return
	}

	ue.AmbrUplink = qos.SessAmbrULStr
	ue.AmbrDownlink = qos.SessAmbrDLStr

	p := m.AddDefaultPDN(ue)
	p.Apn = qos.APN
	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = mme.BitRateToBps(qos.SessAmbrDLStr)
	p.SessAmbrULBps = mme.BitRateToBps(qos.SessAmbrULStr)
	p.Qci = qos.QCI
	p.Arp = qos.ARP
	p.PdnType = bearer.PDNType
	p.UeIP = bearer.IPv4
	p.UeIPv6Prefix = bearer.IPv6Prefix
	p.UeIPv6IID = bearer.IPv6IID
	p.Dns = bearer.DNS
	p.EsmCause = bearer.ESMCause
	p.SgwFTEID = bearer.SGW
	p.SgwN3IPv6 = bearer.SGWN3IPv6

	logger.MmeLog.Info("EPS default bearer established",
		zap.String("imsi", ue.IMSI()),
		zap.Uint8("pdn-type", p.PdnType),
		zap.String("dns", p.Dns.String()),
		zap.Uint8("esm-cause", p.EsmCause),
	)

	naspdu, err := buildProtectedAttachAccept(m, ctx, ue, qos)
	if err != nil {
		logger.MmeLog.Error("failed to build Attach Accept", zap.Error(err))
		return
	}

	// The attach is now authenticated and accepted, so this context becomes the
	// subscriber's single registered context, superseding any prior one. Doing
	// this here (not when the IMSI was first learned) keeps an unauthenticated
	// attach from tearing down a registered UE (TS 24.501 §4.4.4.3 analogue).
	m.CommitUEIdentity(ue, mme.MintAuthProofForAttachCommit())

	// On Attach the MME deletes any stored UE Radio Capability and omits it from
	// the Initial Context Setup, so the eNB re-fetches it from the UE (TS 23.401).
	ue.RadioCapability = nil

	sendInitialContextSetup(m, ctx, ue, qos, naspdu)

	// Guard the Attach Accept: if the UE does not send Attach Complete, the MME
	// retransmits it and ultimately releases the UE (T3450, TS 24.301).
	m.ArmNASGuard(ue, "Attach Accept", naspdu)
}

// sendInitialContextSetup establishes the UE's S1 context and default E-RAB at
// the eNB (TS 36.413), with K_eNB and the UE security capabilities for AS
// security. naspdu carries the Attach Accept on attach; it is nil on a Service
// Request, where the EPS bearer context already exists and only the radio and S1
// bearers are re-established.
func sendInitialContextSetup(m *mme.MME, ctx context.Context, ue *mme.UeContext, qos *mme.EpsQoS, naspdu []byte) {
	// Derive K_eNB for delivery to the eNB and seed the X2-handover key chain
	// (NH for NCC=1, ready for the first Path Switch). Re-seeded on every context
	// setup, so a Service Request that re-derives K_eNB restarts the chain
	// (TS 33.401).
	kenb, kenbCount, err := ue.DeriveInitialKeNB()
	if err != nil {
		logger.MmeLog.Error("failed to derive AS keys", zap.Error(err))
		return
	}

	uecap, err := eps.ParseUENetworkCapability(ue.UeNetCap)
	if err != nil {
		logger.MmeLog.Error("failed to parse UE network capability", zap.Error(err))
		return
	}

	p := m.DefaultPDN(ue)
	if p == nil {
		logger.MmeLog.Error("Initial Context Setup with no active PDN", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
		return
	}

	// The S1-U endpoint is advertised as the IPv4, IPv6, or dual-stack transport
	// layer address the N3 has (TS 36.413).
	sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
	if err != nil {
		logger.MmeLog.Error("failed to encode S-GW transport layer address", zap.Error(err))
		return
	}

	ics := &s1ap.InitialContextSetupRequest{
		MMEUES1APID:               ue.S1.MMEUES1APID,
		ENBUES1APID:               ue.S1.ENBUES1APID,
		UEAggregateMaximumBitRate: s1ap.UEAggregateMaximumBitRate{DL: s1ap.BitRate(qos.AMBRDL), UL: s1ap.BitRate(qos.AMBRUL)},
		ERABToBeSetup: []s1ap.ERABToBeSetupItemCtxtSUReq{{
			ERABID: s1ap.ERABID(p.Ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(qos.QCI),
				ARP: s1ap.AllocationAndRetentionPriority{
					PriorityLevel:           qos.ARP,
					PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
					PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
				},
			},
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.SgwFTEID.TEID),
			NASPDU:                s1ap.NASPDU(naspdu),
		}},
		// The eNB selects AS algorithms from these bitmaps.
		UESecurityCapabilities: mme.S1apSecurityCapabilities(uecap),
		SecurityKey:            kenb,
		UERadioCapability:      ue.RadioCapability,
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
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.Uint32("enb-ue-id", uint32(ue.S1.ENBUES1APID)),
		zap.String("ue-ip", p.UeIP.String()),
		zap.Uint32("kenb-ul-count", kenbCount),
		zap.Uint8("eea", ue.EEA()),
		zap.Uint8("eia", ue.EIA()),
	)
	m.SendS1AP(ctx, ue, mme.S1APProcedureInitialContextSetupRequest, b)
}

// buildProtectedAttachAccept assembles the Attach Accept (with the embedded
// Activate Default EPS Bearer Context Request) and protects it for the UE.
func buildProtectedAttachAccept(m *mme.MME, ctx context.Context, ue *mme.UeContext, qos *mme.EpsQoS) ([]byte, error) {
	p := m.DefaultPDN(ue)
	if p == nil {
		return nil, fmt.Errorf("attach accept with no active PDN")
	}

	pti := uint8(0)
	if pc, err := eps.ParsePDNConnectivityRequest(ue.EsmContainer); err == nil {
		pti = pc.ProcedureTransactionIdentity
	}

	esm, err := buildActivateDefaultESM(p, qos, pti)
	if err != nil {
		return nil, err
	}

	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		return nil, err
	}

	tac, err := m.OperatorTAC(ctx)
	if err != nil {
		return nil, err
	}

	taiList, err := eps.TAIList{MCC: plmn.Mcc, MNC: plmn.Mnc, TACs: []uint16{tac}}.Marshal()
	if err != nil {
		return nil, err
	}

	mmeGroupID, mmeCode := m.MmeIdentity()

	guti := m.AssignGUTI(ue, plmn, mmeGroupID, mmeCode)

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
	if ue.CombinedAttach {
		cause := mme.EmmCauseCSDomainNotAvailable
		accept.EMMCause = &cause
	}

	plain, err := accept.Marshal()
	if err != nil {
		return nil, err
	}

	wire, err := ue.ProtectDownlink(plain, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		return nil, err
	}

	return wire, nil
}

// handleAttachComplete finalises the attach: the UE is EMM-REGISTERED with an active
// default bearer.
func handleAttachComplete(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	m.StopNASGuard(ue)

	if _, err := eps.ParseAttachComplete(plain); err != nil {
		logger.MmeLog.Warn("failed to decode Attach Complete", zap.Error(err))
		return
	}

	ue.SetEMMState(mme.EMMRegistered)

	metrics.RegistrationAttempt(metrics.RAT4G, attachTypeName(ue), metrics.ResultAccept)

	logger.MmeLog.Info("UE attached (EMM-REGISTERED)",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.String("imsi", ue.IMSI()),
	)

	sendNetworkName(m, ctx, ue)
}

// sendNetworkName provides the operator's network name to the UE in an EMM
// INFORMATION message (TS 24.301). The procedure is optional, so it is skipped
// when no service provider name is configured.
func sendNetworkName(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	op, err := m.Bearer.GetOperator(ctx)
	if err != nil {
		logger.MmeLog.Warn("failed to get operator for network name", zap.String("imsi", ue.IMSI()), zap.Error(err))
		return
	}

	if op.SpnFullName == "" && op.SpnShortName == "" {
		return
	}

	m.SendDownlinkProtected(ctx, ue, &eps.EMMInformation{
		FullNetworkName:  op.SpnFullName,
		ShortNetworkName: op.SpnShortName,
	})
}

// buildActivateDefaultESM assembles the ACTIVATE DEFAULT EPS BEARER CONTEXT
// REQUEST for a PDN connection (TS 24.301 §8.3.1): the negotiated PDN address,
// QoS, APN, and the PCO carrying the DNS server and (for IPv4-capable bearers)
// the IPv4 Link MTU. It is carried inside the Attach Accept for the default
// bearer and inside the E-RAB Setup Request for an additional PDN connection.
func buildActivateDefaultESM(p *mme.PdnConnection, qos *mme.EpsQoS, pti uint8) ([]byte, error) {
	apn, err := eps.MarshalAPN(qos.APN)
	if err != nil {
		return nil, err
	}

	// PDN Address per the negotiated type (TS 24.301): IPv4 carries the
	// address; IPv6 carries the SLAAC interface identifier (the prefix reaches the
	// UE via Router Advertisement); IPv4v6 carries both.
	var pdnAddr eps.PDNAddress

	switch p.PdnType {
	case eps.PDNTypeIPv6:
		pdnAddr = eps.PDNAddress{PDNType: eps.PDNTypeIPv6, IPv6IID: p.UeIPv6IID}
	case eps.PDNTypeIPv4v6:
		pdnAddr = eps.PDNAddress{PDNType: eps.PDNTypeIPv4v6, IPv4: p.UeIP.As4(), IPv6IID: p.UeIPv6IID}
	default:
		pdnAddr = eps.PDNAddress{PDNType: eps.PDNTypeIPv4, IPv4: p.UeIP.As4()}
	}

	activate := &eps.ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity:            p.Ebi,
		ProcedureTransactionIdentity: pti,
		EPSQoS:                       eps.EPSQoS{QCI: qos.QCI}.Marshal(),
		AccessPointName:              apn,
		PDNAddress:                   pdnAddr.Marshal(),
		// Signal the per-APN Session-AMBR so the UE can enforce its uplink share
		// (TS 24.301 §8.3.6.7; the P-GW/UPF also enforces both directions).
		APNAMBR: eps.APNAMBRFromBitsPerSecond(mme.BitRateToBps(qos.SessAmbrDLStr), mme.BitRateToBps(qos.SessAmbrULStr)).Marshal(),
	}

	// Advertise the DNS server and, for IPv4-capable bearers, the IPv4 Link MTU
	// to the UE in the PCO (TS 24.008). SLAAC carries no DNS, so the PCO is the
	// only way an IPv6 UE learns its resolver; the IPv6 link MTU is carried in the
	// Router Advertisement (there is no IPv6 PCO MTU container).
	var dnsServers [][]byte

	if p.Dns.IsValid() {
		if p.Dns.Is4() {
			b := p.Dns.As4()
			dnsServers = [][]byte{b[:]}
		} else {
			b := p.Dns.As16()
			dnsServers = [][]byte{b[:]}
		}
	}

	var ipv4LinkMTU uint16
	if p.PdnType == eps.PDNTypeIPv4 || p.PdnType == eps.PDNTypeIPv4v6 {
		ipv4LinkMTU = qos.MTU
	}

	activate.ProtocolConfigurationOptions = eps.BuildProtocolConfigurationOptions(dnsServers, ipv4LinkMTU)

	// On an IPv4v6→single-stack downgrade, tell the UE which family was allowed
	// (#50/#51) so it does not retry the other on this APN (TS 24.301).
	if p.EsmCause != 0 {
		cause := p.EsmCause
		activate.ESMCause = &cause
	}

	return activate.Marshal()
}
