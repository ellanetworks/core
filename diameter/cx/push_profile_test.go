// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestPushProfileRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]PushProfileRequest{
		"user data": {PrivateIdentity: testPrivate, UserData: []byte("<IMSSubscription/>"), Features: FeatureAliasIndication},
		"charging":  {PrivateIdentity: testPrivate, Charging: &ChargingInformation{PrimaryChargingCollectionFunction: "aaa://cdf.example.org"}},
		"allowed WebRTC functions": {
			PrivateIdentity: testPrivate,
			AllowedWebRTC:   &AllowedWebRTCFunctions{AuthenticationFunctions: []string{"waf1", "waf2"}, WebServerFunctions: []string{"wwsf"}},
		},
		"all allowed functions withdrawn": {PrivateIdentity: testPrivate, AllowedWebRTC: &AllowedWebRTCFunctions{}},
		"everything": {
			PrivateIdentity: testPrivate, UserData: []byte("<IMSSubscription/>"),
			Charging:      &ChargingInformation{PrimaryEventChargingFunction: "aaa://ocs.example.org"},
			AllowedWebRTC: &AllowedWebRTCFunctions{WebServerFunctions: []string{"wwsf"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewPushProfileRequest(hssEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			if req.CommandCode != CommandPushProfile {
				t.Fatalf("command %d", req.CommandCode)
			}

			requireVendorSpecificApplicationID(t, req)
			checkFlags(t, req.AVPs)

			got, err := ParsePushProfileRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestPushProfileRequestValidation(t *testing.T) {
	valid := PushProfileRequest{PrivateIdentity: testPrivate, UserData: []byte("<IMSSubscription/>")}

	for name, tt := range map[string]struct {
		env    tgpp.Envelope
		mutate func(*PushProfileRequest)
	}{
		"no destination host": {cscfEnvelope, func(*PushProfileRequest) {}},
		"no private identity": {hssEnvelope, func(r *PushProfileRequest) { r.PrivateIdentity = "" }},
		"nothing to push":     {hssEnvelope, func(r *PushProfileRequest) { r.UserData = nil }},
		"no primary charging": {hssEnvelope, func(r *PushProfileRequest) {
			r.Charging = &ChargingInformation{SecondaryEventChargingFunction: "aaa://x"}
		}},
		"empty WAF": {hssEnvelope, func(r *PushProfileRequest) {
			r.AllowedWebRTC = &AllowedWebRTCFunctions{AuthenticationFunctions: []string{""}}
		}},
	} {
		r := valid
		tt.mutate(&r)

		if _, err := NewPushProfileRequest(tt.env, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParsePushProfileRequestErrors(t *testing.T) {
	data := diameter.OctetString(AVPUserData, diameter.AVPFlagMandatory, tgpp.VendorID, []byte("<IMSSubscription/>"))
	base := request(CommandPushProfile, userName(testPrivate), data)

	if _, err := ParsePushProfileRequest(base); err != nil {
		t.Fatalf("base request: %v", err)
	}

	empty := without(base, AVPUserData, tgpp.VendorID)

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no Destination-Host": {without(base, diameter.AVPDestinationHost, 0), diameter.ResultMissingAVP},
		"no User-Name":        {without(base, diameter.AVPUserName, 0), diameter.ResultMissingAVP},
		"nothing to push":     {empty, diameter.ResultMissingAVP},
		"two User-Data":       {with(base, data), diameter.ResultAVPOccursTooManyTimes},
		"charging without primary": {
			with(empty, diameter.Grouped(AVPChargingInformation, diameter.AVPFlagMandatory, tgpp.VendorID,
				vendorString(AVPSecondaryEventChargingFunctionName, "aaa://x"))),
			diameter.ResultInvalidAVPValue,
		},
		"empty WAF name": {
			with(empty, diameter.Grouped(AVPAllowedWAFWWSFIdentities, 0, tgpp.VendorID,
				diameter.UTF8String(AVPWebRTCAuthenticationFunctionName, 0, tgpp.VendorID, ""))),
			diameter.ResultInvalidAVPValue,
		},
		"Server-Name": {with(base, vendorString(AVPServerName, testServer)), diameter.ResultAVPUnsupported},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParsePushProfileRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestPushProfileRequestWithOnlyDigestData(t *testing.T) {
	req := request(CommandPushProfile, userName(testPrivate),
		diameter.Grouped(AVPSIPAuthDataItem, diameter.AVPFlagMandatory, tgpp.VendorID, vendorString(AVPSIPAuthenticationScheme, string(SchemeSIPDigest))))

	got, err := ParsePushProfileRequest(req)
	if err != nil || got.PrivateIdentity != testPrivate || got.UserData != nil || got.Charging != nil || got.AllowedWebRTC != nil {
		t.Fatalf("ParsePushProfileRequest = %+v, %v", got, err)
	}
}

func TestPushProfileAnswer(t *testing.T) {
	req := request(CommandPushProfile)

	ans, err := NewPushProfileAnswer(req, cscfIdentity, PushProfile{Features: FeatureAliasIndication})
	if err != nil {
		t.Fatal(err)
	}

	requireVendorSpecificApplicationID(t, ans)

	got, err := ParsePushProfileAnswer(roundTrip(t, ans))
	if err != nil || got != (PushProfile{Result: tgpp.Result{Code: diameter.ResultSuccess}, Features: FeatureAliasIndication}) {
		t.Fatalf("ParsePushProfileAnswer = %+v, %v", got, err)
	}

	if _, err := NewPushProfileAnswer(req, cscfIdentity, PushProfile{Result: tgpp.Experimental(tgpp.ResultErrorTooMuchData)}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("error result = %v", err)
	}

	for _, r := range []tgpp.Result{
		tgpp.Experimental(tgpp.ResultErrorNotSupportedUserData),
		tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		tgpp.Experimental(tgpp.ResultErrorTooMuchData),
		{Code: diameter.ResultUnableToComply},
	} {
		_, err := ParsePushProfileAnswer(NewAnswer(req, cscfIdentity, r, FeatureAliasIndication))

		var re *ResultError
		if !errors.As(err, &re) || re.Result != r || re.Features != FeatureAliasIndication {
			t.Errorf("%s: err = %#v", r, err)
		}
	}

	if _, err := ParsePushProfileAnswer(&diameter.Message{}); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("empty answer = %v", err)
	}
}
