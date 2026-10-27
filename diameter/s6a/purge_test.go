// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestPurgeUERoundTrip(t *testing.T) {
	r := PurgeUERequest{IMSI: testIMSI, Features: Features{List1: FeaturePartialPurge}, Flags: PURFlagUEPurgedInMME}

	req := must[*diameter.Message](t)(NewPurgeUERequest(mmeEnvelope, r))
	requireNoVendorSpecificApplicationID(t, req)

	got, err := ParsePurgeUERequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	p := PurgeUE{Features: Features{List1: FeaturePartialPurge}, Flags: PUAFlagFreezeMTMSI}

	ans := must[*diameter.Message](t)(NewPurgeUEAnswer(req, hssIdentity, p))
	if back, err := ParsePurgeUEAnswer(roundTrip(t, ans)); err != nil || back != p {
		t.Fatalf("PUA = %+v, %v", back, err)
	}

	back, err := ParsePurgeUEAnswer(success(req, optionalU32(AVPPUAFlags, uint32(PUAFlagFreezePTMSI|1<<8))))
	if err != nil || back.Flags != PUAFlagFreezePTMSI|1<<8 || back.Flags.Known() {
		t.Fatalf("undefined PUA-Flags bits not kept: %+v, %v", back, err)
	}
}

func TestPurgeUEValidation(t *testing.T) {
	if _, err := NewPurgeUERequest(mmeEnvelope, PurgeUERequest{IMSI: testIMSI, Flags: 1 << 5}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("undefined PUR flag: %v", err)
	}

	if _, err := NewPurgeUERequest(mmeEnvelope, PurgeUERequest{}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("no IMSI: %v", err)
	}

	req := request(CommandPurgeUE, mmeEnvelope)
	if _, err := NewPurgeUEAnswer(req, hssIdentity, PurgeUE{Flags: 1 << 5}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("undefined PUA flag: %v", err)
	}
}

func TestParsePurgeUEErrors(t *testing.T) {
	base := must[*diameter.Message](t)(NewPurgeUERequest(mmeEnvelope, PurgeUERequest{IMSI: testIMSI}))

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no User-Name":  {without(base, diameter.AVPUserName, 0), diameter.ResultMissingAVP},
		"short flags":   {with(base, vendorOctets(AVPPURFlags, []byte{1})), diameter.ResultInvalidAVPLength},
		"unknown M AVP": {with(base, vendorU32(4242, 1)), diameter.ResultAVPUnsupported},
	} {
		if _, err := ParsePurgeUERequest(tt.req); avpResult(t, err) != tt.want {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	got, err := ParsePurgeUERequest(with(base, optionalU32(AVPPURFlags, uint32(PURFlagUEPurgedInSGSN|1<<7))))
	if err != nil || got.Flags != PURFlagUEPurgedInSGSN {
		t.Fatalf("PUR-Flags = %+v, %v", got, err)
	}

	if _, err := ParsePurgeUEAnswer(success(base, vendorOctets(AVPPUAFlags, []byte{1}))); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("short PUA-Flags: %v", err)
	}

	var re *ResultError
	if _, err := ParsePurgeUEAnswer(tgpp.NewExperimentalAnswer(base, hssIdentity, tgpp.ResultErrorUserUnknown)); !errors.As(err, &re) {
		t.Fatalf("user unknown: %v", err)
	}
}
