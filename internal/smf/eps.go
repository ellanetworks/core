// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// The SMF (acting as the combined PGW-C, TS 23.501 §4.3) keys each 4G PDN
// connection by its default bearer's EPS bearer identity (5..15) as the PDU
// session id, so one IMSI can hold several. A subscriber is never attached to 4G
// and 5G at once, so the EBI cannot collide with a live 5G PDU session id.

// CreateEPSSession negotiates the PDN type, allocates the UE address(es), and
// programs the user plane for a 4G default EPS bearer, with the SMF as the
// converged session anchor (SMF+PGW-C / combined S-GW-U+P-GW-U, TS 23.401
// §4.4.3, TS 23.501 §4.3). For IPv6/IPv4v6 it allocates a /64 prefix and a SLAAC
// interface identifier (the prefix reaches the UE later via Router Advertisement
// once ModifyEPSSession registers the IPv6 session). It returns the negotiated
// type, the allocated addresses, and the S-GW S1-U F-TEID the eNB sends uplink
// traffic to; the eNB downlink endpoint is supplied later via ModifyEPSSession.
func (s *SMF) CreateEPSSession(ctx context.Context, req models.EPSBearerRequest) (bearer models.EPSBearer, err error) {
	ctx, span := tracer.Start(ctx, "smf/create_eps_session",
		trace.WithAttributes(
			attribute.String("ue.imsi", req.IMSI),
			attribute.Int("eps.bearer_id", int(req.EPSBearerIdentity)),
			attribute.String("eps.apn", req.APN),
		),
	)
	defer span.End()

	defer func() { recordSessionEstablishment(metrics.RAT4G, err) }()

	supi, err := etsi.NewSUPIFromIMSI(req.IMSI)
	if err != nil {
		return models.EPSBearer{}, fmt.Errorf("invalid imsi %q: %w", req.IMSI, err)
	}

	policy := &Policy{
		PolicyID: req.PolicyID,
		Ambr:     models.Ambr{Uplink: req.AMBRUplink, Downlink: req.AMBRDownlink},
		IPv4Pool: req.IPv4Pool,
		IPv6Pool: req.IPv6Pool,
		DNS:      net.ParseIP(req.DNS),
		MTU:      req.MTU,
	}

	pdnType, err := s.negotiatePDUSessionType(ctx, req.RequestedPDNType, policy)
	if err != nil {
		return models.EPSBearer{}, fmt.Errorf("negotiate PDN type: %w", err)
	}

	smContext := s.NewSession(supi, req.EPSBearerIdentity, req.APN, nil)
	smContext.IsEPS = true
	smContext.PDUSessionType = pdnType
	smContext.SetPolicyData(policy)

	var dns netip.Addr
	if policy.DNS != nil {
		dns, _ = netip.AddrFromSlice(policy.DNS)
	}

	bearer = models.EPSBearer{PDNType: pdnType, DNS: dns.Unmap()}

	// When the UE asked for IPv4v6 but the data network offers a single family,
	// the network signals the limitation with ESM cause #50/#51 in the Activate
	// Default EPS Bearer Context Request (TS 24.301 §6.5.1.3).
	if req.RequestedPDNType == nasMessage.PDUSessionTypeIPv4IPv6 && pdnType != nasMessage.PDUSessionTypeIPv4IPv6 {
		if pdnType == nasMessage.PDUSessionTypeIPv4 {
			bearer.ESMCause = eps.ESMCausePDNTypeIPv4OnlyAllowed
		} else {
			bearer.ESMCause = eps.ESMCausePDNTypeIPv6OnlyAllowed
		}
	}

	// dlPdrIP keys the downlink PDR: the IPv4 address for IPv4/IPv4v6, the /64
	// prefix base for IPv6-only (dual-stack gets a second PDR automatically).
	var dlPdrIP netip.Addr

	if pdnType == nasMessage.PDUSessionTypeIPv4 || pdnType == nasMessage.PDUSessionTypeIPv4IPv6 {
		ipv4, allocErr := s.store.AllocateIP(ctx, req.IMSI, req.APN, req.EPSBearerIdentity)
		if allocErr != nil {
			s.abortEPSSession(ctx, supi, req.APN, req.EPSBearerIdentity)

			return models.EPSBearer{}, fmt.Errorf("allocate UE IPv4: %w", allocErr)
		}

		smContext.PDUIPV4Address = netipToIP(ipv4)
		bearer.IPv4 = ipv4
		dlPdrIP = ipv4
	}

	if pdnType == nasMessage.PDUSessionTypeIPv6 || pdnType == nasMessage.PDUSessionTypeIPv4IPv6 {
		ipv6Prefix, allocErr := s.store.AllocateIPv6(ctx, req.IMSI, req.APN, req.EPSBearerIdentity)
		if allocErr != nil {
			s.abortEPSSession(ctx, supi, req.APN, req.EPSBearerIdentity)

			return models.EPSBearer{}, fmt.Errorf("allocate UE IPv6 prefix: %w", allocErr)
		}

		smContext.PDUIPV6Prefix = netipToIP(ipv6Prefix)

		iid, iidErr := s.assignIID(req.APN)
		if iidErr != nil {
			s.abortEPSSession(ctx, supi, req.APN, req.EPSBearerIdentity)

			return models.EPSBearer{}, fmt.Errorf("assign IPv6 IID: %w", iidErr)
		}

		smContext.IPv6IID = iid
		bearer.IPv6Prefix = ipv6Prefix
		bearer.IPv6IID = iid

		if pdnType == nasMessage.PDUSessionTypeIPv6 {
			dlPdrIP = ipv6Prefix
		}
	}

	smContext.Tunnel = &UPTunnel{DataPath: &DataPath{UpLinkTunnel: &GTPTunnel{}, DownLinkTunnel: &GTPTunnel{}}}

	if err := smContext.Tunnel.DataPath.ActivateTunnelAndPDR(s, smContext, policy, dlPdrIP); err != nil {
		s.abortEPSSession(ctx, supi, req.APN, req.EPSBearerIdentity)

		return models.EPSBearer{}, fmt.Errorf("activate data path: %w", err)
	}

	if err := s.sendPFCPRules(ctx, smContext); err != nil {
		s.abortEPSSession(ctx, supi, req.APN, req.EPSBearerIdentity)

		return models.EPSBearer{}, fmt.Errorf("establish UPF session: %w", err)
	}

	ul := smContext.Tunnel.DataPath.UpLinkTunnel
	bearer.SGW = models.FTEID{TEID: ul.TEID, Addr: ul.N3IPv4}
	bearer.SGWN3IPv6 = ul.N3IPv6

	return bearer, nil
}

// ModifyEPSSession sets the downlink endpoint of an established EPS session to
// the eNB S1-U F-TEID learned from the Initial Context Setup Response, so the
// UPF encapsulates downlink traffic toward the eNB (plain, PSC-less GTP-U on
// S1-U). It mirrors the 5G handling of the gNB F-TEID from the N2 setup response.
func (s *SMF) ModifyEPSSession(ctx context.Context, imsi string, ebi uint8, enb models.FTEID) error {
	ctx, span := tracer.Start(ctx, "smf/modify_eps_session",
		trace.WithAttributes(
			attribute.String("ue.imsi", imsi),
			attribute.Int("eps.bearer_id", int(ebi)),
		),
	)
	defer span.End()

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.GetSession(CanonicalName(supi, ebi))
	if smContext == nil {
		return fmt.Errorf("no EPS session for %s", imsi)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil || !smContext.Tunnel.DataPath.Activated {
		return fmt.Errorf("EPS session for %s is not activated", imsi)
	}

	enbIP := net.IP(enb.Addr.AsSlice())
	smContext.Tunnel.ANInformation.TEID = enb.TEID

	dl := smContext.Tunnel.DataPath.DownLinkTunnel.PDR
	ul := smContext.Tunnel.DataPath.UpLinkTunnel.PDR
	dl.FAR.ApplyAction = models.ApplyAction{Forw: true}

	// The S1-U transport family follows the eNB endpoint (TS 29.281): the downlink
	// outer header creation and the uplink outer header removal must both match it.
	// The uplink removal was set to IPv4 at session creation (before the eNB
	// endpoint was known), so correct it here.
	if enbIP.To4() == nil {
		smContext.Tunnel.ANInformation.IPv6Address = enbIP
		dl.FAR.ForwardingParameters = &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv6,
				TEID:        enb.TEID,
				IPv6Address: enbIP,
				S1U:         true,
			},
		}
		ohr := models.OuterHeaderRemovalGtpUUdpIpv6
		ul.OuterHeaderRemoval = &ohr
	} else {
		smContext.Tunnel.ANInformation.IPv4Address = enbIP
		dl.FAR.ForwardingParameters = &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv4,
				TEID:        enb.TEID,
				IPv4Address: enbIP,
				S1U:         true,
			},
		}
		ohr := models.OuterHeaderRemovalGtpUUdpIpv4
		ul.OuterHeaderRemoval = &ohr
	}

	dl.State = RuleUpdate
	dl.FAR.State = RuleUpdate
	ul.State = RuleUpdate

	var policyID string
	if smContext.PolicyData != nil {
		policyID = smContext.PolicyData.PolicyID
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.RemoteSEID,
		policyID,
		[]*PDR{dl, ul},
		[]*FAR{dl.FAR},
		nil,
	)); err != nil {
		return err
	}

	// With the eNB endpoint now known, register the IPv6 session so the UPF's RA
	// responder answers the UE's Router Solicitation with the /64 prefix. No-op
	// for an IPv4-only bearer (PDUIPV6Prefix is nil).
	s.registerIPv6SessionIfNeeded(ctx, smContext)

	return nil
}

// ReleaseEPSSession tears down the 4G default bearer: it frees the UPF session
// (PDRs/FARs/QER + TEID) and the UE IP lease.
func (s *SMF) ReleaseEPSSession(ctx context.Context, imsi string, ebi uint8) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	return s.ReleaseSmContext(ctx, CanonicalName(supi, ebi))
}

// DeactivateEPSSession puts the 4G default bearer into buffering mode when the UE
// goes ECM-IDLE: the downlink FAR is switched from forward to buffer so downlink
// data raises a notification (the paging trigger) instead of being sent to the
// released eNB tunnel. The session is retained; ModifyEPSSession restores
// forwarding on the next Service Request.
func (s *SMF) DeactivateEPSSession(ctx context.Context, imsi string, ebi uint8) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	return s.DeactivateSmContext(ctx, CanonicalName(supi, ebi))
}

// abortEPSSession rolls back a partially-created session: it tears down the UPF
// session if one was established, frees whichever address leases were taken, and
// removes the context from the pool.
func (s *SMF) abortEPSSession(ctx context.Context, supi etsi.SUPI, apn string, ebi uint8) {
	ref := CanonicalName(supi, ebi)

	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	if sc.Tunnel != nil && sc.PFCPContext != nil && sc.PFCPContext.RemoteSEID != 0 {
		if err := s.releaseTunnel(ctx, sc); err != nil {
			logger.SmfLog.Warn("failed to release tunnel for aborted EPS session", zap.String("imsi", supi.IMSI()), zap.Error(err))
		}
	}

	if sc.PDUIPV4Address != nil {
		if _, err := s.store.ReleaseIP(ctx, supi.IMSI(), apn, ebi); err != nil {
			logger.SmfLog.Warn("failed to release UE IPv4 after aborted EPS session", zap.String("imsi", supi.IMSI()), zap.Error(err))
		}
	}

	if sc.PDUIPV6Prefix != nil {
		if _, err := s.store.ReleaseIPv6(ctx, supi.IMSI(), apn, ebi); err != nil {
			logger.SmfLog.Warn("failed to release UE IPv6 after aborted EPS session", zap.String("imsi", supi.IMSI()), zap.Error(err))
		}
	}

	s.removeSessionUnlocked(ctx, ref)
}
