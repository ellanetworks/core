// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
)

type Visitor struct {
	PartnerID    string
	PartnerName  string
	HomePLMN     tgpp.PLMN
	Home         Home
	Subscription *s6a.SubscriptionData
	Registered   bool
	Confirmed    bool
}

func NewVisitor(partner db.RoamingPartner, plmn db.RoamingPLMN) (Visitor, error) {
	realm, err := diameternode.DiameterRealm(plmn.Mcc, plmn.Mnc)
	if err != nil {
		return Visitor{}, err
	}

	return Visitor{
		PartnerID:   partner.ID,
		PartnerName: partner.Name,
		HomePLMN:    plmnOf(plmn),
		Home:        Home{Realm: realm},
	}, nil
}
