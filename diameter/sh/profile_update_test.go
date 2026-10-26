// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sh

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

var smsRegistrationData = []byte("<Sh-Data/>")

func smsRegistrationRequest() ProfileUpdateRequest {
	return ProfileUpdateRequest{
		User:           UserIdentity{PublicIdentity: testTel},
		DataReferences: []DataReference{DataSMSRegistrationInfo},
		UserData:       smsRegistrationData,
	}
}

func TestProfileUpdateRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]ProfileUpdateRequest{
		"SMS registration":   smsRegistrationRequest(),
		"MSISDN":             {User: UserIdentity{MSISDN: testMSISDN}, DataReferences: []DataReference{DataSMSRegistrationInfo}, UserData: smsRegistrationData},
		"private identity":   {User: UserIdentity{PublicIdentity: testPublic}, PrivateIdentity: testPrivate, DataReferences: []DataReference{DataSTNSR}, UserData: smsRegistrationData},
		"wildcarded PSI":     {User: UserIdentity{PublicIdentity: testPublic}, WildcardedPublicIdentity: testWildcarded, DataReferences: []DataReference{DataRepositoryData}, UserData: smsRegistrationData},
		"wildcarded IMPU":    {User: UserIdentity{PublicIdentity: testPublic}, WildcardedIMPU: testWildcarded, DataReferences: []DataReference{DataRepositoryData}, UserData: smsRegistrationData},
		"external":           {User: UserIdentity{ExternalIdentifier: testExternal}, DataReferences: []DataReference{DataSMSRegistrationInfo}, UserData: smsRegistrationData},
		"features required":  {User: UserIdentity{PublicIdentity: testPublic}, DataReferences: []DataReference{DataRepositoryData}, UserData: smsRegistrationData, Features: FeatureUpdateEff, FeaturesRequired: true},
		"several references": {User: UserIdentity{PublicIdentity: testPublic}, DataReferences: []DataReference{DataRepositoryData, DataDSAI}, UserData: smsRegistrationData, Features: FeatureUpdateEff | FeatureUpdateEffEnhance, FeaturesRequired: true},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewProfileUpdateRequest(asEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			if req.CommandCode != CommandProfileUpdate || req.ApplicationID != ApplicationID {
				t.Fatalf("command %d application %d", req.CommandCode, req.ApplicationID)
			}

			if req.Flags != diameter.FlagRequest|diameter.FlagProxiable {
				t.Fatalf("flags %#x", req.Flags)
			}

			requireVendorSpecificApplicationID(t, req)
			checkFlags(t, req.AVPs)

			got, err := ParseProfileUpdateRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestProfileUpdateRequestValidation(t *testing.T) {
	for name, tt := range map[string]struct {
		env    tgpp.Envelope
		mutate func(*ProfileUpdateRequest)
	}{
		"no session":        {tgpp.Envelope{Origin: asIdentity, DestinationRealm: testRealm}, func(*ProfileUpdateRequest) {}},
		"no user identity":  {asEnvelope, func(r *ProfileUpdateRequest) { r.User = UserIdentity{} }},
		"no data reference": {asEnvelope, func(r *ProfileUpdateRequest) { r.DataReferences = nil }},
		"unknown reference": {asEnvelope, func(r *ProfileUpdateRequest) { r.DataReferences = []DataReference{20} }},
		"no user data":      {asEnvelope, func(r *ProfileUpdateRequest) { r.UserData = nil }},
		"features required": {asEnvelope, func(r *ProfileUpdateRequest) { r.FeaturesRequired = true }},
		"bad wildcarded":    {asEnvelope, func(r *ProfileUpdateRequest) { r.WildcardedIMPU = "chatroom" }},
		"both wildcarded": {asEnvelope, func(r *ProfileUpdateRequest) {
			r.WildcardedPublicIdentity, r.WildcardedIMPU = testWildcarded, testWildcarded
		}},
		"several without enh.": {asEnvelope, func(r *ProfileUpdateRequest) { r.DataReferences = []DataReference{DataRepositoryData, DataDSAI} }},
		"several, enh. without Update-Eff": {asEnvelope, func(r *ProfileUpdateRequest) {
			r.DataReferences = []DataReference{DataRepositoryData, DataDSAI}
			r.Features, r.FeaturesRequired = FeatureUpdateEffEnhance, true
		}},
		"several, features not required": {asEnvelope, func(r *ProfileUpdateRequest) {
			r.DataReferences = []DataReference{DataRepositoryData, DataDSAI}
			r.Features = FeatureUpdateEff | FeatureUpdateEffEnhance
		}},
	} {
		r := smsRegistrationRequest()
		tt.mutate(&r)

		if _, err := NewProfileUpdateRequest(tt.env, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseProfileUpdateRequestErrors(t *testing.T) {
	base := mustMessage(t)(NewProfileUpdateRequest(asEnvelope, smsRegistrationRequest()))
	ref := diameter.Unsigned32(AVPDataReference, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(DataDSAI))

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no user identity":     {without(base, AVPUserIdentity, tgpp.VendorID), diameter.ResultMissingAVP},
		"no data reference":    {without(base, AVPDataReference, tgpp.VendorID), diameter.ResultMissingAVP},
		"no user data":         {without(base, AVPUserData, tgpp.VendorID), diameter.ResultMissingAVP},
		"two user data":        {with(base, diameter.OctetString(AVPUserData, diameter.AVPFlagMandatory, tgpp.VendorID, smsRegistrationData)), diameter.ResultAVPOccursTooManyTimes},
		"several without enh.": {with(base, ref), diameter.ResultAVPOccursTooManyTimes},
		"several, enh. without Update-Eff": {with(base, ref, tgpp.SupportedFeatures{
			VendorID: tgpp.VendorID, FeatureListID: featureListID, FeatureList: uint32(FeatureUpdateEffEnhance), Mandatory: true,
		}.AVP()), diameter.ResultAVPOccursTooManyTimes},
		"unknown reference": {with(without(base, AVPDataReference, tgpp.VendorID),
			diameter.Unsigned32(AVPDataReference, diameter.AVPFlagMandatory, tgpp.VendorID, 20)), diameter.ResultInvalidAVPValue},
		"short reference": {with(without(base, AVPDataReference, tgpp.VendorID),
			diameter.OctetString(AVPDataReference, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{1})), diameter.ResultInvalidAVPLength},
		"bad user identity": {with(without(base, AVPUserIdentity, tgpp.VendorID),
			diameter.Grouped(AVPUserIdentity, diameter.AVPFlagMandatory, tgpp.VendorID)), diameter.ResultInvalidAVPValue},
		"bad wildcarded": {with(base, diameter.UTF8String(AVPWildcardedIMPU, 0, tgpp.VendorID, "chatroom")), diameter.ResultInvalidAVPValue},
		"both wildcarded": {with(base,
			diameter.UTF8String(AVPWildcardedPublicIdentity, 0, tgpp.VendorID, testWildcarded),
			diameter.UTF8String(AVPWildcardedIMPU, 0, tgpp.VendorID, testWildcarded)), diameter.ResultContradictingAVPs},
		"unknown mandatory AVP": {with(base, diameter.UTF8String(9999, diameter.AVPFlagMandatory, tgpp.VendorID, "x")), diameter.ResultAVPUnsupported},
	} {
		if _, err := ParseProfileUpdateRequest(roundTrip(t, tt.req)); avpResult(t, err) != tt.want {
			t.Errorf("%s: err = %v, want result %d", name, err, tt.want)
		}
	}
}

func TestParseProfileUpdateRequestContradictingWildcardedFailedAVPs(t *testing.T) {
	req := with(mustMessage(t)(NewProfileUpdateRequest(asEnvelope, smsRegistrationRequest())),
		diameter.UTF8String(AVPWildcardedPublicIdentity, 0, tgpp.VendorID, testWildcarded),
		diameter.UTF8String(AVPWildcardedIMPU, 0, tgpp.VendorID, testWildcarded))

	_, err := ParseProfileUpdateRequest(roundTrip(t, req))

	var avpErr *diameter.AVPError
	if !errors.As(err, &avpErr) || len(avpErr.AVPs) != 2 ||
		avpErr.AVPs[0].Code != AVPWildcardedPublicIdentity || avpErr.AVPs[1].Code != AVPWildcardedIMPU {
		t.Fatalf("err = %v", err)
	}
}

func TestParseProfileUpdateRequestAcceptsUnrequiredEnhancement(t *testing.T) {
	r := ProfileUpdateRequest{
		User: UserIdentity{PublicIdentity: testPublic}, DataReferences: []DataReference{DataRepositoryData, DataDSAI},
		UserData: smsRegistrationData, Features: FeatureUpdateEff | FeatureUpdateEffEnhance, FeaturesRequired: true,
	}

	req := mustMessage(t)(NewProfileUpdateRequest(asEnvelope, r))
	req = with(without(req, tgpp.AVPSupportedFeatures, tgpp.VendorID), tgpp.SupportedFeatures{
		VendorID: tgpp.VendorID, FeatureListID: featureListID, FeatureList: uint32(r.Features),
	}.AVP())

	got, err := ParseProfileUpdateRequest(roundTrip(t, req))
	if err != nil || len(got.DataReferences) != 2 || got.FeaturesRequired {
		t.Fatalf("parsed %+v, %v", got, err)
	}
}

func TestParseProfileUpdateRequestIgnoresOptionalUnknownAVP(t *testing.T) {
	req := with(mustMessage(t)(NewProfileUpdateRequest(asEnvelope, smsRegistrationRequest())),
		diameter.UTF8String(9999, 0, tgpp.VendorID, "x"))

	if _, err := ParseProfileUpdateRequest(roundTrip(t, req)); err != nil {
		t.Fatal(err)
	}
}

func TestProfileUpdateAnswerRoundTrip(t *testing.T) {
	req := profileUpdateRequest()

	for name, a := range map[string]ProfileUpdate{
		"success":         {Result: tgpp.Result{Code: diameter.ResultSuccess}},
		"features":        {Result: tgpp.Result{Code: diameter.ResultSuccess}, Features: FeatureUpdateEff},
		"wildcarded PSI":  {Result: tgpp.Result{Code: diameter.ResultSuccess}, WildcardedPublicIdentity: testWildcarded},
		"wildcarded IMPU": {Result: tgpp.Result{Code: diameter.ResultSuccess}, WildcardedIMPU: testWildcarded},
	} {
		t.Run(name, func(t *testing.T) {
			ans, err := NewProfileUpdateAnswer(req, hssIdentity, a)
			if err != nil {
				t.Fatal(err)
			}

			if ans.CommandCode != CommandProfileUpdate || ans.Flags&diameter.FlagRequest != 0 {
				t.Fatalf("command %d flags %#x", ans.CommandCode, ans.Flags)
			}

			requireVendorSpecificApplicationID(t, ans)
			checkFlags(t, ans.AVPs)

			got, err := ParseProfileUpdateAnswer(roundTrip(t, ans))
			if err != nil || !reflect.DeepEqual(got, a) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestProfileUpdateAnswerZeroResultMeansSuccess(t *testing.T) {
	ans := mustMessage(t)(NewProfileUpdateAnswer(profileUpdateRequest(), hssIdentity, ProfileUpdate{}))

	got, err := ParseProfileUpdateAnswer(roundTrip(t, ans))
	if err != nil || got.Result != (tgpp.Result{Code: diameter.ResultSuccess}) {
		t.Fatalf("result = %+v, %v", got.Result, err)
	}
}

func TestProfileUpdateAnswerValidation(t *testing.T) {
	req := profileUpdateRequest()

	for name, a := range map[string]ProfileUpdate{
		"failure result":  {Result: tgpp.Experimental(tgpp.ResultErrorUserDataCannotBeModified)},
		"bad wildcarded":  {WildcardedIMPU: "chatroom"},
		"both wildcarded": {WildcardedPublicIdentity: testWildcarded, WildcardedIMPU: testWildcarded},
	} {
		if _, err := NewProfileUpdateAnswer(req, hssIdentity, a); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestProfileUpdateErrorAnswerRoundTrip(t *testing.T) {
	req := profileUpdateRequestWith(FeatureUpdateEff | FeatureUpdateEffEnhance)
	dsai := DataDSAI

	for name, e := range map[string]ProfileUpdateError{
		"cannot be modified": {ResultError: ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserDataCannotBeModified)}},
		"user unknown":       {ResultError: ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown), Features: FeatureUpdateEff}},
		"base error":         {ResultError: ResultError{Result: tgpp.Result{Code: diameter.ResultUnableToComply}}},
		"repository data": {
			ResultError:      ResultError{Result: tgpp.Experimental(tgpp.ResultErrorTransparentDataOutOfSync), Features: FeatureUpdateEff},
			RepositoryDataID: &RepositoryDataID{ServiceIndication: []byte("mmtel"), SequenceNumber: 65535},
		},
		"data reference": {
			ResultError:   ResultError{Result: tgpp.Experimental(tgpp.ResultErrorDSAINotAvailable), Features: FeatureUpdateEff | FeatureUpdateEffEnhance},
			DataReference: &dsai,
		},
		"wildcarded": {
			ResultError:    ResultError{Result: tgpp.Experimental(tgpp.ResultErrorOperationNotAllowed)},
			WildcardedIMPU: testWildcarded,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ans, err := NewProfileUpdateErrorAnswer(req, hssIdentity, e)
			if err != nil {
				t.Fatal(err)
			}

			checkFlags(t, ans.AVPs)

			_, err = ParseProfileUpdateAnswer(roundTrip(t, ans))

			var got *ProfileUpdateError
			if !errors.As(err, &got) || !reflect.DeepEqual(*got, e) {
				t.Fatalf("round trip = %v", err)
			}

			var re *ResultError
			if !errors.As(err, &re) || re.Result != e.Result {
				t.Fatalf("ResultError = %v", err)
			}

			if r, ok := tgpp.ResultOf(err); !ok || r != e.Result {
				t.Fatalf("ResultOf = %v, %v", r, ok)
			}
		})
	}
}

func TestProfileUpdateErrorAnswerValidation(t *testing.T) {
	req := profileUpdateRequestWith(FeatureUpdateEff | FeatureUpdateEffEnhance)
	failure := ResultError{Result: tgpp.Experimental(tgpp.ResultErrorTransparentDataOutOfSync), Features: FeatureUpdateEff | FeatureUpdateEffEnhance}
	unknown := DataReference(20)
	dsai := DataDSAI
	repository := &RepositoryDataID{ServiceIndication: []byte("x"), SequenceNumber: 1}

	for name, e := range map[string]ProfileUpdateError{
		"success result":          {ResultError: ResultError{Result: tgpp.Result{Code: diameter.ResultSuccess}}},
		"no service indication":   {ResultError: failure, RepositoryDataID: &RepositoryDataID{SequenceNumber: 1}},
		"sequence number too big": {ResultError: failure, RepositoryDataID: &RepositoryDataID{ServiceIndication: []byte("x"), SequenceNumber: 65536}},
		"unknown data reference":  {ResultError: failure, DataReference: &unknown},
		"both wildcarded":         {ResultError: failure, WildcardedPublicIdentity: testWildcarded, WildcardedIMPU: testWildcarded},
		"repository data, HSS without Update-Eff": {
			ResultError: ResultError{Result: failure.Result}, RepositoryDataID: repository,
		},
		"data reference, HSS without Update-Eff-Enhance": {
			ResultError: ResultError{Result: failure.Result, Features: FeatureUpdateEff}, DataReference: &dsai,
		},
	} {
		if _, err := NewProfileUpdateErrorAnswer(req, hssIdentity, e); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	for name, tt := range map[string]struct {
		req *diameter.Message
		e   ProfileUpdateError
	}{
		"repository data, AS without Update-Eff":        {profileUpdateRequest(), ProfileUpdateError{ResultError: failure, RepositoryDataID: repository}},
		"data reference, AS without Update-Eff-Enhance": {profileUpdateRequestWith(FeatureUpdateEff), ProfileUpdateError{ResultError: failure, DataReference: &dsai}},
	} {
		if _, err := NewProfileUpdateErrorAnswer(tt.req, hssIdentity, tt.e); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseProfileUpdateAnswerMalformed(t *testing.T) {
	success := mustMessage(t)(NewProfileUpdateAnswer(profileUpdateRequest(), hssIdentity, ProfileUpdate{}))

	for name, ans := range map[string]*diameter.Message{
		"no result":      without(success, diameter.AVPResultCode, 0),
		"bad wildcarded": with(success, diameter.UTF8String(AVPWildcardedIMPU, 0, tgpp.VendorID, "chatroom")),
		"both wildcarded": with(success,
			diameter.UTF8String(AVPWildcardedPublicIdentity, 0, tgpp.VendorID, testWildcarded),
			diameter.UTF8String(AVPWildcardedIMPU, 0, tgpp.VendorID, testWildcarded)),
	} {
		if _, err := ParseProfileUpdateAnswer(roundTrip(t, ans)); !errors.Is(err, ErrMalformedAnswer) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseProfileUpdateErrorAnswerDropsMalformedDetails(t *testing.T) {
	ans := NewAnswer(profileUpdateRequest(), hssIdentity, tgpp.Experimental(tgpp.ResultErrorTransparentDataOutOfSync), 0)
	ans.AVPs = append(ans.AVPs,
		diameter.Grouped(AVPRepositoryDataID, 0, tgpp.VendorID,
			diameter.Unsigned32(AVPSequenceNumber, 0, tgpp.VendorID, 1)),
		diameter.Unsigned32(AVPDataReference, diameter.AVPFlagMandatory, tgpp.VendorID, 20),
	)

	_, err := ParseProfileUpdateAnswer(roundTrip(t, ans))

	var e *ProfileUpdateError
	if !errors.As(err, &e) || e.RepositoryDataID != nil || e.DataReference != nil {
		t.Fatalf("err = %v", err)
	}
}
