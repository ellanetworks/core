// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter"
)

func TestParseResult(t *testing.T) {
	req := &diameter.Message{Flags: diameter.FlagRequest}

	r, err := ParseResult(NewAnswer(req, testIdentity, diameter.ResultSuccess))
	if err != nil || !r.Success() || r.Experimental {
		t.Fatalf("success = %+v, %v", r, err)
	}

	r, err = ParseResult(NewAnswer(req, testIdentity, diameter.ResultUnableToComply))
	if err != nil || r.Success() || r.Code != diameter.ResultUnableToComply {
		t.Fatalf("base error = %+v, %v", r, err)
	}

	r, err = ParseResult(NewExperimentalAnswer(req, testIdentity, ResultErrorAbsentUser))
	if err != nil || r.Success() || !r.IsExperimental(ResultErrorAbsentUser) || r.IsExperimental(ResultErrorUserUnknown) {
		t.Fatalf("experimental = %+v, %v", r, err)
	}

	other := &diameter.Message{AVPs: []diameter.AVP{diameter.Grouped(diameter.AVPExperimentalResult, diameter.AVPFlagMandatory, 0,
		diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, 9999),
		diameter.Unsigned32(diameter.AVPExperimentalResultCode, diameter.AVPFlagMandatory, 0, ResultErrorAbsentUser))}}

	if r, err := ParseResult(other); err != nil || r.VendorID != 9999 || r.IsExperimental(ResultErrorAbsentUser) {
		t.Fatalf("other vendor = %+v, %v", r, err)
	}
}

func TestParseResultMalformed(t *testing.T) {
	for name, m := range map[string]*diameter.Message{
		"empty":             {},
		"short Result-Code": {AVPs: []diameter.AVP{diameter.OctetString(diameter.AVPResultCode, diameter.AVPFlagMandatory, 0, []byte{0x01})}},
		"no Vendor-Id": {AVPs: []diameter.AVP{diameter.Grouped(diameter.AVPExperimentalResult, diameter.AVPFlagMandatory, 0,
			diameter.Unsigned32(diameter.AVPExperimentalResultCode, diameter.AVPFlagMandatory, 0, ResultErrorAbsentUser))}},
		"no code": {AVPs: []diameter.AVP{diameter.Grouped(diameter.AVPExperimentalResult, diameter.AVPFlagMandatory, 0,
			diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, VendorID))}},
	} {
		if _, err := ParseResult(m); !errors.Is(err, ErrMalformedResult) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestResultString(t *testing.T) {
	cases := []struct {
		result Result
		want   string
	}{
		{Result{Code: diameter.ResultSuccess}, "result 2001 DIAMETER_SUCCESS"},
		{Experimental(ResultErrorSMDeliveryFailure), "experimental result 5555 (vendor 10415) DIAMETER_ERROR_SM_DELIVERY_FAILURE"},
		{Result{Code: ResultErrorAbsentUser, Experimental: true, VendorID: 9999}, "experimental result 5550 (vendor 9999)"},
		{Result{Code: 4999}, "result 4999"},
	}

	for _, tc := range cases {
		if got := tc.result.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestResultFailure(t *testing.T) {
	for code, want := range map[uint32]bool{0: false, 1001: false, 2001: false, 3002: true, 4261: true, 5065: true, 6000: false} {
		if got := (Result{Code: code}).Failure(); got != want {
			t.Errorf("Result{%d}.Failure() = %v", code, got)
		}
	}
}

func TestParseFinalResult(t *testing.T) {
	req := &diameter.Message{Flags: diameter.FlagRequest}

	for code, ok := range map[uint32]bool{1001: false, 2001: true, 3002: true, 5012: true, 6000: false} {
		_, err := ParseFinalResult(NewAnswer(req, testIdentity, code))
		if (err == nil) != ok || (err != nil && !errors.Is(err, ErrMalformedResult)) {
			t.Errorf("code %d: err = %v", code, err)
		}
	}

	if _, err := ParseFinalResult(&diameter.Message{}); !errors.Is(err, ErrMalformedResult) {
		t.Errorf("empty answer: err = %v", err)
	}
}

func TestOrSuccess(t *testing.T) {
	if (Result{}).OrSuccess() != (Result{Code: diameter.ResultSuccess}) {
		t.Error("zero result is not DIAMETER_SUCCESS")
	}

	if r := Experimental(ResultFirstRegistration); r.OrSuccess() != r {
		t.Error("non-zero result changed")
	}
}

func TestResultClass(t *testing.T) {
	for _, tc := range []struct {
		result               Result
		transient, permanent bool
	}{
		{Result{Code: diameter.ResultSuccess}, false, false},
		{Result{Code: diameter.ResultTooBusy}, false, false},
		{Experimental(ResultRequestedServiceTemporarilyNotAuthorized), true, false},
		{Result{Code: diameter.ResultUnableToComply}, false, true},
		{Experimental(ResultErrorUserUnknown), false, true},
		{Result{Code: 6000}, false, false},
	} {
		if tc.result.Transient() != tc.transient || tc.result.Permanent() != tc.permanent {
			t.Errorf("%s: Transient() = %v, Permanent() = %v", tc.result, tc.result.Transient(), tc.result.Permanent())
		}
	}
}
