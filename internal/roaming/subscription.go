// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync/atomic"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
)

const IMSAPN = "ims"

var (
	ErrAPNNotSubscribed   = errors.New("APN not subscribed by the home network")
	ErrHomeRouted         = errors.New("APN subscribed for home-routed roaming only")
	ErrIMSAPN             = errors.New("the IMS APN is home-routed for roamers")
	ErrBarred             = errors.New("APN barred by operator determined barring")
	ErrNotLocal           = fmt.Errorf("%w: not a local data network of the roaming partner", models.ErrUnknownAPN)
	ErrNoHomeSubscription = errors.New("no home subscription for the visitor on this node")
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
	case barred(home):
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

func barred(home *s6a.SubscriptionData) bool {
	if home.SubscriberStatus == nil || *home.SubscriberStatus != s6a.SubscriberStatusOperatorDeterminedBarring || home.OperatorDeterminedBarring == nil {
		return false
	}

	return *home.OperatorDeterminedBarring&(s6a.ODBAllPacketOrientedServices|s6a.ODBRoamerAccessVPLMNAP) != 0
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
	IsHomeSubscriber(ctx context.Context, imsi string) (bool, error)
	AccessAndMobility(ctx context.Context, imsi string) (*udm.AccessAndMobilitySubscription, error)
	UEAMBR(ctx context.Context, imsi string) (models.Ambr, error)
	SessionManagement(ctx context.Context, imsi string) (*udm.SessionManagementSubscription, error)
	FramedRoutes(ctx context.Context, imsi, dnn string) ([]netip.Prefix, error)
	StaticIP(ctx context.Context, imsi, dnn string, ipv6 bool) (netip.Addr, bool, error)
}

type HomeData struct {
	PLMN         models.PlmnID
	Subscription *s6a.SubscriptionData
}

type HomeSubscriptions interface {
	VisitorHome(imsi string) (HomeData, bool)
}

type homes struct {
	HomeSubscriptions
}

type Subscriptions struct {
	local Local
	homes atomic.Pointer[homes]
}

func NewSubscriptions(local Local, h HomeSubscriptions) *Subscriptions {
	s := &Subscriptions{local: local}
	s.SetHomeSubscriptions(h)

	return s
}

func (s *Subscriptions) SetHomeSubscriptions(h HomeSubscriptions) {
	if h == nil {
		s.homes.Store(nil)
		return
	}

	s.homes.Store(&homes{h})
}

func (s *Subscriptions) visitorHome(imsi string) (HomeData, bool) {
	h := s.homes.Load()
	if h == nil {
		return HomeData{}, false
	}

	d, ok := h.VisitorHome(imsi)
	if !ok || d.Subscription == nil {
		return HomeData{}, false
	}

	return d, true
}

func (s *Subscriptions) home(imsi string) (*s6a.SubscriptionData, bool) {
	d, ok := s.visitorHome(imsi)

	return d.Subscription, ok
}

func (s *Subscriptions) IsHomeSubscriber(ctx context.Context, imsi string) (bool, error) {
	return s.local.IsHomeSubscriber(ctx, imsi)
}

func (s *Subscriptions) AccessAndMobility(ctx context.Context, imsi string) (*udm.AccessAndMobilitySubscription, error) {
	am, err := s.local.AccessAndMobility(ctx, imsi)
	if err != nil || !am.Visitor {
		return am, err
	}

	home, ok := s.home(imsi)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoHomeSubscription, imsi)
	}

	am.UEAMBR = UEAMBR(am.UEAMBR, home)

	return am, nil
}

func (s *Subscriptions) UEAMBR(ctx context.Context, imsi string) (models.Ambr, error) {
	ambr, err := s.local.UEAMBR(ctx, imsi)
	if err != nil {
		return models.Ambr{}, err
	}

	home, _ := s.home(imsi)

	return UEAMBR(ambr, home), nil
}

func (s *Subscriptions) SessionManagement(ctx context.Context, imsi string) (*udm.SessionManagementSubscription, error) {
	sm, err := s.local.SessionManagement(ctx, imsi)
	if err != nil || !sm.Visitor {
		return sm, err
	}

	home, ok := s.visitorHome(imsi)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoHomeSubscription, imsi)
	}

	merged := mergeSessionManagement(sm, home.Subscription)
	merged.HomePLMN = home.PLMN

	return merged, nil
}

func (s *Subscriptions) SelectAPN(ctx context.Context, imsi, requested string) (string, error) {
	sm, err := s.local.SessionManagement(ctx, imsi)
	if err != nil {
		return "", err
	}

	if !sm.Visitor {
		return selectHomeAPN(sm, imsi, requested)
	}

	home, ok := s.home(imsi)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNoHomeSubscription, imsi)
	}

	return SelectAPN(home, sm, requested)
}

func selectHomeAPN(sm *udm.SessionManagementSubscription, imsi, requested string) (string, error) {
	if requested == "" {
		c, ok := sm.DefaultAPN()
		if !ok {
			return "", fmt.Errorf("subscriber %s has no default APN", imsi)
		}

		return c.DNN, nil
	}

	if _, ok := sm.ForAPN(requested); !ok {
		return "", models.ErrUnknownAPN
	}

	return requested, nil
}

func (s *Subscriptions) FramedRoutes(ctx context.Context, imsi, dnn string) ([]netip.Prefix, error) {
	return s.local.FramedRoutes(ctx, imsi, dnn)
}

func (s *Subscriptions) StaticIP(ctx context.Context, imsi, dnn string, ipv6 bool) (netip.Addr, bool, error) {
	return s.local.StaticIP(ctx, imsi, dnn, ipv6)
}
