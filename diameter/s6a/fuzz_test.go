// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"errors"
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

func FuzzParseRequests(f *testing.F) {
	mustMessage := must[*diameter.Message](f)
	sub := testSubscription()

	fuzzSeeds(f,
		mustMessage(NewUpdateLocationRequest(mmeEnvelope, testULR())),
		mustMessage(NewAuthenticationInformationRequest(mmeEnvelope, testAIR())),
		mustMessage(NewCancelLocationRequest(hssEnvelope, CancelLocationRequest{IMSI: testIMSI, Type: CancellationSubscriptionWithdrawal})),
		mustMessage(NewInsertSubscriberDataRequest(hssEnvelope, InsertSubscriberDataRequest{IMSI: testIMSI, Subscription: sub})),
		mustMessage(NewDeleteSubscriberDataRequest(hssEnvelope, DeleteSubscriberDataRequest{IMSI: testIMSI, ContextIdentifiers: []uint32{2}})),
		mustMessage(NewPurgeUERequest(mmeEnvelope, PurgeUERequest{IMSI: testIMSI})),
		mustMessage(NewResetRequest(hssEnvelope, ResetRequest{UserIDs: []string{"00101"}, Subscription: &sub})),
		mustMessage(NewNotifyRequest(mmeEnvelope, NotifyRequest{IMSI: testIMSI, Terminal: &TerminalInformation{IMEI: testIMEI}})),
	)

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		ulr, err := ParseUpdateLocationRequest(m)
		requireAVPError(t, err)

		if err == nil {
			if _, err := NewUpdateLocationRequest(mmeEnvelope, ulr); err != nil {
				t.Fatalf("parsed ULR %+v does not rebuild: %v", ulr, err)
			}
		}

		air, err := ParseAuthenticationInformationRequest(m)
		requireAVPError(t, err)

		if err == nil {
			if _, err := NewAuthenticationInformationRequest(mmeEnvelope, air); err != nil {
				t.Fatalf("parsed AIR %+v does not rebuild: %v", air, err)
			}
		}

		_, err = ParseCancelLocationRequest(m)
		requireAVPError(t, err)

		_, err = ParseInsertSubscriberDataRequest(m)
		requireAVPError(t, err)

		_, err = ParseDeleteSubscriberDataRequest(m)
		requireAVPError(t, err)

		_, err = ParsePurgeUERequest(m)
		requireAVPError(t, err)

		_, err = ParseResetRequest(m)
		requireAVPError(t, err)

		_, err = ParseNotifyRequest(m)
		requireAVPError(t, err)
	})
}

func FuzzParseAnswers(f *testing.F) {
	mustMessage := must[*diameter.Message](f)
	req := request(CommandUpdateLocation, mmeEnvelope)
	sub := testSubscription()

	ula := mustMessage(NewUpdateLocationAnswer(req, hssIdentity, UpdateLocation{Subscription: &sub, Flags: ULAFlagSeparationIndication}))
	ula.AVPs = append(ula.AVPs, open5GSSubscriptionData())

	fuzzSeeds(f,
		ula,
		mustMessage(NewAuthenticationInformationAnswer(req, hssIdentity, AuthenticationInformation{EUTRAN: []EUTRANVector{testEUTRANVector(1)}})),
		NewErrorAnswer(req, hssIdentity, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorRoamingNotAllowed), ErrorDiagnostic: ptr(ErrorDiagnosticODBAllAPN)}),
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

		_, err = ParseUpdateLocationAnswer(m)
		check(err)

		_, err = ParseAuthenticationInformationAnswer(m)
		check(err)

		_, err = ParseCancelLocationAnswer(m)
		check(err)

		_, err = ParseInsertSubscriberDataAnswer(m)
		check(err)

		_, err = ParseDeleteSubscriberDataAnswer(m)
		check(err)

		_, err = ParsePurgeUEAnswer(m)
		check(err)

		_, err = ParseResetAnswer(m)
		check(err)

		_, err = ParseNotifyAnswer(m)
		check(err)
	})
}
