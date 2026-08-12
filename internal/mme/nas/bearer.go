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

func activateDefaultBearer(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn) {
	if requestESMInformation(ctx, ue, ueConn, func(pti uint8) {
		// T3489's final expiry outlives the request's context.
		rejectAttachESM(context.Background(), m, ue, ueConn, pti, eps.ESMCauseESMInformationNotReceived)
	}) {
		return
	}

	qos, err := mme.ResolveAttachQoS(ctx, m, ue)
	if errors.Is(err, mme.ErrUnknownAPN) {
		// The requested APN is not bound to any policy in the subscriber's profile
		// (TS 24.301 §6.5.1.4, ESM cause #27).
		logger.From(ctx, logger.MmeLog).Info("attach rejected: requested APN not in subscriber profile",
			zap.String("imsi", ue.IMSI()), zap.String("apn", ue.RequestedAPN))
		rejectAttachESM(ctx, m, ue, ueConn, uint8(ue.RequestedPTI), eps.ESMCauseMissingOrUnknownAPN)

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
		rejectAttach(ctx, m, ue, ueConn, eps.EMMCauseEPSServicesNotAllowed)

		return
	}

	if cause, refused := requestTypeRefusal(ue.RequestedType); refused {
		logger.From(ctx, logger.MmeLog).Info("attach rejected: request type not served",
			zap.String("imsi", ue.IMSI()), zap.Stringer("request-type", ue.RequestedType))
		rejectAttachESM(ctx, m, ue, ueConn, uint8(ue.RequestedPTI), cause)

		return
	}

	bearer, err := m.Session.CreateEPSSession(ctx, models.EPSBearerRequest{
		IMSI:              ue.IMSI(),
		EPSBearerIdentity: mme.DefaultERABID,
		PDUSessionID:      ue.RequestedPDUSessionID,
		Snssai:            qos.Snssai,
		PolicyID:          qos.PolicyID,
		APN:               qos.APN,
		AMBRUplink:        qos.SessAmbrUL,
		AMBRDownlink:      qos.SessAmbrDL,
		IPv4Pool:          qos.IPv4Pool,
		IPv6Pool:          qos.IPv6Pool,
		DNS:               qos.DNS,
		MTU:               qos.MTU,
		RequestedPDNType:  ue.RequestedPDNType,
		RequestType:       ue.RequestedType,
	})
	if err != nil {
		logger.From(ctx, logger.MmeLog).Info("attach rejected: default bearer setup failed",
			zap.String("imsi", ue.IMSI()), zap.Error(err))
		rejectAttachESM(ctx, m, ue, ueConn, uint8(ue.RequestedPTI), attachBearerRejectCause(ue.RequestedType, err))

		return
	}

	pdnType, dns, esmCause := m.InstallDefaultBearer(ue, qos, bearer)

	logger.From(ctx, logger.MmeLog).Info("EPS default bearer established",
		zap.String("imsi", ue.IMSI()),
		zap.Uint8("pdn-type", pdnType),
		zap.String("dns", dns),
		zap.Stringer("esm-cause", esmCause),
	)

	plain, err := buildAttachAccept(ctx, m, ue, qos)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Attach Accept", zap.Error(err))

		rejectAttachESM(ctx, m, ue, ueConn, uint8(ue.RequestedPTI), eps.ESMCauseRequestRejectedUnspecified)

		if p := m.DefaultPDN(ue); p != nil {
			m.ReleasePDN(ctx, ue, p)
		}

		return
	}

	// Supersede any prior context for this subscriber only once the attach is
	// authenticated and accepted, so an unauthenticated attach cannot tear down a
	// registered UE (TS 24.501 §4.4.4.3 analogue).
	m.CommitUEIdentity(ctx, ue, mme.MintAuthProofForAttachCommit())

	// Drop any stored UE Radio Capability and omit it from the Initial Context
	// Setup, so the eNB re-fetches it from the UE (TS 23.401).
	ue.RadioCapability = nil

	ics, carrier, ok := buildInitialContextSetup(ctx, m, ue, ueConn, qos)
	if !ok {
		if p := m.DefaultPDN(ue); p != nil {
			m.ReleasePDN(ctx, ue, p)
		}

		return
	}

	if err := ueConn.SendProtected(plain, eps.SHTIntegrityProtectedCiphered, func(wire []byte) error {
		return sendInitialContextSetup(ctx, ueConn, ics, carrier, wire)
	}); err != nil {
		mme.ReportProtectFailure(ctx, ueConn, "Attach Accept", err)

		if p := m.DefaultPDN(ue); p != nil {
			m.ReleasePDN(ctx, ue, p)
		}

		return
	}

	ue.AdvanceRegStep(mme.RegStepContextSetup)

	// Keep the sent Attach Accept so a duplicate Attach Request with identical IEs
	// can be answered by resending it (TS 24.301 §5.5.1.2.7 case d).
	ueConn.AttachAcceptPlain = plain

	// T3450 retransmits the Attach Accept, then releases the UE, if no Attach
	// Complete arrives (TS 24.301).
	ueConn.ArmNASGuard("Attach Accept", plain, eps.SHTIntegrityProtectedCiphered)
}

func buildInitialContextSetup(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, qos *mme.EpsQoS) (*s1ap.InitialContextSetupRequest, uint8, bool) {
	kenb, kenbCount, err := ue.DeriveInitialKeNB()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to derive AS keys", zap.Error(err))
		return nil, 0, false
	}

	uecap := ue.UeNetCap()
	pdns := m.SnapshotPDNs(ue)
	erabs := make([]s1ap.ERABToBeSetupItemCtxtSUReq, 0, len(pdns))

	carrier := uint8(0)

	for _, p := range pdns {
		if carrier == 0 || p.Ebi < carrier {
			carrier = p.Ebi
		}
	}

	for _, p := range pdns {
		// The S1-U endpoint advertises whichever transport address family the N3 has
		// (TS 36.413).
		sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
		if err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to encode S-GW transport layer address",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", p.Ebi), zap.Error(err))

			continue
		}

		erabs = append(erabs, s1ap.ERABToBeSetupItemCtxtSUReq{
			ERABID: s1ap.ERABID(p.Ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(p.Qci),
				ARP: mme.BearerARP(p.Arp),
			},
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.SgwFTEID.TEID),
		})
	}

	if len(erabs) == 0 {
		logger.From(ctx, logger.MmeLog).Error("Initial Context Setup with no encodable E-RAB", zap.String("imsi", ue.IMSI()))
		return nil, 0, false
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
		zap.Uint32("enb-ue-id", uint32(ueConn.ENBUES1APID)),
		zap.Uint8("nas-pdu-bearer", carrier),
		zap.Int("bearers", len(erabs)),
		zap.Uint32("kenb-ul-count", kenbCount),
		zap.Stringer("eea", ue.EEA()),
		zap.Stringer("eia", ue.EIA()),
	)

	return ics, carrier, true
}

func sendInitialContextSetup(ctx context.Context, ueConn *mme.UeConn, ics *s1ap.InitialContextSetupRequest, carrier uint8, wire []byte) error {
	if len(wire) > 0 {
		for i := range ics.ERABToBeSetup {
			if uint8(ics.ERABToBeSetup[i].ERABID) == carrier {
				ics.ERABToBeSetup[i].NASPDU = s1ap.NASPDU(wire)
			}
		}
	}

	if err := ueConn.SendInitialContextSetup(ctx, ics); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to send Initial Context Setup Request", zap.Error(err))

		return err
	}

	ueConn.ICS = mme.ICSPending

	return nil
}

func buildAttachAccept(ctx context.Context, m *mme.MME, ue *mme.UeContext, qos *mme.EpsQoS) ([]byte, error) {
	p := m.DefaultPDN(ue)
	if p == nil {
		return nil, fmt.Errorf("attach accept with no active PDN")
	}

	operator, err := m.Operator(ctx)
	if err != nil {
		return nil, err
	}

	plmn := operator.PLMN()

	esm, err := buildActivateDefaultESM(p, qos, uint8(ue.RequestedPTI), plmn, ue.UsesEPCO(p))
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

	nfs := m.NetworkFeatureSupport(ue.UeNetCap())

	accept := &eps.AttachAccept{
		EPSAttachResult:       eps.AttachResultEPS,
		T3412:                 t3412,
		TAIList:               taiList,
		ESMMessageContainer:   esm,
		GUTI:                  &guti,
		NetworkFeatureSupport: nfs,
	}

	// The MME has no SGs interface, so a combined EPS/IMSI attach succeeds for
	// EPS services only. EMM cause #18 makes the UE stop attempting CS
	// registration on this PLMN (TS 24.301).
	if ue.CombinedAttach {
		cause := eps.EMMCauseCSDomainNotAvailable
		accept.Cause = &cause
	}

	return accept.MarshalBinary()
}

func handleAttachComplete(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, msg *eps.AttachComplete) nasreply.Disposition {
	if ue.RegStep() != mme.RegStepContextSetup {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Attach Complete outside the context-setup sub-phase")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ueConn.StopNASGuard()

	m.CommitGUTIRealloc(ue)

	ue.TransitionTo(mme.EMMRegistered)

	metrics.RegistrationAttempt(metrics.RAT4G, attachTypeName(ue), metrics.ResultAccept)

	logger.From(ctx, logger.MmeLog).Info("UE attached (EMM-REGISTERED)",
		zap.String("imsi", ue.IMSI()),
	)

	acceptDefaultBearerFromAttach(ctx, m, ue, msg.ESMMessageContainer)

	sendNetworkName(ctx, m, ue, ueConn)

	return nasreply.Handled()
}

func acceptDefaultBearerFromAttach(ctx context.Context, m *mme.MME, ue *mme.UeContext, container []byte) {
	accept, err := eps.ParseActivateDefaultEPSBearerContextAccept(container)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("ignoring the ESM message container of the Attach Complete",
			zap.String("imsi", ue.IMSI()), zap.Error(err))

		return
	}

	handleActivateDefaultBearerAccept(ctx, m, ue, accept)
}

// sendNetworkName provides the operator's network name to the UE in an EMM
// INFORMATION message (TS 24.301).
func sendNetworkName(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn) {
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

	ueConn.SendDownlinkProtected(ctx, info)
}

func buildActivateDefaultESM(p *mme.PdnConnection, qos *mme.EpsQoS, pti uint8, plmn models.PlmnID, useEPCO bool) ([]byte, error) {
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
	dnsServers := nas.DNSServers(p.Dns)

	var ipv4LinkMTU uint16
	if p.PdnType == eps.PDNTypeIPv4 || p.PdnType == eps.PDNTypeIPv4v6 {
		ipv4LinkMTU = qos.MTU
	}

	pco := nas.NewProtocolConfigurationOptions(dnsServers, ipv4LinkMTU)

	if snssai := p.Snssai; snssai != nil && p.PDUSessionID != 0 {
		container, err := snssaiPCOContainer(*snssai, plmn)
		if err != nil {
			return nil, err
		}

		pco.Containers = append(pco.Containers, container)

		mapped, err := mme.MappedFiveGSQoSContainers(p.Ebi, qos)
		if err != nil {
			return nil, err
		}

		pco.Containers = append(pco.Containers, mapped...)
	}

	if useEPCO {
		activate.ExtendedProtocolConfigurationOptions = &pco
	} else {
		activate.ProtocolConfigurationOptions = &pco
	}

	// On an IPv4v6→single-stack downgrade, tell the UE which family was allowed
	// (#50/#51) so it does not retry the other on this APN (TS 24.301).
	if p.EsmCause != 0 {
		cause := p.EsmCause
		activate.Cause = &cause
	}

	return activate.MarshalBinary()
}
