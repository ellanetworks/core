// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/udm"
)

var (
	ErrDNNNotSubscribed5G = errors.New("roaming: DNN not subscribed by the home network on this S-NSSAI")
	ErrHomeRouted5G       = errors.New("roaming: the home network does not allow local breakout for this DNN")
	ErrDNNNotLocal        = errors.New("roaming: DNN not offered to roamers of this partner")
)

type Visitor5G struct {
	PartnerID    string
	PartnerName  string
	Home         HomeTarget
	ConfirmURI   string
	Confirmed    bool
	Registered   bool
	Guami        *wire.Guami
	Subscription *wire.SubscriptionDataSets
	SDM          *SDMSubscription
	SoR          *SoR
	Since        time.Time
	LCSPrivacy   *LCSPrivacy
}

func NewVisitor5G(partner db.RoamingPartner, plmn db.RoamingPLMN, serving models.PlmnID, routingIndicator string) Visitor5G {
	return Visitor5G{
		PartnerID:   partner.ID,
		PartnerName: partner.Name,
		Since:       time.Now(),
		Home: HomeTarget{
			PartnerID:        partner.ID,
			PLMN:             wire.PlmnID{Mcc: plmn.Mcc, Mnc: plmn.Mnc},
			Serving:          wire.PlmnIDFromModel(serving),
			RoutingIndicator: routingIndicator,
		},
	}
}

func (v Visitor5G) AccessAndMobility() *wire.AccessAndMobilitySubscriptionData {
	if v.Subscription == nil {
		return nil
	}

	return v.Subscription.AmData
}

func (v Visitor5G) SmfSelection() *wire.SmfSelectionSubscriptionData {
	if v.Subscription == nil {
		return nil
	}

	return v.Subscription.SmfSelData
}

func (v Visitor5G) HomePLMN() models.PlmnID {
	return v.Home.PLMN.Model()
}

func MergeAccessAndMobility5G(local *udm.AccessAndMobilitySubscription, home *wire.AccessAndMobilitySubscriptionData) (*udm.AccessAndMobilitySubscription, error) {
	out := *local

	if home == nil {
		return &out, nil
	}

	if home.SubscribedUeAmbr != nil {
		ambr, err := home.SubscribedUeAmbr.Model()
		if err != nil {
			return nil, fmt.Errorf("home UE-AMBR: %w", err)
		}

		out.UEAMBR = local.UEAMBR.Min(ambr)
	}

	if home.Nssai != nil {
		subscribed := slices.Concat(home.Nssai.DefaultSingleNssais, home.Nssai.SingleNssais)

		out.SubscribedSNSSAIs = slices.DeleteFunc(slices.Clone(local.SubscribedSNSSAIs), func(s models.Snssai) bool {
			return !slices.ContainsFunc(subscribed, func(h wire.Snssai) bool { return h.Equal(wire.SnssaiFromModel(s)) })
		})
	}

	return &out, nil
}

func DefaultDNN5G(home *wire.SmfSelectionSubscriptionData, snssai models.Snssai) (string, bool) {
	if home == nil {
		return "", false
	}

	info, ok := home.SnssaiInfo(wire.SnssaiFromModel(snssai))
	if !ok {
		return "", false
	}

	d, ok := info.DefaultDnn()
	if !ok {
		return "", false
	}

	return strings.ToLower(d.Dnn), true
}

func BreakoutDNN5G(home *wire.SmfSelectionSubscriptionData, local *udm.SessionManagementSubscription, snssai models.Snssai, dnn string) error {
	if home == nil {
		return ErrDNNNotSubscribed5G
	}

	info, ok := home.SnssaiInfo(wire.SnssaiFromModel(snssai))
	if !ok {
		return fmt.Errorf("%w: S-NSSAI %s", ErrDNNNotSubscribed5G, wire.SnssaiFromModel(snssai))
	}

	d, ok := info.Dnn(dnn)
	if !ok || d.DnnBarred {
		return fmt.Errorf("%w: %q", ErrDNNNotSubscribed5G, dnn)
	}

	c, ok := local.ForDNN(snssai, dnn)
	if !ok || c.IMS {
		return fmt.Errorf("%w: %q", ErrDNNNotLocal, dnn)
	}

	if !d.LboRoamingAllowed {
		return fmt.Errorf("%w: %q", ErrHomeRouted5G, dnn)
	}

	return nil
}

var ErrNoVisitorContext = errors.New("roaming: no visitor context for the callback")
