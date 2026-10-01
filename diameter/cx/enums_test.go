// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestEnumNames(t *testing.T) {
	for _, tt := range []struct {
		value fmt.Stringer
		want  string
	}{
		{AuthorizationRegistrationAndCapabilities, "REGISTRATION_AND_CAPABILITIES"},
		{AuthorizationType(3), "AuthorizationType(3)"},
		{AssignmentNoAssignment, "NO_ASSIGNMENT"},
		{AssignmentTimeoutDeregistrationStoreServer, "TIMEOUT_DEREGISTRATION_STORE_SERVER_NAME"},
		{AssignmentDeregistrationTooMuchData, "DEREGISTRATION_TOO_MUCH_DATA"},
		{AssignmentType(12), "AssignmentType(12)"},
		{ReasonRemoveSCSCF, "REMOVE_S-CSCF"},
		{ReasonCode(4), "ReasonCode(4)"},
		{IdentityWildcardedIMPU, "WILDCARDED_IMPU"},
		{ProfilePartUnregistered, "UNREGISTERED"},
		{SessionCaseOriginatingCDIV, "ORIGINATING_CDIV"},
		{RegistrationTypeReRegistration, "RE-REGISTRATION"},
		{DefaultHandlingSessionTerminated, "SESSION_TERMINATED"},
		{DefaultHandling(9), "DefaultHandling(9)"},
		{Features(0), "0"},
		{FeatureAliasIndication | FeaturePCSCFRestoration, "AliasInd|P-CSCF-Restoration-mechanism"},
		{FeatureSharedIFCSets | Features(0x30), "SiFC|0x30"},
	} {
		if got := tt.value.String(); got != tt.want {
			t.Errorf("%#v.String() = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestZeroResultMeansSuccess(t *testing.T) {
	ans, err := NewLocationInfoAnswer(request(CommandLocationInfo), hssIdentity, LocationInfo{ServerName: testServer})
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseLocationInfoAnswer(roundTrip(t, ans))
	if err != nil || got.Result != (tgpp.Result{Code: diameter.ResultSuccess}) {
		t.Fatalf("result = %+v, %v", got.Result, err)
	}
}

func TestZeroNumberOfItemsRequestsOneVector(t *testing.T) {
	req, err := NewMultimediaAuthRequest(cscfEnvelope, MultimediaAuthRequest{
		PrivateIdentity: testPrivate, PublicIdentity: testPublic, ServerName: testServer, Scheme: SchemeDigestAKAv1MD5,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseMultimediaAuthRequest(roundTrip(t, req))
	if err != nil || got.NumberOfItems != 1 {
		t.Fatalf("NumberOfItems = %d, %v", got.NumberOfItems, err)
	}
}

func TestCommandErrorsUnwrapToResultError(t *testing.T) {
	sae := &ServerAssignmentError{ResultError: ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown), Features: FeatureAliasIndication}}
	rte := &RegistrationTerminationError{ResultError: ResultError{Result: tgpp.Result{Code: diameter.ResultUnableToComply}}}

	for _, err := range []error{fmt.Errorf("wrapped: %w", sae), rte} {
		var re *ResultError
		if !errors.As(err, &re) {
			t.Fatalf("errors.As(%v, *ResultError) = false", err)
		}

		if r, ok := tgpp.ResultOf(err); !ok || r != re.Result {
			t.Fatalf("tgpp.ResultOf(%v) = %+v, %v", err, r, ok)
		}
	}

	if sae.Error() != "cx: request failed with experimental result 5001 (vendor 10415) DIAMETER_ERROR_USER_UNKNOWN" {
		t.Fatalf("Error() = %q", sae.Error())
	}
}
