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
