// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// registrationAreaTAIList builds a UE's registration area as the TAI list IE for
// ATTACH/TAU ACCEPT (TS 24.301 §9.9.3.33).
func registrationAreaTAIList(area []models.Tai) (eps.TAIList, error) {
	tais := make([]eps.TAI, 0, len(area))

	for _, t := range area {
		if t.PlmnID == nil {
			return nil, fmt.Errorf("tai has no PLMN ID")
		}

		tac, err := strconv.ParseUint(t.Tac, 16, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid TAC %q: %w", t.Tac, err)
		}

		tais = append(tais, eps.TAI{PLMN: nas.PLMN{MCC: t.PlmnID.Mcc, MNC: t.PlmnID.Mnc}, TAC: uint16(tac)})
	}

	return eps.NewTAIList(tais...)
}

func activateDefaultBearer(ctx context.Context, m *mme.MME, ue *mme.UeContext) {
	if requestESMInformation(ctx, m, ue) {
		return
	}

	qos, err := mme.ResolveAttachQoS(ctx, m, ue)
	if errors.Is(err, mme.ErrUnknownAPN) {
		// The requested APN is not bound to any policy in the subscriber's profile
		// (TS 24.301 §6.5.1.4, ESM cause #27).
		logger.From(ctx, logger.MmeLog).Info("attach rejected: requested APN not in subscriber profile",
			zap.String("imsi", ue.IMSI()), zap.String("apn", ue.RequestedAPN))
		rejectAttachESM(ctx, m, ue, eps.ESMCauseMissingOrUnknownAPN)

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
		rejectAttach(ctx, m, ue, eps.EMMCauseEPSServicesNotAllowed)

		return
	}

	bearer, err := m.Session.CreateEPSSession(ctx, models.EPSBearerRequest{
		IMSI:              ue.IMSI(),
		EPSBearerIdentity: mme.DefaultERABID,
		PDUSessionID:      ue.RequestedPDUSessionID,
		RequestType:       ue.RequestedType,
		Snssai:            &qos.Snssai,
		PolicyID:          qos.PolicyID,
		APN:               qos.APN,
		AMBRUplink:        qos.SessAmbrUL,
		AMBRDownlink:      qos.SessAmbrDL,
		IPv4Pool:          qos.IPv4Pool,
		IPv6Pool:          qos.IPv6Pool,
		DNS:               qos.DNS,
		MTU:               qos.MTU,
		RequestedPDNType:  ue.RequestedPDNType,
	})
	if err != nil {
		logger.From(ctx, logger.MmeLog).Info("attach rejected: default bearer setup failed",
			zap.String("imsi", ue.IMSI()), zap.Error(err))
		rejectAttachESM(ctx, m, ue, sessionSetupESMCause(bearer))

		return
	}

	pdnType, dns, esmCause := m.InstallDefaultBearer(ue, qos, bearer)

	logger.From(ctx, logger.MmeLog).Info("EPS default bearer established",
		zap.String("imsi", ue.IMSI()),
		zap.Uint8("pdn-type", pdnType),
		zap.String("dns", dns),
		zap.Stringer("esm-cause", esmCause),
	)

	naspdu, err := buildProtectedAttachAccept(ctx, m, ue, qos)
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

	sendInitialContextSetup(ctx, m, ue, qos, naspdu)

	// T3450 retransmits the Attach Accept, then releases the UE, if no Attach
	// Complete arrives (TS 24.301).
	ue.Conn().ArmNASGuard("Attach Accept", naspdu)
}

// sendInitialContextSetup establishes the UE's S1 context and default E-RAB at the
// eNB (TS 36.413). naspdu carries the Attach Accept on attach; it is nil on a
// Service Request, where only the radio and S1 bearers are re-established.
func sendInitialContextSetup(ctx context.Context, m *mme.MME, ue *mme.UeContext, qos *mme.EpsQoS, naspdu []byte) {
	// Derive K_eNB and seed the X2-handover key chain (NH for NCC=1). Re-seeded on
	// every context setup, so a Service Request restarts the chain (TS 33.401).
	kenb, kenbCount, err := ue.DeriveInitialKeNB()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to derive AS keys", zap.Error(err))
		return
	}

	uecap := ue.UeNetCap()

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
		UEAggregateMaximumBitRate: s1ap.UEAggregateMaximumBitRate{DL: s1ap.BitRate(qos.AMBRDL.Bps()), UL: s1ap.BitRate(qos.AMBRUL.Bps())},
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
		zap.Stringer("eea", ue.EEA()),
		zap.Stringer("eia", ue.EIA()),
	)

	if err := ue.Conn().SendInitialContextSetup(ctx, ics); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to send Initial Context Setup Request", zap.Error(err))
		return
	}

	ue.Conn().ICS = mme.ICSPending
}

func buildProtectedAttachAccept(ctx context.Context, m *mme.MME, ue *mme.UeContext, qos *mme.EpsQoS) ([]byte, error) {
	p := m.DefaultPDN(ue)
	if p == nil {
		return nil, fmt.Errorf("attach accept with no active PDN")
	}

	operator, err := m.Operator(ctx)
	if err != nil {
		return nil, err
	}

	plmn := operator.PLMN()

	esm, err := buildActivateDefaultESM(p, qos, uint8(ue.RequestedPTI), plmn)
	if err != nil {
		return nil, err
	}

	served, err := operator.ServedTAIs()
	if err != nil {
		return nil, err
	}

	ue.AllocateRegistrationArea(served)

	taiList, err := registrationAreaTAIList(ue.RegistrationArea())
	if err != nil {
		return nil, err
	}

	mmeGroupID, mmeCode := m.MmeIdentity()

	guti, err := m.ReallocateGUTI(ctx, ue, plmn, mmeGroupID, mmeCode)
	if err != nil {
		return nil, err
	}

	t3412, err := nas.GPRSTimer2FromDuration(mme.T3412PeriodicTAU)
	if err != nil {
		return nil, fmt.Errorf("encode T3412: %w", err)
	}

	nfs := m.NetworkFeatureSupport(ue)

	accept := &eps.AttachAccept{
		EPSAttachResult:     eps.AttachResultEPS,
		T3412:               t3412,
		TAIList:             taiList,
		ESMMessageContainer: esm,
		GUTI:                &guti,
		// Advertise IMS voice over PS session (TS 24.301). Without it a
		// voice-centric UE concludes E-UTRAN cannot serve voice and leaves for
		// another RAT (TS 23.221).
		NetworkFeatureSupport: nfs,
	}

	// The MME has no SGs interface, so a combined EPS/IMSI attach succeeds for
	// EPS services only. EMM cause #18 makes the UE stop attempting CS
	// registration on this PLMN (TS 24.301).
	if ue.CombinedAttach {
		cause := eps.EMMCauseCSDomainNotAvailable
		accept.Cause = &cause
	}

	plain, err := accept.MarshalBinary()
	if err != nil {
		return nil, err
	}

	wire, err := ue.ProtectDownlink(plain, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		return nil, err
	}

	return wire, nil
}

func handleAttachComplete(ctx context.Context, m *mme.MME, ue *mme.UeContext) nasreply.Disposition {
	if ue.RegStep() != mme.RegStepContextSetup {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Attach Complete outside the context-setup sub-phase")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ue.Conn().StopNASGuard()

	m.CommitGUTIRealloc(ue)

	ue.TransitionTo(mme.EMMRegistered)

	metrics.RegistrationAttempt(metrics.RAT4G, attachTypeName(ue), metrics.ResultAccept)

	logger.From(ctx, logger.MmeLog).Info("UE attached (EMM-REGISTERED)",
		zap.String("imsi", ue.IMSI()),
	)

	sendNetworkName(ctx, m, ue)

	return nasreply.Handled()
}

// sendNetworkName provides the operator's network name to the UE in an EMM
// INFORMATION message (TS 24.301).
func sendNetworkName(ctx context.Context, m *mme.MME, ue *mme.UeContext) {
	op, err := m.Bearer.GetOperator(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to get operator for network name", zap.String("imsi", ue.IMSI()), zap.Error(err))
		return
	}

	if op.SpnFullName == "" && op.SpnShortName == "" {
		return
	}

	info := &eps.EMMInformation{}
	if op.SpnFullName != "" {
		info.FullNameForNetwork = new(nas.NewNetworkName(op.SpnFullName))
	}

	if op.SpnShortName != "" {
		info.ShortNameForNetwork = new(nas.NewNetworkName(op.SpnShortName))
	}

	ue.Conn().SendDownlinkProtected(ctx, info)
}

func snssaiContainer(snssai models.Snssai, plmn models.PlmnID) (nas.PCOContainer, error) {
	ie, err := snssai.NAS()
	if err != nil {
		return nas.PCOContainer{}, fmt.Errorf("encode S-NSSAI: %w", err)
	}

	value, err := ie.MarshalBinary()
	if err != nil {
		return nas.PCOContainer{}, fmt.Errorf("encode S-NSSAI: %w", err)
	}

	return nas.NewSNSSAIContainer(value, nas.PLMN{MCC: plmn.Mcc, MNC: plmn.Mnc})
}

// buildActivateDefaultESM assembles the ACTIVATE DEFAULT EPS BEARER CONTEXT
// REQUEST for a PDN connection (TS 24.301 §8.3.1).
func buildActivateDefaultESM(p *mme.PdnConnection, qos *mme.EpsQoS, pti uint8, plmn models.PlmnID) ([]byte, error) {
	apn := eps.APN(qos.APN)

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

	apnAMBR, err := eps.APNAMBRFromKbps(qos.SessAmbrDL.Bps()/1000, qos.SessAmbrUL.Bps()/1000)
	if err != nil {
		return nil, fmt.Errorf("failed to encode APN-AMBR: %w", err)
	}

	activate := &eps.ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity: eps.EPSBearerIdentity(p.Ebi),
		PTI:               nas.ProcedureTransactionIdentity(pti),
		EPSQoS:            eps.EPSQoS{QCI: qos.QCI},
		AccessPointName:   apn,
		PDNAddress:        pdnAddr,
		// Signal the per-APN Session-AMBR so the UE can enforce its uplink share
		// (TS 24.301 §8.3.6.7; the P-GW/UPF also enforces both directions).
		APNAMBR: &apnAMBR,
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

	pco := nas.NewProtocolConfigurationOptions(dnsServers, ipv4LinkMTU)

	// The UE stores the S-NSSAI and PLMN against the PDU session identity it
	// allocated and uses them to move the connection to 5GS
	// (TS 24.501 §6.1.4.2, TS 23.501 §5.15.7.1).
	snssai, err := snssaiContainer(qos.Snssai, plmn)
	if err != nil {
		return nil, err
	}

	pco.Containers = append(pco.Containers, snssai)
	activate.ProtocolConfigurationOptions = &pco

	// On an IPv4v6→single-stack downgrade, tell the UE which family was allowed
	// (#50/#51) so it does not retry the other on this APN (TS 24.301).
	if p.EsmCause != 0 {
		cause := p.EsmCause
		activate.Cause = &cause
	}

	return activate.MarshalBinary()
}
