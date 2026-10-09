// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
)

type SubscribedQoS struct {
	Var5qi      int32
	Arp         int32
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
	Subscribed   SubscribedQoS
}

type PolicyDecision struct {
	Revision    uint64
	PolicyID    string
	Var5qi      int32
	Arp         int32
	SessionAMBR models.Ambr
	Rules       []PCCRule
}

type PCCRule struct {
	ID      string
	Version uint64
	QCI     uint8
	ARP     models.Arp
	MBR     models.Ambr
	GBR     models.Ambr
	Gate    models.GateStatus
	Filters []models.SDFFilter
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
	DNS      net.IP
	MTU      uint16
	IPv4Pool string
	IPv6Pool string
	PCSCF    []netip.Addr
}

func subscribedQoS(c udm.DNNConfiguration) SubscribedQoS {
	return SubscribedQoS{Var5qi: c.Var5qi, Arp: c.Arp, SessionAMBR: c.SessionAMBR}
}

func (q SubscribedQoS) equal(o SubscribedQoS) bool {
	return q.Var5qi == o.Var5qi && q.Arp == o.Arp &&
		q.SessionAMBR.Uplink.Equal(o.SessionAMBR.Uplink) && q.SessionAMBR.Downlink.Equal(o.SessionAMBR.Downlink)
}

func (s *SMF) sessionSubscription(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) (udm.DNNConfiguration, error) {
	sm, err := s.subscriptions.SessionManagement(ctx, supi.IMSI())
	if err != nil {
		return udm.DNNConfiguration{}, fmt.Errorf("get session management subscription: %w", err)
	}

	if c, ok := sm.ForDNN(*snssai, dnn); ok {
		return c, nil
	}

	if sm.Subscribes(*snssai) {
		return udm.DNNConfiguration{}, fmt.Errorf("%w: %q on sst=%d sd=%q", ErrDNNNotInSlice, dnn, snssai.Sst, snssai.Sd)
	}

	return udm.DNNConfiguration{}, fmt.Errorf("%w: %q on sst=%d sd=%q", ErrNoPolicyMatch, dnn, snssai.Sst, snssai.Sd)
}

func (s *SMF) checkSubscribed(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) error {
	_, err := s.sessionSubscription(ctx, supi, snssai, dnn)

	return err
}

type sessionInputs struct {
	policy     *Policy
	dn         DNNStore
	subscribed SubscribedQoS
}

func (s *SMF) prepareSession(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) (sessionInputs, error) {
	c, err := s.sessionSubscription(ctx, supi, snssai, dnn)
	if err != nil {
		return sessionInputs{}, err
	}

	dn, err := s.store.ResolveDNN(ctx, dnn)
	if err != nil {
		return sessionInputs{}, fmt.Errorf("resolve data network %q: %w", dnn, err)
	}

	config, err := dn.Config(ctx)
	if err != nil {
		return sessionInputs{}, fmt.Errorf("data network %q configuration: %w", dnn, err)
	}

	return sessionInputs{policy: dataNetworkPolicy(config), dn: dn, subscribed: subscribedQoS(c)}, nil
}

func dataNetworkPolicy(c DataNetworkConfig) *Policy {
	return &Policy{DNS: c.DNS, MTU: c.MTU, IPv4Pool: c.IPv4Pool, IPv6Pool: c.IPv6Pool, PCSCF: c.PCSCF}
}

func applyDecision(p *Policy, d *PolicyDecision) {
	p.PolicyID = d.PolicyID
	p.Ambr = d.SessionAMBR
	p.QosData = models.QosData{QFI: models.DefaultQFI, Var5qi: d.Var5qi, Arp: &models.Arp{PriorityLevel: d.Arp}}
}
