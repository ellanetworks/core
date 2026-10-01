// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestSessionTerminationRoundTrip(t *testing.T) {
	for _, env := range []tgpp.Envelope{afEnvelope, {SessionID: "s;1", Origin: afIdentity, DestinationHost: pcrfIdentity.OriginHost, DestinationRealm: testEPCRealm}} {
		for c := TerminationLogout; c <= maxTerminationCause; c++ {
			req, err := NewSessionTerminationRequest(env, SessionTerminationRequest{Cause: c})
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseSessionTerminationRequest(roundTrip(t, req))
			if err != nil || got.Cause != c {
				t.Fatalf("round trip of %s = %+v, %v", c, got, err)
			}
		}
	}

	for _, c := range []TerminationCause{0, 9} {
		if _, err := NewSessionTerminationRequest(afEnvelope, SessionTerminationRequest{Cause: c}); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("cause %d: err = %v", c, err)
		}
	}
}

func TestParseSessionTerminationRequestErrors(t *testing.T) {
	base := request(CommandSessionTermination, afEnvelope,
		diameter.Unsigned32(diameter.AVPTerminationCause, diameter.AVPFlagMandatory, 0, uint32(TerminationLogout)))

	for name, tc := range map[string]struct {
		msg    *diameter.Message
		result uint32
	}{
		"no Termination-Cause": {without(base, diameter.AVPTerminationCause, 0), diameter.ResultMissingAVP},
		"Termination-Cause 0": {
			with(without(base, diameter.AVPTerminationCause, 0), diameter.Unsigned32(diameter.AVPTerminationCause, diameter.AVPFlagMandatory, 0, 0)),
			diameter.ResultInvalidAVPValue,
		},
		"two Termination-Cause": {
			with(base, diameter.Unsigned32(diameter.AVPTerminationCause, diameter.AVPFlagMandatory, 0, 4)),
			diameter.ResultAVPOccursTooManyTimes,
		},
		"Specific-Action": {with(base, vendorUnsigned(AVPSpecificAction, 1)), diameter.ResultAVPUnsupported},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseSessionTerminationRequest(tc.msg)
			if got := avpError(t, err).ResultCode; got != tc.result {
				t.Fatalf("result = %d, want %d (%v)", got, tc.result, err)
			}
		})
	}

	kamailio := with(base,
		vendorOctets(AVPAFApplicationIdentifier, []byte("IMS Services")),
		vendorUnsigned(AVPRequiredAccessInfo, 0),
		diameter.OctetString(diameter.AVPClass, diameter.AVPFlagMandatory, 0, []byte("c")),
	)
	if _, err := ParseSessionTerminationRequest(kamailio); err != nil {
		t.Fatalf("AF extras: %v", err)
	}
}

func TestSessionTerminationAnswer(t *testing.T) {
	req := request(CommandSessionTermination, afEnvelope)

	ans, err := NewSessionTerminationAnswer(req, pcrfIdentity, SessionTerminationAnswer{})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := ParseSessionTerminationAnswer(roundTrip(t, ans)); err != nil || got.Result.Code != diameter.ResultSuccess {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestAbortSessionRoundTrip(t *testing.T) {
	for c := AbortBearerReleased; c <= maxAbortCause; c++ {
		req, err := NewAbortSessionRequest(pcrfEnvelope, AbortSessionRequest{Cause: c})
		if err != nil {
			t.Fatal(err)
		}

		got, err := ParseAbortSessionRequest(roundTrip(t, req))
		if err != nil || got.Cause != c {
			t.Fatalf("round trip of %s = %+v, %v", c, got, err)
		}
	}

	if _, err := NewAbortSessionRequest(afEnvelope, AbortSessionRequest{}); !errors.Is(err, ErrInvalidMessage) {
		t.Errorf("no destination host: err = %v", err)
	}

	if _, err := NewAbortSessionRequest(pcrfEnvelope, AbortSessionRequest{Cause: 6}); !errors.Is(err, ErrInvalidMessage) {
		t.Errorf("Abort-Cause 6: err = %v", err)
	}
}

func TestParseAbortSessionRequestErrors(t *testing.T) {
	base := request(CommandAbortSession, pcrfEnvelope, vendorUnsigned(AVPAbortCause, uint32(AbortBearerReleased)))

	for name, tc := range map[string]struct {
		msg    *diameter.Message
		result uint32
	}{
		"no Abort-Cause":      {without(base, AVPAbortCause, tgpp.VendorID), diameter.ResultMissingAVP},
		"no Destination-Host": {without(base, diameter.AVPDestinationHost, 0), diameter.ResultMissingAVP},
		"Abort-Cause":         {with(without(base, AVPAbortCause, tgpp.VendorID), vendorUnsigned(AVPAbortCause, 6)), diameter.ResultInvalidAVPValue},
		"short Abort-Cause":   {with(without(base, AVPAbortCause, tgpp.VendorID), vendorOctets(AVPAbortCause, []byte{0})), diameter.ResultInvalidAVPValue},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseAbortSessionRequest(tc.msg)
			if got := avpError(t, err).ResultCode; got != tc.result {
				t.Fatalf("result = %d, want %d (%v)", got, tc.result, err)
			}
		})
	}
}

func TestAbortSessionAnswer(t *testing.T) {
	req := request(CommandAbortSession, pcrfEnvelope)

	ans, err := NewAbortSessionAnswer(req, afIdentity, AbortSessionAnswer{})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := ParseAbortSessionAnswer(roundTrip(t, ans)); err != nil || got.Result.Code != diameter.ResultSuccess {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	unknown := NewAnswer(req, afIdentity, tgpp.Result{Code: diameter.ResultUnknownSessionID})

	var re *ResultError
	if _, err := ParseAbortSessionAnswer(unknown); !errors.As(err, &re) || re.Code != diameter.ResultUnknownSessionID {
		t.Fatalf("unknown session = %v", err)
	}
}
