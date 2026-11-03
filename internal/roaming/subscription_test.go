// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/fakehss"
	"github.com/ellanetworks/core/internal/udm"
)

func ptr[T any](v T) *T { return &v }

func homeSubscription() *s6a.SubscriptionData {
	sub := fakehss.Subscription(1, fakehss.APN(1, "internet"), fakehss.APN(2, "iot"), fakehss.APN(3, "ims"))

	return &sub
}

func TestBreakoutRules(t *testing.T) {
	odb := func(bits s6a.OperatorDeterminedBarring) func(*s6a.SubscriptionData) {
		return func(s *s6a.SubscriptionData) {
			s.SubscriberStatus = ptr(s6a.SubscriberStatusOperatorDeterminedBarring)
			s.OperatorDeterminedBarring = ptr(bits)
		}
	}

	apn := func(f func(*s6a.APNConfiguration)) func(*s6a.SubscriptionData) {
		return func(s *s6a.SubscriptionData) { f(&s.APNConfigurationProfile.Configurations[0]) }
	}

	for _, tc := range []struct {
		name    string
		apn     string
		modify  func(*s6a.SubscriptionData)
		want    string
		wantErr error
	}{
		{name: "default APN", want: "internet"},
		{name: "requested APN", apn: "iot", want: "iot"},
		{name: "APN not subscribed", apn: "corporate", wantErr: roaming.ErrAPNNotSubscribed},
		{name: "wildcard APN", apn: "corporate", modify: func(s *s6a.SubscriptionData) {
			s.APNConfigurationProfile.Configurations = append(s.APNConfigurationProfile.Configurations, fakehss.APN(9, s6a.WildcardAPN))
		}, want: "corporate"},
		{name: "no APN configuration profile", modify: func(s *s6a.SubscriptionData) { s.APNConfigurationProfile = nil }, wantErr: roaming.ErrAPNNotSubscribed},
		{name: "IMS APN", apn: "IMS", wantErr: roaming.ErrIMSAPN},
		{name: "VPLMN dynamic address not allowed", modify: apn(func(c *s6a.APNConfiguration) { c.VPLMNDynamicAddressAllowed = false }), wantErr: roaming.ErrHomeRouted},
		{name: "static PGW without an allocation type", modify: apn(func(c *s6a.APNConfiguration) {
			c.PDNGateway = &s6a.MIP6AgentInfo{HomeAgentHost: "pgw.example.org"}
		}), wantErr: roaming.ErrHomeRouted},
		{name: "static PGW", modify: apn(func(c *s6a.APNConfiguration) {
			c.PDNGateway = &s6a.MIP6AgentInfo{HomeAgentHost: "pgw.example.org"}
			c.PDNGWAllocationType = ptr(s6a.PDNGWAllocationStatic)
		}), wantErr: roaming.ErrHomeRouted},
		{name: "dynamically allocated PGW", modify: apn(func(c *s6a.APNConfiguration) {
			c.PDNGateway = &s6a.MIP6AgentInfo{HomeAgentHost: "pgw.example.org"}
			c.PDNGWAllocationType = ptr(s6a.PDNGWAllocationDynamic)
		}), want: "internet"},
		{name: "static served party IP address", modify: apn(func(c *s6a.APNConfiguration) {
			c.ServedPartyIPAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.1")}
		}), wantErr: roaming.ErrHomeRouted},
		{name: "all packet oriented services barred", modify: odb(s6a.ODBAllPacketOrientedServices), wantErr: roaming.ErrBarred},
		{name: "roamer access to VPLMN APs barred", modify: odb(s6a.ODBRoamerAccessVPLMNAP), wantErr: roaming.ErrBarred},
		{name: "roamer access to HPLMN APs barred", modify: odb(s6a.ODBRoamerAccessHPLMNAP), want: "internet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sub := homeSubscription()
			if tc.modify != nil {
				tc.modify(sub)
			}

			got, _, err := roaming.Breakout(sub, tc.apn)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}

			if got != tc.want {
				t.Fatalf("APN = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBreakoutNeverUsesTheWildcardAsTheDefault(t *testing.T) {
	sub := fakehss.Subscription(9, fakehss.APN(1, "internet"), fakehss.APN(9, s6a.WildcardAPN))

	if _, _, err := roaming.Breakout(&sub, ""); !errors.Is(err, roaming.ErrAPNNotSubscribed) {
		t.Fatalf("error = %v, want %v", err, roaming.ErrAPNNotSubscribed)
	}
}

type localSubscriptions struct {
	visitor bool
	err     error
}

func localDNN(dnn string, def, ims bool) udm.DNNConfiguration {
	return udm.DNNConfiguration{
		Snssai:      models.Snssai{Sst: 1},
		DNN:         dnn,
		IMS:         ims,
		Default:     def,
		Var5qi:      7,
		Arp:         models.Arp{PriorityLevel: 3},
		SessionAMBR: models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")},
	}
}

func (l localSubscriptions) IsHomeSubscriber(context.Context, string) (bool, error) {
	return !l.visitor, nil
}

func (l localSubscriptions) AccessAndMobility(context.Context, string) (*udm.AccessAndMobilitySubscription, error) {
	return &udm.AccessAndMobilitySubscription{
		UEAMBR:  models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("100 Mbps")},
		Allow4G: true,
		Visitor: l.visitor,
	}, l.err
}

func (l localSubscriptions) UEAMBR(context.Context, string) (models.Ambr, error) {
	return models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("100 Mbps")}, l.err
}

func (l localSubscriptions) SessionManagement(context.Context, string) (*udm.SessionManagementSubscription, error) {
	if l.err != nil {
		return nil, l.err
	}

	return &udm.SessionManagementSubscription{
		DNNs: []udm.DNNConfiguration{
			localDNN("iot", true, false),
			localDNN("internet", false, false),
			localDNN("ims", false, true),
			localDNN("local-only", false, false),
		},
		Visitor: l.visitor,
	}, nil
}

func (localSubscriptions) FramedRoutes(context.Context, string, string) ([]netip.Prefix, error) {
	return nil, nil
}

func (localSubscriptions) StaticIP(context.Context, string, string, bool) (netip.Addr, bool, error) {
	return netip.Addr{}, false, nil
}

type homes map[string]*s6a.SubscriptionData

func (h homes) VisitorHome(imsi string) (roaming.HomeData, bool) {
	s, ok := h[imsi]
	return roaming.HomeData{PLMN: models.PlmnID{Mcc: "999", Mnc: "70"}, Subscription: s}, ok
}

const visitorIMSI = "999700000000001"

func TestSessionManagementOfAVisitorMergesHomeAndLocalPolicy(t *testing.T) {
	subs := roaming.NewSubscriptions(localSubscriptions{visitor: true}, homes{visitorIMSI: homeSubscription()})

	sm, err := subs.SessionManagement(t.Context(), visitorIMSI)
	if err != nil {
		t.Fatal(err)
	}

	if len(sm.DNNs) != 2 {
		t.Fatalf("DNNs = %+v, want internet and iot only (no IMS, no local-only DNN)", sm.DNNs)
	}

	if sm.HomePLMN != (models.PlmnID{Mcc: "999", Mnc: "70"}) {
		t.Fatalf("home PLMN = %+v, want 999/70", sm.HomePLMN)
	}

	def, ok := sm.DefaultAPN()
	if !ok || def.DNN != "internet" {
		t.Fatalf("default APN = %+v, want the home default APN internet", def)
	}

	c, ok := sm.ForAPN("iot")
	if !ok || c.Default {
		t.Fatalf("iot = %+v, want a non-default configuration", c)
	}

	if c.Var5qi != 7 || c.Arp.PriorityLevel != 3 {
		t.Fatalf("5QI/ARP = %d/%d, want the local 7/3", c.Var5qi, c.Arp.PriorityLevel)
	}

	if c.SessionAMBR.Uplink.Bps() != 50_000_000 || c.SessionAMBR.Downlink.Bps() != 100_000_000 {
		t.Fatalf("Session-AMBR = %v, want min(home 50/150 Mbps, local 100/100 Mbps)", c.SessionAMBR)
	}
}

func TestUEAMBROfAVisitorIsTheMinimumOfHomeAndLocal(t *testing.T) {
	subs := roaming.NewSubscriptions(localSubscriptions{visitor: true}, homes{visitorIMSI: homeSubscription()})

	am, err := subs.AccessAndMobility(t.Context(), visitorIMSI)
	if err != nil {
		t.Fatal(err)
	}

	if am.UEAMBR.Uplink.Bps() != 80_000_000 || am.UEAMBR.Downlink.Bps() != 100_000_000 {
		t.Fatalf("UE-AMBR = %v, want min(home 80/300 Mbps, local 1000/100 Mbps)", am.UEAMBR)
	}

	ambr, err := subs.UEAMBR(t.Context(), visitorIMSI)
	if err != nil || !ambr.Equal(am.UEAMBR) {
		t.Fatalf("UEAMBR = %v, %v, want %v", ambr, err, am.UEAMBR)
	}
}

func TestVisitorWithoutAHomeSubscriptionFailsClosed(t *testing.T) {
	subs := roaming.NewSubscriptions(localSubscriptions{visitor: true}, homes{})

	if _, err := subs.SessionManagement(t.Context(), visitorIMSI); !errors.Is(err, roaming.ErrNoHomeSubscription) {
		t.Fatalf("SessionManagement error = %v, want %v", err, roaming.ErrNoHomeSubscription)
	}

	if _, err := subs.AccessAndMobility(t.Context(), visitorIMSI); !errors.Is(err, roaming.ErrNoHomeSubscription) {
		t.Fatalf("AccessAndMobility error = %v, want %v", err, roaming.ErrNoHomeSubscription)
	}

	if _, err := subs.SelectAPN(t.Context(), visitorIMSI, ""); !errors.Is(err, roaming.ErrNoHomeSubscription) {
		t.Fatalf("SelectAPN error = %v, want %v", err, roaming.ErrNoHomeSubscription)
	}
}

func TestHomeSubscribersKeepTheLocalSubscription(t *testing.T) {
	subs := roaming.NewSubscriptions(localSubscriptions{}, homes{visitorIMSI: homeSubscription()})

	sm, err := subs.SessionManagement(t.Context(), visitorIMSI)
	if err != nil {
		t.Fatal(err)
	}

	if len(sm.DNNs) != 4 {
		t.Fatalf("DNNs = %+v, want the four local DNNs", sm.DNNs)
	}

	apn, err := subs.SelectAPN(t.Context(), visitorIMSI, "")
	if err != nil || apn != "iot" {
		t.Fatalf("SelectAPN = %q, %v, want the local default iot", apn, err)
	}

	if _, err := subs.SelectAPN(t.Context(), visitorIMSI, "corporate"); !errors.Is(err, models.ErrUnknownAPN) {
		t.Fatalf("SelectAPN error = %v, want %v", err, models.ErrUnknownAPN)
	}
}

func TestSelectAPNForAVisitor(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested string
		want      string
		wantErr   error
	}{
		{name: "home default", want: "internet"},
		{name: "subscribed and local", requested: "iot", want: "iot"},
		{name: "not subscribed at home", requested: "local-only", wantErr: roaming.ErrAPNNotSubscribed},
		{name: "IMS", requested: "ims", wantErr: roaming.ErrIMSAPN},
	} {
		t.Run(tc.name, func(t *testing.T) {
			subs := roaming.NewSubscriptions(localSubscriptions{visitor: true}, homes{visitorIMSI: homeSubscription()})

			got, err := subs.SelectAPN(t.Context(), visitorIMSI, tc.requested)
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("SelectAPN = %q, %v, want %q, %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestSelectAPNRefusesASubscribedAPNThatIsNotLocal(t *testing.T) {
	sub := fakehss.Subscription(1, fakehss.APN(1, "internet"), fakehss.APN(2, "corporate"))
	subs := roaming.NewSubscriptions(localSubscriptions{visitor: true}, homes{visitorIMSI: &sub})

	_, err := subs.SelectAPN(t.Context(), visitorIMSI, "corporate")
	if !errors.Is(err, roaming.ErrNotLocal) || !errors.Is(err, models.ErrUnknownAPN) {
		t.Fatalf("error = %v, want %v", err, roaming.ErrNotLocal)
	}
}
