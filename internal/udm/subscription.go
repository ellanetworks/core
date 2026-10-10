// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
)

var ErrDNNNotSubscribed = errors.New("DNN not subscribed")

type SubscriptionStore interface {
	GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error)
	GetProfileByID(ctx context.Context, id string) (*db.Profile, error)
	ListPoliciesByProfile(ctx context.Context, profileID string) ([]db.Policy, error)
	ListNetworkSlicesByIDs(ctx context.Context, ids []string) ([]db.NetworkSlice, error)
	GetDataNetworkByID(ctx context.Context, id string) (*db.DataNetwork, error)
}

type SessionSubscriptionStore interface {
	FramedRoutes(ctx context.Context, imsi, dnn string) ([]netip.Prefix, error)
	StaticIP(ctx context.Context, imsi, dnn string, ipv6 bool) (netip.Addr, bool, error)
}

type AccessAndMobilitySubscription struct {
	SubscribedSNSSAIs []models.Snssai
	UEAMBR            models.Ambr
	Allow4G           bool
	Allow5G           bool
}

type DNNConfiguration struct {
	Snssai      models.Snssai
	DNN         string
	Default     bool
	Var5qi      int32
	Arp         int32
	SessionAMBR models.Ambr
}

type SessionManagementSubscription struct {
	DNNs []DNNConfiguration
}

type Subscriptions struct {
	store    SubscriptionStore
	sessions SessionSubscriptionStore
}

func NewSubscriptions(store SubscriptionStore, sessions SessionSubscriptionStore) *Subscriptions {
	return &Subscriptions{store: store, sessions: sessions}
}

func (s *Subscriptions) AccessAndMobility(ctx context.Context, imsi string) (*AccessAndMobilitySubscription, error) {
	profile, err := s.profile(ctx, imsi)
	if err != nil {
		return nil, err
	}

	ambr, err := ueAMBR(profile)
	if err != nil {
		return nil, err
	}

	policies, err := s.store.ListPoliciesByProfile(ctx, profile.ID)
	if err != nil {
		return nil, fmt.Errorf("list policies for profile %s: %w", profile.ID, err)
	}

	sliceIDs := make([]string, 0, len(policies))
	for _, p := range policies {
		if !slices.Contains(sliceIDs, p.SliceID) {
			sliceIDs = append(sliceIDs, p.SliceID)
		}
	}

	slices.Sort(sliceIDs)

	subscribed, err := s.store.ListNetworkSlicesByIDs(ctx, sliceIDs)
	if err != nil {
		return nil, fmt.Errorf("list network slices: %w", err)
	}

	snssais := make([]models.Snssai, 0, len(subscribed))
	for i := range subscribed {
		snssais = append(snssais, snssaiOf(&subscribed[i]))
	}

	return &AccessAndMobilitySubscription{
		SubscribedSNSSAIs: snssais,
		UEAMBR:            ambr,
		Allow4G:           profile.Allow4G,
		Allow5G:           profile.Allow5G,
	}, nil
}

func (s *Subscriptions) UEAMBR(ctx context.Context, imsi string) (models.Ambr, error) {
	profile, err := s.profile(ctx, imsi)
	if err != nil {
		return models.Ambr{}, err
	}

	return ueAMBR(profile)
}

func (s *Subscriptions) SessionManagement(ctx context.Context, imsi string) (*SessionManagementSubscription, error) {
	sub, err := s.subscriber(ctx, imsi)
	if err != nil {
		return nil, err
	}

	policies, err := s.store.ListPoliciesByProfile(ctx, sub.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("list policies for profile %s: %w", sub.ProfileID, err)
	}

	sliceIDs := make([]string, 0, len(policies))
	for _, p := range policies {
		if !slices.Contains(sliceIDs, p.SliceID) {
			sliceIDs = append(sliceIDs, p.SliceID)
		}
	}

	subscribed, err := s.store.ListNetworkSlicesByIDs(ctx, sliceIDs)
	if err != nil {
		return nil, fmt.Errorf("list network slices: %w", err)
	}

	byID := make(map[string]models.Snssai, len(subscribed))
	for i := range subscribed {
		byID[subscribed[i].ID] = snssaiOf(&subscribed[i])
	}

	sm := &SessionManagementSubscription{DNNs: make([]DNNConfiguration, 0, len(policies))}

	for _, p := range policies {
		snssai, ok := byID[p.SliceID]
		if !ok {
			continue
		}

		dn, err := s.store.GetDataNetworkByID(ctx, p.DataNetworkID)
		if err != nil {
			return nil, fmt.Errorf("get data network %s: %w", p.DataNetworkID, err)
		}

		ambr, err := sessionAMBR(&p)
		if err != nil {
			continue
		}

		sm.DNNs = append(sm.DNNs, DNNConfiguration{
			Snssai:      snssai,
			DNN:         dn.Name,
			Default:     p.IsDefault,
			Var5qi:      p.Var5qi,
			Arp:         p.Arp,
			SessionAMBR: ambr,
		})
	}

	return sm, nil
}

func (s *Subscriptions) FramedRoutes(ctx context.Context, imsi, dnn string) ([]netip.Prefix, error) {
	return s.sessions.FramedRoutes(ctx, imsi, dnn)
}

func (s *Subscriptions) StaticIP(ctx context.Context, imsi, dnn string, ipv6 bool) (netip.Addr, bool, error) {
	return s.sessions.StaticIP(ctx, imsi, dnn, ipv6)
}

func (sm *SessionManagementSubscription) DefaultDNN(snssai models.Snssai) (string, bool) {
	var chosen *DNNConfiguration

	for i := range sm.DNNs {
		c := &sm.DNNs[i]
		if !sameSnssai(c.Snssai, snssai) {
			continue
		}

		if chosen == nil || c.Default {
			chosen = c
		}

		if c.Default {
			break
		}
	}

	if chosen == nil {
		return "", false
	}

	return chosen.DNN, true
}

func (sm *SessionManagementSubscription) ForDNN(snssai models.Snssai, dnn string) (DNNConfiguration, bool) {
	for _, c := range sm.DNNs {
		if c.DNN == dnn && sameSnssai(c.Snssai, snssai) {
			return c, true
		}
	}

	return DNNConfiguration{}, false
}

func (sm *SessionManagementSubscription) Subscribes(snssai models.Snssai) bool {
	return slices.ContainsFunc(sm.DNNs, func(c DNNConfiguration) bool { return sameSnssai(c.Snssai, snssai) })
}

func (sm *SessionManagementSubscription) DefaultAPN() (DNNConfiguration, bool) {
	for _, c := range sm.DNNs {
		if c.Default {
			return c, true
		}
	}

	return DNNConfiguration{}, false
}

func (sm *SessionManagementSubscription) ForAPN(apn string) (DNNConfiguration, bool) {
	var (
		chosen DNNConfiguration
		found  bool
	)

	for _, c := range sm.DNNs {
		if c.DNN != apn {
			continue
		}

		if !found || c.Default {
			chosen, found = c, true
		}

		if c.Default {
			break
		}
	}

	return chosen, found
}

func (s *Subscriptions) profile(ctx context.Context, imsi string) (*db.Profile, error) {
	sub, err := s.subscriber(ctx, imsi)
	if err != nil {
		return nil, err
	}

	profile, err := s.store.GetProfileByID(ctx, sub.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("get profile %s: %w", sub.ProfileID, err)
	}

	return profile, nil
}

func (s *Subscriptions) subscriber(ctx context.Context, imsi string) (*db.Subscriber, error) {
	sub, err := s.store.GetSubscriber(ctx, imsi)
	if errors.Is(err, db.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrSubscriberUnknown, imsi)
	}

	if err != nil {
		return nil, fmt.Errorf("get subscriber %s: %w", imsi, err)
	}

	return sub, nil
}

func ueAMBR(profile *db.Profile) (models.Ambr, error) {
	downlink, err := models.ParseBitRate(profile.UeAmbrDownlink)
	if err != nil {
		return models.Ambr{}, fmt.Errorf("profile %s UE-AMBR downlink: %w", profile.ID, err)
	}

	uplink, err := models.ParseBitRate(profile.UeAmbrUplink)
	if err != nil {
		return models.Ambr{}, fmt.Errorf("profile %s UE-AMBR uplink: %w", profile.ID, err)
	}

	return models.Ambr{Uplink: uplink, Downlink: downlink}, nil
}

func sessionAMBR(p *db.Policy) (models.Ambr, error) {
	uplink, err := models.ParseBitRate(p.SessionAmbrUplink)
	if err != nil {
		return models.Ambr{}, fmt.Errorf("policy %s Session-AMBR uplink: %w", p.ID, err)
	}

	downlink, err := models.ParseBitRate(p.SessionAmbrDownlink)
	if err != nil {
		return models.Ambr{}, fmt.Errorf("policy %s Session-AMBR downlink: %w", p.ID, err)
	}

	return models.Ambr{Uplink: uplink, Downlink: downlink}, nil
}

func snssaiOf(slice *db.NetworkSlice) models.Snssai {
	sd := ""
	if slice.Sd != nil {
		sd = *slice.Sd
	}

	return models.Snssai{Sst: slice.Sst, Sd: sd}
}

func sameSnssai(a, b models.Snssai) bool {
	return a.Sst == b.Sst && models.NormalizeSD(a.Sd) == models.NormalizeSD(b.Sd)
}
