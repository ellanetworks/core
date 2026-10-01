// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	testPrivate = "001010000000001@ims.mnc001.mcc001.3gppnetwork.org"
	testPublic  = "sip:001010000000001@ims.mnc001.mcc001.3gppnetwork.org"
	testTel     = "tel:+15551230002"
	testServer  = "sip:scscf.ims.mnc001.mcc001.3gppnetwork.org:6060"
	testRealm   = "ims.mnc001.mcc001.3gppnetwork.org"
)

var (
	cscfIdentity = diameter.Identity{OriginHost: "scscf.ims.mnc001.mcc001.3gppnetwork.org", OriginRealm: testRealm}
	hssIdentity  = diameter.Identity{OriginHost: "hss.ims.mnc001.mcc001.3gppnetwork.org", OriginRealm: testRealm}
	cscfEnvelope = tgpp.Envelope{
		SessionID:        "scscf.ims.mnc001.mcc001.3gppnetwork.org;1;1",
		Origin:           cscfIdentity,
		DestinationRealm: testRealm,
	}
	hssEnvelope = tgpp.Envelope{
		SessionID:        "hss.ims.mnc001.mcc001.3gppnetwork.org;1;1",
		Origin:           hssIdentity,
		DestinationHost:  "scscf.ims.mnc001.mcc001.3gppnetwork.org",
		DestinationRealm: testRealm,
	}
)

func roundTrip(t *testing.T, m *diameter.Message) *diameter.Message {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := diameter.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return out
}

func mustMessage(t testing.TB) func(*diameter.Message, error) *diameter.Message {
	return func(m *diameter.Message, err error) *diameter.Message {
		t.Helper()

		if err != nil {
			t.Fatal(err)
		}

		return m
	}
}

func avpResult(t *testing.T, err error) uint32 {
	t.Helper()

	var avpErr *diameter.AVPError
	if !errors.As(err, &avpErr) {
		t.Fatalf("err = %v, want an AVP error", err)
	}

	return avpErr.ResultCode
}

func request(command uint32, avps ...diameter.AVP) *diameter.Message {
	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   command,
		ApplicationID: ApplicationID,
		AVPs:          append(append(hssEnvelope.AVPs(), vendorSpecificApplicationID()), avps...),
	}
}

func without(m *diameter.Message, code, vendorID uint32) *diameter.Message {
	out := *m
	out.AVPs = nil

	for _, a := range m.AVPs {
		if a.Code != code || a.VendorID != vendorID {
			out.AVPs = append(out.AVPs, a)
		}
	}

	return &out
}

func with(m *diameter.Message, avps ...diameter.AVP) *diameter.Message {
	out := *m
	out.AVPs = append(append([]diameter.AVP(nil), m.AVPs...), avps...)

	return &out
}

func requireVendorSpecificApplicationID(t *testing.T, m *diameter.Message) {
	t.Helper()

	a, ok := m.Find(diameter.AVPVendorSpecificApplicationID, 0)
	if !ok {
		t.Fatal("no Vendor-Specific-Application-Id")
	}

	inner, err := a.Grouped()
	if err != nil {
		t.Fatal(err)
	}

	vendor, _ := diameter.Find(inner, diameter.AVPVendorID, 0)
	app, _ := diameter.Find(inner, diameter.AVPAuthApplicationID, 0)

	if v, _ := vendor.Unsigned32(); v != tgpp.VendorID {
		t.Fatalf("Vendor-Id = %d", v)
	}

	if v, _ := app.Unsigned32(); v != ApplicationID {
		t.Fatalf("Auth-Application-Id = %d", v)
	}
}

func TestRequestsCarryTheCxHeader(t *testing.T) {
	must := mustMessage(t)

	for name, m := range map[string]*diameter.Message{
		"UAR": must(NewUserAuthorizationRequest(cscfEnvelope, UserAuthorizationRequest{PrivateIdentity: testPrivate, PublicIdentity: testPublic, VisitedNetwork: testRealm})),
		"LIR": must(NewLocationInfoRequest(cscfEnvelope, LocationInfoRequest{PublicIdentity: testTel})),
		"MAR": must(NewMultimediaAuthRequest(cscfEnvelope, MultimediaAuthRequest{
			PrivateIdentity: testPrivate, PublicIdentity: testPublic, ServerName: testServer, NumberOfItems: 1, Scheme: SchemeDigestAKAv1MD5,
		})),
		"SAR": must(NewServerAssignmentRequest(cscfEnvelope, ServerAssignmentRequest{
			PrivateIdentity: testPrivate, PublicIdentities: []string{testPublic}, ServerName: testServer, Type: AssignmentRegistration,
		})),
		"RTR": must(NewRegistrationTerminationRequest(hssEnvelope, RegistrationTerminationRequest{PrivateIdentity: testPrivate})),
	} {
		t.Run(name, func(t *testing.T) {
			if m.ApplicationID != ApplicationID || m.Flags != diameter.FlagRequest|diameter.FlagProxiable || m.AVPs[0].Code != diameter.AVPSessionID {
				t.Fatalf("header = %+v", m)
			}

			requireVendorSpecificApplicationID(t, m)

			state, _ := m.Find(diameter.AVPAuthSessionState, 0)
			if v, _ := state.Unsigned32(); v != diameter.AuthSessionStateNoStateMaintained {
				t.Fatalf("Auth-Session-State = %d", v)
			}

			if _, ok := m.Find(tgpp.AVPSupportedFeatures, tgpp.VendorID); ok {
				t.Fatal("Supported-Features without features")
			}

			for _, a := range m.AVPs {
				if a.VendorID == tgpp.VendorID && a.Code < 627 && a.Flags&diameter.AVPFlagMandatory == 0 {
					t.Errorf("AVP %d without the M bit", a.Code)
				}
			}
		})
	}
}

func TestFeaturesAdvertised(t *testing.T) {
	req, err := NewUserAuthorizationRequest(cscfEnvelope, UserAuthorizationRequest{
		PrivateIdentity: testPrivate, PublicIdentity: testPublic, VisitedNetwork: testRealm, Features: FeatureIMSRestoration,
	})
	if err != nil {
		t.Fatal(err)
	}

	sf, ok := req.Find(tgpp.AVPSupportedFeatures, tgpp.VendorID)
	if !ok || sf.Flags&diameter.AVPFlagMandatory != 0 {
		t.Fatalf("Supported-Features = %+v", sf)
	}

	got, err := ParseUserAuthorizationRequest(roundTrip(t, req))
	if err != nil || got.Features != FeatureIMSRestoration {
		t.Fatalf("features = %#x, %v", got.Features, err)
	}

	ans := NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultErrorFeatureUnsupported), FeatureAliasIndication)
	requireVendorSpecificApplicationID(t, ans)

	if features(ans) != FeatureAliasIndication {
		t.Fatal("error answer without the HSS features")
	}

	if !tgpp.IsExperimental(func() error { _, err := ParseUserAuthorizationAnswer(ans); return err }(), tgpp.ResultErrorFeatureUnsupported) {
		t.Fatal("feature error not surfaced")
	}
}

func TestErrorAnswer(t *testing.T) {
	req := request(CommandServerAssignment)
	ans := NewErrorAnswer(req, hssIdentity, tgpp.MissingAVP(AVPServerName, tgpp.VendorID), 0)

	requireVendorSpecificApplicationID(t, ans)

	if _, ok := ans.Find(diameter.AVPFailedAVP, 0); !ok {
		t.Fatal("no Failed-AVP")
	}

	var re *ResultError

	_, err := ParseServerAssignmentAnswer(ans)
	if !errors.As(err, &re) || re.Code != diameter.ResultMissingAVP || re.Error() != "cx: request failed with result 5005 DIAMETER_MISSING_AVP" {
		t.Fatalf("err = %v", err)
	}

	if _, err := ParseServerAssignmentAnswer(&diameter.Message{}); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("empty answer = %v", err)
	}
}

func TestValidPublicIdentity(t *testing.T) {
	for id, want := range map[string]bool{
		testPublic:                     true,
		"SIP:alice@example.org":        true,
		"sips:alice@example.org":       true,
		testTel:                        true,
		"tel:0198765432100":            true,
		"sip:":                         false,
		"alice@example.org":            false,
		"":                             false,
		"mailto:alice@example.org":     false,
		"sip:chatlist!.*!@example.org": true,
	} {
		if got := ValidPublicIdentity(id); got != want {
			t.Errorf("ValidPublicIdentity(%q) = %v", id, got)
		}
	}
}

func TestCxResultNames(t *testing.T) {
	for r, want := range map[tgpp.Result]string{
		tgpp.Experimental(tgpp.ResultFirstRegistration):          "experimental result 2001 (vendor 10415) DIAMETER_FIRST_REGISTRATION",
		tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch):   "experimental result 5002 (vendor 10415) DIAMETER_ERROR_IDENTITIES_DONT_MATCH",
		{Code: diameter.ResultAuthorizationRejected}:             "result 5003 DIAMETER_AUTHORIZATION_REJECTED",
		tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered): "experimental result 5003 (vendor 10415) DIAMETER_ERROR_IDENTITY_NOT_REGISTERED",
	} {
		if got := r.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}
