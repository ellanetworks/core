// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/wire"
)

type N26Store interface {
	GetSubscriberOrVisitor(ctx context.Context, imsi string) (*db.Subscriber, error)
	GetRoamingPartner(ctx context.Context, id string) (*db.RoamingPartner, error)
	GetProfileByID(ctx context.Context, id string) (*db.Profile, error)
}

type N26 struct {
	store N26Store
	now   func() time.Time
}

func NewN26(store N26Store) *N26 {
	return &N26{store: store, now: time.Now}
}

func (n *N26) Allowed(ctx context.Context, imsi string) (bool, error) {
	sub, err := n.store.GetSubscriberOrVisitor(ctx, imsi)
	if errors.Is(err, db.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("get subscriber or visitor %s: %w", imsi, err)
	}

	if sub.RoamingPartnerID == nil {
		return true, nil
	}

	partner, err := n.store.GetRoamingPartner(ctx, *sub.RoamingPartnerID)
	if errors.Is(err, db.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("get roaming partner %s: %w", *sub.RoamingPartnerID, err)
	}

	profile, err := n.store.GetProfileByID(ctx, partner.ProfileID)
	if err != nil {
		return false, fmt.Errorf("get profile %s of roaming partner %s: %w", partner.ProfileID, partner.Name, err)
	}

	return N26Agreed(*partner, *profile, imsi, n.now()), nil
}

func N26Agreed(partner db.RoamingPartner, profile db.Profile, imsi string, now time.Time) bool {
	if partner.DiameterPeerID == nil || partner.SEPPFQDN == nil || !profile.Allow4G || !profile.Allow5G {
		return false
	}

	if _, ok := partner.PLMNOfIMSI(imsi); !ok {
		return false
	}

	for _, regType := range []string{db.UERegistrationTypeMME, db.UERegistrationTypeAMF3GPPAccess} {
		if _, withdrawn := partner.Withdrawal(imsi, regType, now); withdrawn {
			return false
		}
	}

	return true
}

func EPSInterworkingSubscribed(sel *wire.SmfSelectionSubscriptionData, serving models.Snssai, dnn string) bool {
	if sel == nil {
		return false
	}

	info, ok := sel.SnssaiInfo(wire.SnssaiFromModel(HomeSnssai(serving)))
	if !ok {
		return false
	}

	d, ok := info.Dnn(dnn)

	return ok && d.IwkEpsInd && !d.DnnBarred
}

func FiveGSInterworkingSubscribed(home *s6a.SubscriptionData, apn string) bool {
	if home == nil || home.APNConfigurationProfile == nil || home.FiveGCNotAllowed() {
		return false
	}

	c, ok := home.APNConfigurationProfile.Resolve(apn)

	return ok && c.Interworking5GSIndicator
}
