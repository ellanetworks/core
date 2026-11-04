// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming_test

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/fakehss"
)

const (
	homeRealm = "epc.mnc070.mcc999.3gppnetwork.org"
	hssHost   = "hss.epc.mnc070.mcc999.3gppnetwork.org"
)

var (
	visited  = tgpp.PLMN{MCC: "001", MNC: "01"}
	ellaID   = diameter.Identity{OriginHost: "mmec01.mmegi8001.mme.epc.mnc001.mcc001.3gppnetwork.org", OriginRealm: "epc.mnc001.mcc001.3gppnetwork.org"}
	hssID    = diameter.Identity{OriginHost: hssHost, OriginRealm: homeRealm}
	testKey  = "465b5ce8b199b49faa5f0a2ee238a6bc"
	testOPc  = "cd63cb71954a9f4e48a5994e37a02baf"
	firstSQN = "000000000001"
)

func newHSS(t *testing.T) (*fakehss.Node, *roaming.S6a) {
	t.Helper()

	hss := fakehss.New(ellaID, hssID)
	hss.Provision(visitorIMSI, fakehss.Subscriber{K: testKey, OPc: testOPc, SQN: firstSQN, Subscription: *hplmnSubscription()})

	return hss, roaming.NewS6a(func() roaming.DiameterNode { return hss })
}

func TestAuthenticationInformationRoutesToTheHomeRealm(t *testing.T) {
	hss, client := newHSS(t)

	vec, host, err := client.AuthenticationInformation(t.Context(), roaming.HPLMNPeer{Realm: homeRealm}, visitorIMSI, visited, nil)
	if err != nil {
		t.Fatal(err)
	}

	if host != hssHost || len(vec.KASME) != 32 || len(vec.RAND) != 16 {
		t.Fatalf("vector %+v from %q, want a 32-octet K_ASME from %s", vec, host, hssHost)
	}

	req := hss.Requests()[0]
	env := tgpp.ParseEnvelope(req)

	if env.DestinationRealm != homeRealm || env.DestinationHost != "" {
		t.Fatalf("envelope %+v, want Destination-Realm %s and no Destination-Host", env, homeRealm)
	}

	air, err := s6a.ParseAuthenticationInformationRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	if air.EUTRAN == nil || air.EUTRAN.NumberOfVectors != 1 || !air.EUTRAN.ImmediateResponsePreferred || air.EUTRAN.Resync != nil || air.VisitedPLMN != visited {
		t.Fatalf("AIR %+v, want one immediate E-UTRAN vector for the visited PLMN", air)
	}
}

func TestAuthenticationInformationSendsTheResynchronisationInfo(t *testing.T) {
	hss, client := newHSS(t)

	resync := &s6a.Resync{RAND: make([]byte, 16), AUTS: make([]byte, 14)}
	if _, _, err := client.AuthenticationInformation(t.Context(), roaming.HPLMNPeer{Realm: homeRealm, HSS: hssHost}, visitorIMSI, visited, resync); err == nil {
		t.Fatal("a resynchronisation with a zero AUTS succeeded")
	}

	req := hss.Requests()[0]

	if env := tgpp.ParseEnvelope(req); env.DestinationHost != hssHost {
		t.Fatalf("Destination-Host = %q, want the known HSS %s", env.DestinationHost, hssHost)
	}

	air, err := s6a.ParseAuthenticationInformationRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	if air.EUTRAN.Resync == nil || len(air.EUTRAN.Resync.RAND) != 16 || len(air.EUTRAN.Resync.AUTS) != 14 {
		t.Fatalf("Re-Synchronization-Info = %+v, want RAND‖AUTS", air.EUTRAN.Resync)
	}
}

func TestAuthenticationInformationReportsTheHomeResult(t *testing.T) {
	hss, client := newHSS(t)
	hss.FailAIR(&s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown)})

	_, _, err := client.AuthenticationInformation(t.Context(), roaming.HPLMNPeer{Realm: homeRealm}, visitorIMSI, visited, nil)

	var re *s6a.ResultError
	if !errors.As(err, &re) || !re.IsExperimental(tgpp.ResultErrorUserUnknown) {
		t.Fatalf("error = %v, want DIAMETER_ERROR_USER_UNKNOWN", err)
	}
}

func TestUnreachableHomeHSS(t *testing.T) {
	hss, client := newHSS(t)
	hss.SetDown(true)

	if _, _, err := client.AuthenticationInformation(t.Context(), roaming.HPLMNPeer{Realm: homeRealm}, visitorIMSI, visited, nil); !errors.Is(err, roaming.ErrHSSUnreachable) {
		t.Fatalf("error = %v, want %v", err, roaming.ErrHSSUnreachable)
	}

	stopped := roaming.NewS6a(func() roaming.DiameterNode { return nil })

	if _, err := stopped.UpdateLocation(t.Context(), roaming.HPLMNPeer{Realm: homeRealm}, s6a.UpdateLocationRequest{IMSI: visitorIMSI}); !errors.Is(err, roaming.ErrHSSUnreachable) {
		t.Fatalf("error = %v, want %v", err, roaming.ErrHSSUnreachable)
	}
}

func TestUpdateLocationReturnsTheSubscription(t *testing.T) {
	hss, client := newHSS(t)

	ula, err := client.UpdateLocation(t.Context(), roaming.HPLMNPeer{Realm: homeRealm, HSS: hssHost}, s6a.UpdateLocationRequest{
		IMSI:        visitorIMSI,
		Features:    roaming.ULRFeatures,
		RATType:     tgpp.RATEUTRAN,
		Flags:       s6a.ULRFlagS6aS6dIndicator | s6a.ULRFlagInitialAttachIndicator,
		VisitedPLMN: visited,
	})
	if err != nil {
		t.Fatal(err)
	}

	if ula.HSS.OriginHost != hssHost || ula.Subscription == nil || ula.Subscription.APNConfigurationProfile == nil {
		t.Fatalf("ULA %+v, want the subscription data from %s", ula, hssHost)
	}

	ulr, err := s6a.ParseUpdateLocationRequest(hss.Requests()[0])
	if err != nil {
		t.Fatal(err)
	}

	if ulr.Features != roaming.ULRFeatures {
		t.Fatalf("Supported-Features = %+v, want %+v", ulr.Features, roaming.ULRFeatures)
	}
}
