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

func fuzzSeeds(f *testing.F, msgs ...*diameter.Message) {
	f.Helper()

	for _, m := range msgs {
		b, err := m.Marshal()
		if err != nil {
			f.Fatal(err)
		}

		f.Add(b)
	}

	f.Add([]byte{})
}

func FuzzParseProfileUpdateRequest(f *testing.F) {
	must := mustMessage(f)

	fuzzSeeds(f,
		must(NewProfileUpdateRequest(asEnvelope, smsRegistrationRequest())),
		must(NewProfileUpdateRequest(asEnvelope, ProfileUpdateRequest{
			User: UserIdentity{MSISDN: testMSISDN}, WildcardedIMPU: testWildcarded, PrivateIdentity: testPrivate,
			DataReferences: []DataReference{DataRepositoryData, DataDSAI}, UserData: smsRegistrationData,
			Features: FeatureUpdateEff | FeatureUpdateEffEnhance, FeaturesRequired: true,
		})),
	)

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		r, err := ParseProfileUpdateRequest(m)
		if err != nil {
			var avpErr *diameter.AVPError
			if !errors.As(err, &avpErr) {
				t.Fatalf("request parser returned %v, want an AVP error", err)
			}

			return
		}

		if r.FeaturesRequired && r.Features == 0 {
			return
		}

		rebuilt, err := NewProfileUpdateRequest(asEnvelope, r)
		if err != nil {
			t.Fatalf("parsed %+v does not rebuild: %v", r, err)
		}

		again, err := ParseProfileUpdateRequest(rebuilt)
		if err != nil || !reflect.DeepEqual(again, r) {
			t.Fatalf("rebuilt %+v parses as %+v, %v", r, again, err)
		}
	})
}

func FuzzParseProfileUpdateAnswer(f *testing.F) {
	must := mustMessage(f)
	req := profileUpdateRequest()
	dsai := DataDSAI

	fuzzSeeds(f,
		must(NewProfileUpdateAnswer(req, hssIdentity, ProfileUpdate{WildcardedIMPU: testWildcarded, Features: FeatureUpdateEff})),
		must(NewProfileUpdateErrorAnswer(req, hssIdentity, ProfileUpdateError{
			ResultError:      ResultError{Result: tgpp.Experimental(tgpp.ResultErrorTransparentDataOutOfSync)},
			RepositoryDataID: &RepositoryDataID{ServiceIndication: []byte("mmtel"), SequenceNumber: 3},
			DataReference:    &dsai,
		})),
	)

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		a, err := ParseProfileUpdateAnswer(m)

		var pe *ProfileUpdateError

		switch {
		case err == nil:
			rebuilt, err := NewProfileUpdateAnswer(req, hssIdentity, a)
			if err != nil {
				t.Fatalf("parsed %+v does not rebuild: %v", a, err)
			}

			again, err := ParseProfileUpdateAnswer(rebuilt)
			if err != nil || !reflect.DeepEqual(again, a) {
				t.Fatalf("rebuilt %+v parses as %+v, %v", a, again, err)
			}
		case errors.As(err, &pe):
			if pe.WildcardedPublicIdentity != "" && pe.WildcardedIMPU != "" {
				return
			}

			rebuilt, err := NewProfileUpdateErrorAnswer(req, hssIdentity, *pe)
			if err != nil {
				t.Fatalf("parsed %+v does not rebuild: %v", pe, err)
			}

			var again *ProfileUpdateError
			if _, err := ParseProfileUpdateAnswer(rebuilt); !errors.As(err, &again) || !reflect.DeepEqual(*again, *pe) {
				t.Fatalf("rebuilt %+v parses as %v", pe, err)
			}
		case !errors.Is(err, ErrMalformedAnswer):
			t.Fatalf("answer parser returned %v", err)
		}
	})
}

func FuzzParseShData(f *testing.F) {
	supported := true

	full, err := MarshalShData(ShData{SMSRegistration: &SMSRegistrationInfo{IPSMGWNumber: "15550000050", SCAddress: "15550000000", SBISupported: &supported}})
	if err != nil {
		f.Fatal(err)
	}

	f.Add(full)
	f.Add([]byte("<Sh-Data/>"))

	f.Fuzz(func(t *testing.T, b []byte) {
		d, err := ParseShData(b)
		if err != nil {
			if !errors.Is(err, ErrInvalidUserData) {
				t.Fatalf("ParseShData returned %v", err)
			}

			return
		}

		out, err := MarshalShData(d)
		if err != nil {
			t.Fatalf("parsed %+v does not marshal: %v", d, err)
		}

		again, err := ParseShData(out)
		if err != nil || !reflect.DeepEqual(again, d) {
			t.Fatalf("re-parsed %+v as %+v, %v", d, again, err)
		}
	})
}
