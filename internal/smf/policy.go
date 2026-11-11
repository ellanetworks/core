// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
)

type SubscribedQoS struct {
	Var5qi      int32
	Arp         models.Arp
	SessionAMBR models.Ambr
}

type PolicyContext struct {
	Supi         etsi.SUPI
	PDUSessionID uint8
	Dnn          string
	Snssai       models.Snssai
	Access       AccessType
	IPv4         netip.Addr
	IPv6Prefix   netip.Prefix
	IMS          bool
	Subscribed   SubscribedQoS
}

type PolicyDecision struct {
	Revision    uint64
	PolicyID    string
	Var5qi      int32
	Arp         models.Arp
	SessionAMBR models.Ambr
	RatingGroup uint32
	Rules       []PCCRule
}

type PCCRule struct {
	ID          string
	Version     uint64
	QCI         uint8
	ARP         models.Arp
	MBR         models.Ambr
	GBR         models.Ambr
	Gate        models.GateStatus
	RatingGroup uint32
	Filters     []models.SDFFilter
}

type PCF interface {
	CreateAssociation(ctx context.Context, ref string, c PolicyContext) (*PolicyDecision, error)
	UpdateAssociation(ctx context.Context, ref string, subscribed SubscribedQoS) (*PolicyDecision, error)
	ReportEnforcementFailure(ref string, reports []RuleReport, cause EnforcementFailure)
	TerminateAssociation(ref string)
}

type RuleReport struct {
	Rule   PCCRule
	Active *PCCRule
}

type EnforcementFailure uint8

const (
	ResourcesNotAllocated EnforcementFailure = iota
	BearerReleased
)

type DataNetworkConfig struct {
	SliceDataNetwork string
	NetworkInstance  string
	DNS              net.IP
	MTU              uint16
	IPv4Pool         string
	IPv6Pool         string
	IMS              bool
	NAT              bool
	LocalSwitch      bool
	PCSCF            []netip.Addr
}

func subscribedQoS(c udm.DNNConfiguration) SubscribedQoS {
	return SubscribedQoS{Var5qi: c.Var5qi, Arp: c.Arp, SessionAMBR: c.SessionAMBR}
}

func (q SubscribedQoS) equal(o SubscribedQoS) bool {
	return q.Var5qi == o.Var5qi && q.Arp == o.Arp &&
		q.SessionAMBR.Uplink.Equal(o.SessionAMBR.Uplink) && q.SessionAMBR.Downlink.Equal(o.SessionAMBR.Downlink)
}

func (s *SMF) sessionSubscription(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) (udm.DNNConfiguration, bool, error) {
	sm, err := s.subscriptions.SessionManagement(ctx, supi.IMSI())
	if err != nil {
		return udm.DNNConfiguration{}, false, fmt.Errorf("get session management subscription: %w", err)
	}

	if c, ok := sm.ForDNN(*snssai, dnn); ok {
		if sm.Visitor && c.IMS {
			return udm.DNNConfiguration{}, true, fmt.Errorf("%w: %q is home-routed for roamers", ErrDNNNotFound, dnn)
		}

		return c, sm.Visitor, nil
	}

	if sm.Subscribes(*snssai) {
		return udm.DNNConfiguration{}, sm.Visitor, fmt.Errorf("%w: %q on sst=%d sd=%q", ErrDNNNotInSlice, dnn, snssai.Sst, snssai.Sd)
	}

	return udm.DNNConfiguration{}, sm.Visitor, fmt.Errorf("%w: %q on sst=%d sd=%q", ErrNoPolicyMatch, dnn, snssai.Sst, snssai.Sd)
}

func (s *SMF) checkSubscribed(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) (bool, error) {
	_, visitor, err := s.sessionSubscription(ctx, supi, snssai, dnn)

	return visitor, err
}

type sessionInputs struct {
	policy     *Policy
	dn         DNNStore
	subscribed SubscribedQoS
	visitor    bool
}

func (s *SMF) prepareSession(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) (sessionInputs, error) {
	c, visitor, err := s.sessionSubscription(ctx, supi, snssai, dnn)
	if err != nil {
		return sessionInputs{}, err
	}

	dn, err := s.store.ResolveDNN(ctx, dnn)
	if err != nil {
		return sessionInputs{}, fmt.Errorf("resolve data network %q: %w", dnn, err)
	}

	config, err := dn.Config(ctx, *snssai)
	if err != nil {
		return sessionInputs{}, fmt.Errorf("data network %q configuration: %w", dnn, err)
	}

	return sessionInputs{policy: dataNetworkPolicy(config), dn: dn, subscribed: subscribedQoS(c), visitor: visitor}, nil
}

func dataNetworkPolicy(c DataNetworkConfig) *Policy {
	return &Policy{SliceDataNetwork: c.SliceDataNetwork, NetworkInstance: c.NetworkInstance, DNS: c.DNS, MTU: c.MTU, IPv4Pool: c.IPv4Pool, IPv6Pool: c.IPv6Pool, IMS: c.IMS, NAT: c.NAT, LocalSwitch: c.LocalSwitch, PCSCF: c.PCSCF}
}

func applyDecision(p *Policy, d *PolicyDecision) {
	p.PolicyID = d.PolicyID
	p.Ambr = d.SessionAMBR
	p.RatingGroup = d.RatingGroup
	arp := d.Arp
	p.QosData = models.QosData{QFI: models.DefaultQFI, Var5qi: d.Var5qi, Arp: &arp}
}

func (d *PolicyDecision) sameSessionPolicy(o *PolicyDecision) bool {
	return d.PolicyID == o.PolicyID && d.Var5qi == o.Var5qi && d.Arp == o.Arp &&
		d.SessionAMBR.Equal(o.SessionAMBR) && d.RatingGroup == o.RatingGroup
}

func (r PCCRule) Equal(o PCCRule) bool {
	return r.ID == o.ID && r.Version == o.Version && r.QCI == o.QCI && r.ARP == o.ARP &&
		r.MBR.Equal(o.MBR) && r.GBR.Equal(o.GBR) && r.Gate == o.Gate && r.RatingGroup == o.RatingGroup &&
		slices.Equal(r.Filters, o.Filters)
}
