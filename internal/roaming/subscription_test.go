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
	"github.com/ellanetworks/core/nas/eps"
)

func ptr[T any](v T) *T { return &v }

func hplmnSubscription() *s6a.SubscriptionData {
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
			sub := hplmnSubscription()
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

type hplmnSubscriptions map[string]*s6a.SubscriptionData

func (h hplmnSubscriptions) VisitorHPLMN(imsi string) (roaming.HPLMNData, bool) {
	s, ok := h[imsi]
	return roaming.HPLMNData{PLMN: models.PlmnID{Mcc: "999", Mnc: "70"}, Subscription: s}, ok
}

const visitorIMSI = "999700000000001"

func TestAPNSubscriptionCarriesTheHomeAPNConfiguration(t *testing.T) {
	sub := hplmnSubscription()
	sub.MSISDN = "15551234"
	sub.ChargingCharacteristics = "0800"
	sub.APNConfigurationProfile.Configurations[1].PDNType = s6a.PDNTypeIPv4OrIPv6
	sub.APNConfigurationProfile.Configurations[1].ChargingCharacteristics = "0400"

	home := models.PlmnID{Mcc: "999", Mnc: "70"}

	got, err := roaming.APNSubscription(home, sub, "iot")
	if err != nil {
		t.Fatal(err)
	}

	if got.HPLMN != home || got.MSISDN != "15551234" || got.ChargingCharacteristics != "0400" || got.QCI != 9 || got.ARP.PriorityLevel != 8 {
		t.Fatalf("APN subscription = %+v", got)
	}

	if len(got.PDNTypes) != 2 || got.PDNTypes[0] != eps.PDNTypeIPv4 || got.PDNTypes[1] != eps.PDNTypeIPv6 {
		t.Fatalf("PDN types = %v, want IPv4 or IPv6", got.PDNTypes)
	}

	if got.APNAMBR == nil || got.APNAMBR.Uplink.Bps() != 50_000_000 || got.APNAMBR.Downlink.Bps() != 150_000_000 {
		t.Fatalf("APN-AMBR = %+v", got.APNAMBR)
	}

	def, err := roaming.APNSubscription(home, sub, "internet")
	if err != nil || def.ChargingCharacteristics != "0800" || len(def.PDNTypes) != 1 || def.PDNTypes[0] != eps.PDNTypeIPv4 {
		t.Fatalf("APN subscription = %+v, %v, want the subscription-level charging characteristics and IPv4 only", def, err)
	}

	if _, err := roaming.APNSubscription(home, sub, "corporate"); !errors.Is(err, roaming.ErrAPNNotSubscribed) {
		t.Fatalf("error = %v, want %v", err, roaming.ErrAPNNotSubscribed)
	}
}

func TestUEAMBROfAVisitorIsTheMinimumOfHomeAndLocal(t *testing.T) {
	subs := roaming.NewSubscriptions(localSubscriptions{visitor: true}, hplmnSubscriptions{visitorIMSI: hplmnSubscription()})

	ambr, err := subs.UEAMBR(t.Context(), visitorIMSI)
	if err != nil || ambr.Uplink.Bps() != 80_000_000 || ambr.Downlink.Bps() != 100_000_000 {
		t.Fatalf("UE-AMBR = %v, %v, want min(home 80/300 Mbps, local 1000/100 Mbps)", ambr, err)
	}
}

func TestVisitorWithoutAHomeSubscriptionFailsClosed(t *testing.T) {
	subs := roaming.NewSubscriptions(localSubscriptions{visitor: true}, hplmnSubscriptions{})

	if _, err := subs.SelectAPN(t.Context(), visitorIMSI, ""); !errors.Is(err, roaming.ErrNoHPLMNSubscription) {
		t.Fatalf("SelectAPN error = %v, want %v", err, roaming.ErrNoHPLMNSubscription)
	}
}

func TestOwnSubscribersKeepTheLocalSubscription(t *testing.T) {
	subs := roaming.NewSubscriptions(localSubscriptions{}, hplmnSubscriptions{visitorIMSI: hplmnSubscription()})

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
			subs := roaming.NewSubscriptions(localSubscriptions{visitor: true}, hplmnSubscriptions{visitorIMSI: hplmnSubscription()})

			got, err := subs.SelectAPN(t.Context(), visitorIMSI, tc.requested)
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("SelectAPN = %q, %v, want %q, %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestSelectAPNRefusesASubscribedAPNThatIsNotLocal(t *testing.T) {
	sub := fakehss.Subscription(1, fakehss.APN(1, "internet"), fakehss.APN(2, "corporate"))
	subs := roaming.NewSubscriptions(localSubscriptions{visitor: true}, hplmnSubscriptions{visitorIMSI: &sub})

	_, err := subs.SelectAPN(t.Context(), visitorIMSI, "corporate")
	if !errors.Is(err, roaming.ErrNotLocal) || !errors.Is(err, models.ErrUnknownAPN) {
		t.Fatalf("error = %v, want %v", err, roaming.ErrNotLocal)
	}
}
