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
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

func activateDefaultBearer(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	qos, err := mme.ResolveAttachQoS(m, ctx, ue)
	if errors.Is(err, mme.ErrUnknownAPN) {
		// The requested APN is not bound to any policy in the subscriber's profile
		// (TS 24.301 §6.5.1.4, ESM cause #27).
		logger.From(ctx, logger.MmeLog).Info("attach rejected: requested APN not in subscriber profile",
			zap.String("imsi", ue.IMSI()), zap.String("apn", ue.RequestedAPN))
		rejectAttach(m, ctx, ue, mme.EmmCauseESMFailure)

		return
	}

	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to resolve subscriber QoS", zap.String("imsi", ue.IMSI()), zap.Error(err))
		return
	}

	// A profile that does not permit 4G (Core Network type restriction, TS 23.501)
	// is rejected with EMM cause #7 "EPS services not allowed" (TS 24.301).
	if !qos.Allow4G {
		logger.From(ctx, logger.MmeLog).Info("attach rejected: 4G not allowed for subscriber",
			zap.String("imsi", ue.IMSI()))
		rejectAttach(m, ctx, ue, mme.EmmCauseEPSServicesNotAllowed)

		return
	}

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
		// IPv4-only data network); reject with EMM cause #19 "ESM failure" (TS 24.301).
		logger.From(ctx, logger.MmeLog).Info("attach rejected: default bearer setup failed",
			zap.String("imsi", ue.IMSI()), zap.Error(err))
		rejectAttach(m, ctx, ue, mme.EmmCauseESMFailure)

		return
	}

	pdnType, dns, esmCause := m.InstallDefaultBearer(ue, qos, bearer)

	logger.From(ctx, logger.MmeLog).Info("EPS default bearer established",
		zap.String("imsi", ue.IMSI()),
		zap.Uint8("pdn-type", pdnType),
		zap.String("dns", dns),
		zap.Uint8("esm-cause", esmCause),
	)

	naspdu, err := buildProtectedAttachAccept(m, ctx, ue, qos)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Attach Accept", zap.Error(err))
		return
	}

	// Supersede any prior context for this subscriber only once the attach is
	// authenticated and accepted, so an unauthenticated attach cannot tear down a
	// registered UE (TS 24.501 §4.4.4.3 analogue).
	m.CommitUEIdentity(ctx, ue, mme.MintAuthProofForAttachCommit())

	// Drop any stored UE Radio Capability and omit it from the Initial Context
	// Setup, so the eNB re-fetches it from the UE (TS 23.401).
	ue.RadioCapability = nil

	ue.AdvanceRegStep(mme.RegStepContextSetup)

	// Keep the sent Attach Accept so a duplicate Attach Request with identical IEs
	// can be answered by resending it (TS 24.301 §5.5.1.2.7 case d).
	ue.Conn().AttachAcceptPdu = naspdu

	sendInitialContextSetup(m, ctx, ue, qos, naspdu)

	// T3450 retransmits the Attach Accept, then releases the UE, if no Attach
	// Complete arrives (TS 24.301).
	ue.Conn().ArmNASGuard("Attach Accept", naspdu)
}

// sendInitialContextSetup establishes the UE's S1 context and default E-RAB at the
// eNB (TS 36.413). naspdu carries the Attach Accept on attach; it is nil on a
// Service Request, where only the radio and S1 bearers are re-established.
func sendInitialContextSetup(m *mme.MME, ctx context.Context, ue *mme.UeContext, qos *mme.EpsQoS, naspdu []byte) {
	// Derive K_eNB and seed the X2-handover key chain (NH for NCC=1). Re-seeded on
	// every context setup, so a Service Request restarts the chain (TS 33.401).
	kenb, kenbCount, err := ue.DeriveInitialKeNB()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to derive AS keys", zap.Error(err))
		return
	}

	uecap, err := eps.ParseUENetworkCapability(ue.UeNetCap)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to parse UE network capability", zap.Error(err))
		return
	}

	defaultPDN := m.DefaultPDN(ue)
	if defaultPDN == nil {
		logger.From(ctx, logger.MmeLog).Error("Initial Context Setup with no active PDN")
		return
	}

	// A UE re-established from ECM-IDLE reactivates the radio and S1 bearers for all
	// the active EPS bearers in one Initial Context Setup; the S1 Service Request has
	// no per-bearer data-status IE, so every active bearer is set up (TS 23.401
	// §5.3.4.1). The NAS PDU (the Attach Accept, on attach only) rides the default
	// bearer.
	pdns := m.SnapshotPDNs(ue)
	erabs := make([]s1ap.ERABToBeSetupItemCtxtSUReq, 0, len(pdns))

	for _, p := range pdns {
		// The S1-U endpoint advertises whichever transport address family the N3 has
		// (TS 36.413).
		sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
		if err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to encode S-GW transport layer address",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", p.Ebi), zap.Error(err))

			continue
		}

		item := s1ap.ERABToBeSetupItemCtxtSUReq{
			ERABID: s1ap.ERABID(p.Ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(p.Qci),
				ARP: s1ap.AllocationAndRetentionPriority{
					PriorityLevel:           p.Arp,
					PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
					PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
				},
			},
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.SgwFTEID.TEID),
		}

		if p.Ebi == defaultPDN.Ebi {
			item.NASPDU = s1ap.NASPDU(naspdu)
		}

		erabs = append(erabs, item)
	}

	if len(erabs) == 0 {
		logger.From(ctx, logger.MmeLog).Error("Initial Context Setup with no encodable E-RAB", zap.String("imsi", ue.IMSI()))
		return
	}

	ics := &s1ap.InitialContextSetupRequest{
		UEAggregateMaximumBitRate: s1ap.UEAggregateMaximumBitRate{DL: s1ap.BitRate(qos.AMBRDL), UL: s1ap.BitRate(qos.AMBRUL)},
		ERABToBeSetup:             erabs,
		UESecurityCapabilities:    mme.S1apSecurityCapabilities(uecap),
		SecurityKey:               kenb,
		UERadioCapability:         ue.RadioCapability,
	}

	// Log the AS-key inputs so an eNB RRC-reconfiguration failure from a key or
	// algorithm mismatch can be told apart from a radio-side release (TS 33.401).
	logger.From(ctx, logger.MmeLog).Info("Initial Context Setup Request",
		zap.Uint32("enb-ue-id", uint32(ue.Conn().ENBUES1APID)),
		zap.String("ue-ip", defaultPDN.UeIP.String()),
		zap.Int("bearers", len(erabs)),
		zap.Uint32("kenb-ul-count", kenbCount),
		zap.Uint8("eea", ue.EEA()),
		zap.Uint8("eia", ue.EIA()),
	)

	if err := ue.Conn().SendInitialContextSetup(ctx, ics); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to send Initial Context Setup Request", zap.Error(err))
		return
	}

	ue.Conn().ICS = mme.ICSPending
}

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

	guti := m.ReallocateGUTI(ue, plmn, mmeGroupID, mmeCode)

	t3412, err := eps.EncodeGPRSTimer(mme.T3412PeriodicTAU)
	if err != nil {
		return nil, fmt.Errorf("encode T3412: %w", err)
	}

	accept := &eps.AttachAccept{
		EPSAttachResult:     eps.AttachResultEPS,
		T3412:               t3412,
		TAIList:             taiList,
		ESMMessageContainer: esm,
		GUTI:                &guti,
		// Advertise IMS voice over PS session (TS 24.301). Without it a
		// voice-centric UE concludes E-UTRAN cannot serve voice and leaves for
		// another RAT (TS 23.221).
		EPSNetworkFeatureSupport: m.NetworkFeatureSupport(),
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

func handleAttachComplete(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	if ue.RegStep() != mme.RegStepContextSetup {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Attach Complete outside the context-setup sub-phase")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ue.Conn().StopNASGuard()

	if _, err := eps.ParseAttachComplete(plain); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Attach Complete", zap.Error(err))
		return nasreply.Handled()
	}

	m.CommitGUTIRealloc(ue)

	ue.TransitionTo(mme.EMMRegistered)

	metrics.RegistrationAttempt(metrics.RAT4G, attachTypeName(ue), metrics.ResultAccept)

	logger.From(ctx, logger.MmeLog).Info("UE attached (EMM-REGISTERED)",
		zap.String("imsi", ue.IMSI()),
	)

	sendNetworkName(m, ctx, ue)

	return nasreply.Handled()
}

// sendNetworkName provides the operator's network name to the UE in an EMM
// INFORMATION message (TS 24.301).
func sendNetworkName(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	op, err := m.Bearer.GetOperator(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to get operator for network name", zap.String("imsi", ue.IMSI()), zap.Error(err))
		return
	}

	if op.SpnFullName == "" && op.SpnShortName == "" {
		return
	}

	ue.Conn().SendDownlinkProtected(ctx, &eps.EMMInformation{
		FullNetworkName:  op.SpnFullName,
		ShortNetworkName: op.SpnShortName,
	})
}

// buildActivateDefaultESM assembles the ACTIVATE DEFAULT EPS BEARER CONTEXT
// REQUEST for a PDN connection (TS 24.301 §8.3.1).
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

	// Advertise DNS and, for IPv4-capable bearers, the IPv4 Link MTU in the PCO
	// (TS 24.008): SLAAC carries no DNS, so the PCO is the only way an IPv6 UE learns
	// its resolver; the IPv6 link MTU rides the Router Advertisement.
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
