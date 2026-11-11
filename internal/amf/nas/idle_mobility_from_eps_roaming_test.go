// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/nas/fgs"
)

const arrivingVisitorIMSI = "999700000000001"

type arrivingVisitorDB struct {
	*fakeDBInstance
}

func (arrivingVisitorDB) GetSubscriberOrVisitor(_ context.Context, imsi string) (*db.Subscriber, error) {
	partner := "partner"

	return &db.Subscriber{Imsi: imsi, ProfileID: "roamers", RoamingPartnerID: &partner}, nil
}

type arrivingPartners struct {
	partners []db.RoamingPartner
}

func (p arrivingPartners) ListRoamingPartners(context.Context) ([]db.RoamingPartner, error) {
	return p.partners, nil
}

func (arrivingPartners) CountSubscribersAndVisitors(context.Context) (int, error) { return 1, nil }

func (arrivingPartners) SetVisitorMSISDN(context.Context, string, string) error { return nil }

func bothAgreements() []db.RoamingPartner {
	sepp, peer := "sepp.5gc.mnc070.mcc999.3gppnetwork.org", "dea"

	return []db.RoamingPartner{{
		ID: "partner", Name: "partner", ProfileID: "roamers", SEPPFQDN: &sepp, DiameterPeerID: &peer,
		PLMNs: []db.RoamingPLMN{{Mcc: "999", Mnc: "70"}},
	}}
}

func TestIdleArrivalOfAVisitorIsAdmittedOnTheMappedContext(t *testing.T) {
	for _, tc := range []struct {
		name     string
		partners []db.RoamingPartner
		cause    fgs.GMMCause
	}{
		{name: "partner with both agreements", partners: bothAgreements()},
		{name: "no partner for the home network", cause: fgs.GMMCausePLMNNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ue, amfInstance, peer, _ := idleArrivalUE(t)
			amfInstance.DBInstance = arrivingVisitorDB{fakeDBInstance: amfInstance.DBInstance.(*fakeDBInstance)}
			amfInstance.Roaming = arrivingPartners{partners: tc.partners}
			peer.MMContextResponse = arrivingMMContext(mustSUPIFromPrefixed("imsi-" + arrivingVisitorIMSI))

			recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest(), false)

			pass, err := authenticationProcedure(context.Background(), amfInstance, ue)

			if tc.cause != 0 {
				var r *amf.Rejection
				if !errors.As(err, &r) || r.Cause != tc.cause {
					t.Fatalf("authentication procedure = %v, %v, want a rejection with cause %v", pass, err, tc.cause)
				}

				return
			}

			if err != nil || !pass {
				t.Fatalf("authentication procedure = %v, %v, want the mapped context taken without authentication", pass, err)
			}

			v, ok := ue.Visitor()
			if !ok || v.PartnerID != "partner" || v.Registered {
				t.Fatalf("visitor = %+v, %v, want an admitted visitor due to register at its home UDM", v, ok)
			}
		})
	}
}

func TestAVisitorPDUSessionGetsAnEBIOnlyWhenItsHomeSubscribesInterworking(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatal(err)
	}

	ue.AttestS1Mode()
	ue.SetAllow4G(true)
	ue.SetVisitorForTest(roaming.Visitor5G{PartnerID: "partner", Registered: true, Subscription: &wire.SubscriptionDataSets{
		SmfSelData: &wire.SmfSelectionSubscriptionData{SubscribedSnssaiInfos: map[string]wire.SnssaiInfo{
			"1": {DnnInfos: []wire.DnnInfo{
				{Dnn: "internet", LboRoamingAllowed: true, IwkEpsInd: true},
				{Dnn: "iot", LboRoamingAllowed: true},
			}},
		}},
	}})

	if ebi := assignEPSBearerIdentity(context.Background(), ue, 1, models.Snssai{Sst: 1}, "internet"); ebi == 0 {
		t.Error("no EBI for a DNN whose home subscription has iwkEpsInd")
	}

	if ebi := assignEPSBearerIdentity(context.Background(), ue, 2, models.Snssai{Sst: 1}, "iot"); ebi != 0 {
		t.Errorf("EBI %d for a DNN whose home subscription has no iwkEpsInd", ebi)
	}
}
