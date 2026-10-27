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

func requireAVPError(t *testing.T, err error) {
	t.Helper()

	var avpErr *diameter.AVPError
	if err != nil && !errors.As(err, &avpErr) {
		t.Fatalf("request parser returned %v, want an AVP error", err)
	}
}

func rebuilds[T any](t *testing.T, parsed T, err error, mayRefuse bool, build func(T) (*diameter.Message, error), parse func(*diameter.Message) (T, error)) {
	t.Helper()

	if err != nil {
		return
	}

	m, err := build(parsed)
	if err != nil {
		if mayRefuse && errors.Is(err, ErrInvalidMessage) {
			return
		}

		t.Fatalf("parsed %+v does not rebuild: %v", parsed, err)
	}

	again, err := parse(roundTrip(t, m))
	if err != nil || !reflect.DeepEqual(again, parsed) {
		t.Fatalf("rebuilt %+v parses as %+v, %v", parsed, again, err)
	}
}

func FuzzParseRequests(f *testing.F) {
	mustMessage := must[*diameter.Message](f)
	sub := testSubscription()

	fuzzSeeds(f,
		mustMessage(NewUpdateLocationRequest(mmeEnvelope, testULR())),
		mustMessage(NewAuthenticationInformationRequest(mmeEnvelope, testAIR())),
		mustMessage(NewCancelLocationRequest(hssEnvelope, CancelLocationRequest{IMSI: testIMSI, Type: CancellationSubscriptionWithdrawal})),
		mustMessage(NewInsertSubscriberDataRequest(hssEnvelope, InsertSubscriberDataRequest{IMSI: testIMSI, Subscription: sub, ResetIDs: []ResetID{testResetID}})),
		mustMessage(NewDeleteSubscriberDataRequest(hssEnvelope, testDSR())),
		mustMessage(NewPurgeUERequest(mmeEnvelope, PurgeUERequest{IMSI: testIMSI, Flags: PURFlagUEPurgedInMME})),
		mustMessage(NewResetRequest(hssEnvelope, ResetRequest{UserIDs: []string{"00101"}, ResetIDs: []ResetID{testResetID}, Subscription: &sub})),
		mustMessage(NewResetRequest(hssEnvelope, ResetRequest{ResetIDs: []ResetID{testResetID}, Deletion: ptr(testDSR().SubscriptionDataDeletion)})),
		mustMessage(NewNotifyRequest(mmeEnvelope, testNOR())),
	)

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		ulr, err := ParseUpdateLocationRequest(m)
		requireAVPError(t, err)
		rebuilds(t, ulr, err, false, func(r UpdateLocationRequest) (*diameter.Message, error) {
			return NewUpdateLocationRequest(mmeEnvelope, r)
		}, ParseUpdateLocationRequest)

		air, err := ParseAuthenticationInformationRequest(m)
		requireAVPError(t, err)
		rebuilds(t, air, err, false, func(r AuthenticationInformationRequest) (*diameter.Message, error) {
			return NewAuthenticationInformationRequest(mmeEnvelope, r)
		}, ParseAuthenticationInformationRequest)

		clr, err := ParseCancelLocationRequest(m)
		requireAVPError(t, err)
		rebuilds(t, clr, err, false, func(r CancelLocationRequest) (*diameter.Message, error) {
			return NewCancelLocationRequest(hssEnvelope, r)
		}, ParseCancelLocationRequest)

		idr, err := ParseInsertSubscriberDataRequest(m)
		requireAVPError(t, err)
		rebuilds(t, idr, err, true, func(r InsertSubscriberDataRequest) (*diameter.Message, error) {
			return NewInsertSubscriberDataRequest(hssEnvelope, r)
		}, ParseInsertSubscriberDataRequest)

		dsr, err := ParseDeleteSubscriberDataRequest(m)
		requireAVPError(t, err)
		rebuilds(t, dsr, err, false, func(r DeleteSubscriberDataRequest) (*diameter.Message, error) {
			return NewDeleteSubscriberDataRequest(hssEnvelope, r)
		}, ParseDeleteSubscriberDataRequest)

		pur, err := ParsePurgeUERequest(m)
		requireAVPError(t, err)
		rebuilds(t, pur, err, false, func(r PurgeUERequest) (*diameter.Message, error) {
			return NewPurgeUERequest(mmeEnvelope, r)
		}, ParsePurgeUERequest)

		rsr, err := ParseResetRequest(m)
		requireAVPError(t, err)
		rebuilds(t, rsr, err, true, func(r ResetRequest) (*diameter.Message, error) {
			return NewResetRequest(hssEnvelope, r)
		}, ParseResetRequest)

		nor, err := ParseNotifyRequest(m)
		requireAVPError(t, err)
		rebuilds(t, nor, err, false, func(r NotifyRequest) (*diameter.Message, error) {
			return NewNotifyRequest(mmeEnvelope, r)
		}, ParseNotifyRequest)
	})
}

func FuzzParseAnswers(f *testing.F) {
	mustMessage := must[*diameter.Message](f)
	req := request(CommandUpdateLocation, mmeEnvelope)
	sub := testSubscription()

	fuzzSeeds(f,
		mustMessage(NewUpdateLocationAnswer(req, hssIdentity, UpdateLocation{Subscription: &sub, Flags: ULAFlagSeparationIndication, ResetIDs: []ResetID{testResetID}})),
		open5GSUpdateLocationAnswer(f),
		mustMessage(NewAuthenticationInformationAnswer(req, hssIdentity, testAIA())),
		mustMessage(NewInsertSubscriberDataAnswer(req, mmeIdentity, testIDA())),
		NewErrorAnswer(req, hssIdentity, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorRoamingNotAllowed), ErrorDiagnostic: ptr(ErrorDiagnosticODBAllAPN)}, Features{}),
		mustMessage(NewPurgeUEAnswer(req, hssIdentity, PurgeUE{Flags: PUAFlagFreezeMTMSI})),
	)

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		check := func(err error) {
			var re *ResultError
			if err != nil && !errors.As(err, &re) && !errors.Is(err, ErrMalformedAnswer) {
				t.Fatalf("answer parser returned %v", err)
			}
		}

		ula, err := ParseUpdateLocationAnswer(m)
		check(err)
		rebuilds(t, ula, err, true, func(u UpdateLocation) (*diameter.Message, error) {
			return NewUpdateLocationAnswer(req, u.HSS, u)
		}, ParseUpdateLocationAnswer)

		aia, err := ParseAuthenticationInformationAnswer(m)
		check(err)
		rebuilds(t, aia, err, true, func(a AuthenticationInformation) (*diameter.Message, error) {
			return NewAuthenticationInformationAnswer(req, a.HSS, a)
		}, ParseAuthenticationInformationAnswer)

		_, err = ParseCancelLocationAnswer(m)
		check(err)

		ida, err := ParseInsertSubscriberDataAnswer(m)
		check(err)
		rebuilds(t, ida, err, true, func(a InsertSubscriberData) (*diameter.Message, error) {
			return NewInsertSubscriberDataAnswer(req, mmeIdentity, a)
		}, ParseInsertSubscriberDataAnswer)

		_, err = ParseDeleteSubscriberDataAnswer(m)
		check(err)

		pua, err := ParsePurgeUEAnswer(m)
		check(err)
		rebuilds(t, pua, err, true, func(p PurgeUE) (*diameter.Message, error) {
			return NewPurgeUEAnswer(req, hssIdentity, p)
		}, ParsePurgeUEAnswer)

		_, err = ParseResetAnswer(m)
		check(err)

		_, err = ParseNotifyAnswer(m)
		check(err)
	})
}
