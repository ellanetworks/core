// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/eps"
)

const IMSAPN = "ims"

var (
	ErrAPNNotSubscribed    = errors.New("APN not subscribed by the home network")
	ErrHomeRouted          = errors.New("APN subscribed for home-routed roaming only")
	ErrIMSAPN              = errors.New("the IMS APN is home-routed for roamers")
	ErrBarred              = errors.New("APN barred by operator determined barring")
	ErrNotLocal            = fmt.Errorf("%w: not a local data network of the roaming partner", models.ErrUnknownAPN)
	ErrNoHPLMNSubscription = errors.New("no home subscription for the visitor on this node")
)

func Breakout(home *s6a.SubscriptionData, apn string) (string, s6a.APNConfiguration, error) {
	if home == nil || home.APNConfigurationProfile == nil {
		return "", s6a.APNConfiguration{}, ErrAPNNotSubscribed
	}

	profile := home.APNConfigurationProfile

	var (
		c  s6a.APNConfiguration
		ok bool
	)

	if apn == "" {
		c, ok = profile.Default()
		apn = strings.ToLower(c.ServiceSelection)
	} else {
		c, ok = profile.Resolve(apn)
	}

	switch {
	case strings.EqualFold(apn, IMSAPN):
		return "", s6a.APNConfiguration{}, ErrIMSAPN
	case !ok:
		return "", s6a.APNConfiguration{}, ErrAPNNotSubscribed
	case home.BarsVisitedPLMNAccessPoints():
		return "", s6a.APNConfiguration{}, ErrBarred
	case !localBreakoutAllowed(c):
		return "", s6a.APNConfiguration{}, ErrHomeRouted
	}

	return apn, c, nil
}

func DefaultAPN(home *s6a.SubscriptionData) string {
	if home == nil || home.APNConfigurationProfile == nil {
		return ""
	}

	c, ok := home.APNConfigurationProfile.Default()
	if !ok {
		return ""
	}

	return strings.ToLower(c.ServiceSelection)
}

func localBreakoutAllowed(c s6a.APNConfiguration) bool {
	switch {
	case !c.VPLMNDynamicAddressAllowed:
		return false
	case len(c.ServedPartyIPAddresses) > 0:
		return false
	case c.PDNGateway == nil:
		return true
	default:
		return c.PDNGWAllocationType != nil && *c.PDNGWAllocationType == s6a.PDNGWAllocationDynamic
	}
}

func SelectAPN(home *s6a.SubscriptionData, local *udm.SessionManagementSubscription, requested string) (string, error) {
	apn, _, err := Breakout(home, requested)
	if err != nil {
		return "", err
	}

	if _, ok := mergeSessionManagement(local, home).ForAPN(apn); !ok {
		return "", fmt.Errorf("%w: %q", ErrNotLocal, apn)
	}

	return apn, nil
}

func UEAMBR(local models.Ambr, home *s6a.SubscriptionData) models.Ambr {
	if home == nil || home.AMBR == nil {
		return local
	}

	return local.Min(ambrOf(*home.AMBR))
}

func mergeSessionManagement(local *udm.SessionManagementSubscription, home *s6a.SubscriptionData) *udm.SessionManagementSubscription {
	out := &udm.SessionManagementSubscription{Visitor: true}
	defaultAPN := DefaultAPN(home)
	defaultChosen := false

	for _, c := range preferDefaults(local.DNNs) {
		if c.IMS {
			continue
		}

		_, hc, err := Breakout(home, c.DNN)
		if err != nil {
			continue
		}

		c.Default = !defaultChosen && strings.EqualFold(c.DNN, defaultAPN)
		defaultChosen = defaultChosen || c.Default

		if hc.AMBR != nil {
			c.SessionAMBR = c.SessionAMBR.Min(ambrOf(*hc.AMBR))
		}

		out.DNNs = append(out.DNNs, c)
	}

	return out
}

func preferDefaults(dnns []udm.DNNConfiguration) []udm.DNNConfiguration {
	out := make([]udm.DNNConfiguration, 0, len(dnns))

	for _, c := range dnns {
		if c.Default {
			out = append(out, c)
		}
	}

	for _, c := range dnns {
		if !c.Default {
			out = append(out, c)
		}
	}

	return out
}

func ambrOf(a s6a.AMBR) models.Ambr {
	return models.Ambr{Uplink: models.BitRateFromBps(uint64(a.UL)), Downlink: models.BitRateFromBps(uint64(a.DL))}
}

type Local interface {
	UEAMBR(ctx context.Context, imsi string) (models.Ambr, error)
	SessionManagement(ctx context.Context, imsi string) (*udm.SessionManagementSubscription, error)
}

type HPLMNData struct {
	PLMN         models.PlmnID
	Subscription *s6a.SubscriptionData
}

type HPLMNDataSource interface {
	VisitorHPLMN(imsi string) (HPLMNData, bool)
}

type N26Policy interface {
	Allowed(ctx context.Context, imsi string) (bool, error)
}

type Subscriptions struct {
	local  Local
	hplmns HPLMNDataSource
}

func NewSubscriptions(local Local, h HPLMNDataSource) *Subscriptions {
	return &Subscriptions{local: local, hplmns: h}
}

func (s *Subscriptions) hplmn(imsi string) (*s6a.SubscriptionData, bool) {
	if s.hplmns == nil {
		return nil, false
	}

	d, ok := s.hplmns.VisitorHPLMN(imsi)
	if !ok || d.Subscription == nil {
		return nil, false
	}

	return d.Subscription, true
}

func (s *Subscriptions) UEAMBR(ctx context.Context, imsi string) (models.Ambr, error) {
	ambr, err := s.local.UEAMBR(ctx, imsi)
	if err != nil {
		return models.Ambr{}, err
	}

	home, _ := s.hplmn(imsi)

	return UEAMBR(ambr, home), nil
}

func (s *Subscriptions) SelectAPN(ctx context.Context, imsi, requested string) (string, error) {
	sm, err := s.local.SessionManagement(ctx, imsi)
	if err != nil {
		return "", err
	}

	if !sm.Visitor {
		return selectOwnAPN(sm, imsi, requested)
	}

	home, ok := s.hplmn(imsi)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNoHPLMNSubscription, imsi)
	}

	return SelectAPN(home, sm, requested)
}

func selectOwnAPN(sm *udm.SessionManagementSubscription, imsi, requested string) (string, error) {
	if requested == "" {
		c, ok := sm.DefaultAPN()
		if !ok {
			return "", fmt.Errorf("subscriber %s has no default APN", imsi)
		}

		return c.DNN, nil
	}

	c, ok := sm.ForAPN(requested)
	if !ok {
		return "", models.ErrUnknownAPN
	}

	return c.DNN, nil
}

func APNSubscription(hplmn models.PlmnID, home *s6a.SubscriptionData, apn string) (*models.APNSubscription, error) {
	_, c, err := Breakout(home, apn)
	if err != nil {
		return nil, err
	}

	sub := &models.APNSubscription{
		HPLMN:                   hplmn,
		PDNTypes:                pdnTypes(c.PDNType),
		ChargingCharacteristics: home.ChargingCharacteristics,
		MSISDN:                  home.MSISDN,
	}

	if c.ChargingCharacteristics != "" {
		sub.ChargingCharacteristics = c.ChargingCharacteristics
	}

	if c.AMBR != nil {
		sub.APNAMBR = new(ambrOf(*c.AMBR))
	}

	if c.QoS != nil {
		sub.QCI = uint8(min(c.QoS.QCI, 255))
		sub.ARP = arpOf(c.QoS.ARP)
	}

	return sub, nil
}

func pdnTypes(t s6a.PDNType) []eps.PDNType {
	switch t {
	case s6a.PDNTypeIPv4:
		return []eps.PDNType{eps.PDNTypeIPv4}
	case s6a.PDNTypeIPv6:
		return []eps.PDNType{eps.PDNTypeIPv6}
	case s6a.PDNTypeIPv4OrIPv6:
		return []eps.PDNType{eps.PDNTypeIPv4, eps.PDNTypeIPv6}
	default:
		return []eps.PDNType{eps.PDNTypeIPv4v6}
	}
}

func arpOf(a tgpp.AllocationRetentionPriority) models.Arp {
	arp := models.Arp{
		PriorityLevel: int32(min(a.PriorityLevel, 15)),
		PreemptCap:    models.PreemptionCapabilityNotPreempt,
		PreemptVuln:   models.PreemptionVulnerabilityNotPreemptable,
	}

	if a.PreemptionCapability == tgpp.PreemptionCapabilityEnabled {
		arp.PreemptCap = models.PreemptionCapabilityMayPreempt
	}

	if a.PreemptionVulnerability == tgpp.PreemptionVulnerabilityEnabled {
		arp.PreemptVuln = models.PreemptionVulnerabilityPreemptable
	}

	return arp
}
