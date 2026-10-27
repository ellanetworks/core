// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"strings"
	"testing"
)

func fuzzDecode[T any, P interface {
	*T
	Checker
}](f *testing.F, roundTrip bool, seeds ...string) {
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		v, err := Decode[T, P](data)
		if err != nil {
			return
		}

		if err := P(&v).Check(Lenient); err != nil {
			t.Fatalf("decoded value fails its own check: %v", err)
		}

		if !roundTrip || P(&v).Check(Strict) != nil {
			return
		}

		b, err := Encode(P(&v))
		if err != nil {
			t.Fatalf("strict-valid value does not encode: %v", err)
		}

		again, err := Decode[T, P](b)
		if err != nil {
			t.Fatalf("%s does not decode: %v", b, err)
		}

		b2, _ := json.Marshal(P(&again))
		if string(b) != string(b2) {
			t.Fatalf("round trip\n%s\n%s", b, b2)
		}
	})
}

func FuzzDecodeSearchResult(f *testing.F) {
	fuzzDecode[SearchResult](f, true, searchResult, `{"validityPeriod":0,"nfInstances":[]}`)
}

func FuzzDecodeNFProfile(f *testing.F) {
	fuzzDecode[NFProfile](f, true, ausfProfile, `{"nfInstanceId":"x","nfType":"UDM","nfStatus":"REGISTERED","nfServiceList":{"a":{}}}`)
}

func FuzzDecodeUEAuthenticationCtx(f *testing.F) {
	fuzzDecode[UEAuthenticationCtx](f, false, `{"authType":"5G_AKA","5gAuthData":`+testAv+`,"_links":{"5g-aka":{"href":"https://a.example.com/x"}}}`,
		`{"authType":"EAP_TLS","5gAuthData":"AA==","_links":{"eap-session":[{"href":"x"}]}}`)
}

func FuzzDecodeConfirmationDataResponse(f *testing.F) {
	fuzzDecode[ConfirmationDataResponse](f, true, `{"authResult":"AUTHENTICATION_SUCCESS","supi":"imsi-001010000000001","kseaf":"`+testRand+testAutn+`"}`)
}

func FuzzDecodeAmData(f *testing.F) {
	fuzzDecode[AccessAndMobilitySubscriptionData](f, false, `{"subscribedUeAmbr":{"uplink":"1 Gbps","downlink":"1 Gbps"},"nssai":{"defaultSingleNssais":[{"sst":1}]},`+
		`"sorInfo":{"ackInd":true,"provisioningTime":"2026-10-27T10:00:00Z","steeringContainer":[{"plmnId":{"mcc":"001","mnc":"01"}}]}}`)
}

func FuzzDecodeSmfSelectData(f *testing.F) {
	fuzzDecode[SmfSelectionSubscriptionData](f, true, `{"subscribedSnssaiInfos":{"1":{"dnnInfos":[{"dnn":"internet","defaultDnnIndicator":true}]}}}`)
}

func FuzzDecodeSmData(f *testing.F) {
	fuzzDecode[SmSubsData](f, false, smData, `{"sharedSmSubsDataIds":["00101-a"],"individualSmSubsData":[]}`)
}

func FuzzDecodeSdmSubscription(f *testing.F) {
	fuzzDecode[SdmSubscription](f, true, `{"nfInstanceId":"54804518-4191-46b3-955c-ac631f953ed8","callbackReference":"https://a.example.com/cb","monitoredResourceUris":["/a"]}`)
}

func FuzzDecodeModificationNotification(f *testing.F) {
	fuzzDecode[ModificationNotification](f, true, `{"notifyItems":[{"resourceId":"/r","changes":[{"op":"ADD","path":"/a","newValue":1}]}]}`)
}

func FuzzDecodeDeregistrationData(f *testing.F) {
	fuzzDecode[DeregistrationData](f, true, `{"deregReason":"UE_INITIAL_REGISTRATION","accessType":"3GPP_ACCESS"}`)
}

func FuzzDecodeProblemDetails(f *testing.F) {
	fuzzDecode[ProblemDetails](f, true, `{"status":403,"cause":"USER_NOT_FOUND","invalidParams":[{"param":"/a"}]}`)
}

func FuzzBitRate(f *testing.F) {
	for _, s := range []string{"100 Mbps", "1.5 Kbps", "0 bps", "18446744073709551615 bps", "0.0000000000001 Tbps", "1 Pbps"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		bps, err := BitRate(s).Bps()
		if err != nil {
			return
		}

		canonical := BitRateFromBps(bps)

		again, err := canonical.Bps()
		if err != nil || again != bps {
			t.Fatalf("%q -> %d -> %q -> %d, %v", s, bps, canonical, again, err)
		}
	})
}

func FuzzParseSuci(f *testing.F) {
	for _, s := range []string{"suci-0-001-01-0000-0-0-0000000001", "suci-0-999-070-12-1-27-abcdef0123", "suci-1-example.com-0-0-0-user"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		suci, err := ParseSuci(s)
		if err != nil {
			return
		}

		if !ValidSuci(s) || !suci.PlmnID().Valid() || suci.SchemeOutput == "" || !ValidRoutingIndicator(suci.RoutingIndicator) {
			t.Fatalf("%q -> %+v", s, suci)
		}
	})
}

func FuzzParseNetworkFQDN(f *testing.F) {
	for _, s := range []string{"sepp.5gc.mnc070.mcc999.3gppnetwork.org", "A.b.5GC.mnc001.mcc001.3gppnetwork.org.", "5gc.mnc001.mcc001.3gppnetwork.org"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		n, ok := ParseNetworkFQDN(s)
		if !ok {
			return
		}

		again, ok := ParseNetworkFQDN(n.String())
		if !ok || again != n || !ValidFqdn(n.String()) {
			t.Fatalf("%q -> %+v -> %q -> %+v, %v", s, n, n.String(), again, ok)
		}

		if !n.InPLMN(PlmnID{Mcc: n.Mcc, Mnc: n.Mnc}) || strings.ToLower(n.String()) != n.String() {
			t.Fatalf("%+v", n)
		}
	})
}

func FuzzSupportedFeaturesIntersect(f *testing.F) {
	f.Add("1", "3")
	f.Add("0A1", "fFf0")
	f.Add("", "1")

	f.Fuzz(func(t *testing.T, a, b string) {
		x, y := SupportedFeatures(a), SupportedFeatures(b)
		if x.Check(Strict) != nil || y.Check(Strict) != nil {
			return
		}

		got := x.Intersect(y)
		if got != y.Intersect(x) || got.Check(Strict) != nil || got.Intersect(x) != got {
			t.Fatalf("%q ∩ %q = %q", a, b, got)
		}

		for n := 1; n <= 4*max(len(a), len(b))+4; n++ {
			if got.Has(n) != (x.Has(n) && y.Has(n)) {
				t.Fatalf("%q ∩ %q = %q: feature %d", a, b, got, n)
			}
		}
	})
}
