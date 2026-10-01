// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/diameter"
)

var testIdentity = diameter.Identity{OriginHost: "hss.example.org", OriginRealm: "example.org"}

func TestEnvelope(t *testing.T) {
	avps := Envelope{
		SessionID:        "smsc.example.org;1;1",
		Origin:           diameter.Identity{OriginHost: "smsc.example.org", OriginRealm: "example.org"},
		DestinationHost:  "hss.example.org",
		DestinationRealm: "example.org",
	}.AVPs()

	want := []uint32{
		diameter.AVPSessionID, diameter.AVPAuthSessionState, diameter.AVPOriginHost,
		diameter.AVPOriginRealm, diameter.AVPDestinationHost, diameter.AVPDestinationRealm,
	}

	if len(avps) != len(want) {
		t.Fatalf("AVPs = %+v", avps)
	}

	for i, code := range want {
		if avps[i].Code != code || avps[i].Flags&diameter.AVPFlagMandatory == 0 {
			t.Errorf("AVP %d = %+v, want code %d", i, avps[i], code)
		}
	}
}

func TestEnvelopeWithoutDestinationHost(t *testing.T) {
	avps := Envelope{
		SessionID:        "smsc.example.org;1;1",
		Origin:           diameter.Identity{OriginHost: "smsc.example.org", OriginRealm: "example.org"},
		DestinationRealm: "example.org",
	}.AVPs()

	if _, ok := diameter.Find(avps, diameter.AVPDestinationHost, 0); ok {
		t.Fatal("Destination-Host sent without a host")
	}

	if realm, ok := diameter.Find(avps, diameter.AVPDestinationRealm, 0); !ok || realm.UTF8String() != "example.org" {
		t.Fatalf("Destination-Realm = %+v", realm)
	}
}

func TestParseEnvelope(t *testing.T) {
	env := Envelope{
		SessionID:        "smsc.example.org;1;1",
		Origin:           diameter.Identity{OriginHost: "smsc.example.org", OriginRealm: "example.org"},
		DestinationHost:  "hss.example.org",
		DestinationRealm: "epc.example.org",
	}

	got := ParseEnvelope(&diameter.Message{AVPs: env.AVPs()})
	if got.SessionID != env.SessionID || got.Origin.OriginHost != env.Origin.OriginHost || got.Origin.OriginRealm != env.Origin.OriginRealm ||
		got.DestinationHost != env.DestinationHost || got.DestinationRealm != env.DestinationRealm {
		t.Fatalf("ParseEnvelope = %+v", got)
	}
}

func TestAnswersCarryAuthSessionState(t *testing.T) {
	req := &diameter.Message{Flags: diameter.FlagRequest | diameter.FlagProxiable}

	for name, ans := range map[string]*diameter.Message{
		"result":       NewAnswer(req, testIdentity, diameter.ResultSuccess),
		"experimental": NewExperimentalAnswer(req, testIdentity, ResultErrorAbsentUser),
		"result value": NewResultAnswer(req, testIdentity, Experimental(ResultErrorUserUnknown)),
		"AVP error":    NewErrorAnswer(req, testIdentity, InvalidAVP(diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "x"))),
	} {
		state, ok := ans.Find(diameter.AVPAuthSessionState, 0)
		if v, _ := state.Unsigned32(); !ok || v != diameter.AuthSessionStateNoStateMaintained {
			t.Errorf("%s: Auth-Session-State = %+v", name, state)
		}
	}
}

func TestNewErrorAnswer(t *testing.T) {
	req := &diameter.Message{Flags: diameter.FlagRequest}
	offending := diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "x")

	ans := NewErrorAnswer(req, testIdentity, InvalidAVP(offending))

	if r, err := ParseResult(ans); err != nil || r.Code != diameter.ResultInvalidAVPValue {
		t.Fatalf("result = %+v, %v", r, err)
	}

	failed, ok := ans.Find(diameter.AVPFailedAVP, 0)
	if !ok {
		t.Fatal("no Failed-AVP")
	}

	inner, err := failed.Grouped()
	if err != nil || len(inner) != 1 || inner[0].Code != diameter.AVPUserName {
		t.Fatalf("Failed-AVP = %+v, %v", inner, err)
	}

	missing := NewErrorAnswer(req, testIdentity, MissingAVP(AVPSCAddress, VendorID))
	if r, _ := ParseResult(missing); r.Code != diameter.ResultMissingAVP {
		t.Fatalf("missing AVP result = %+v", r)
	}

	if r, _ := ParseResult(NewErrorAnswer(req, testIdentity, errors.New("boom"))); r.Code != diameter.ResultUnableToComply {
		t.Fatalf("generic error result = %+v", r)
	}
}

func TestE164(t *testing.T) {
	for _, n := range []string{"1", "15551230002", "123456789012345"} {
		b, err := EncodeE164(n)
		if err != nil {
			t.Fatalf("EncodeE164(%q): %v", n, err)
		}

		if got, err := DecodeE164(b); err != nil || got != n {
			t.Fatalf("DecodeE164 = %q, %v", got, err)
		}
	}

	for _, n := range []string{"", "1234567890123456", "+1555", "12*"} {
		if _, err := EncodeE164(n); !errors.Is(err, ErrInvalidNumber) {
			t.Errorf("EncodeE164(%q) = %v", n, err)
		}
	}

	if _, err := DecodeE164([]byte{0xba}); !errors.Is(err, ErrInvalidNumber) {
		t.Errorf("DecodeE164 of *# = %v", err)
	}
}

func TestValidIMSI(t *testing.T) {
	for imsi, want := range map[string]bool{
		"001010000000001":  true,
		"001010":           true,
		"00101":            false,
		"0010100000000011": false,
		"00101a000000001":  false,
		"":                 false,
	} {
		if got := ValidIMSI(imsi); got != want {
			t.Errorf("ValidIMSI(%q) = %v", imsi, got)
		}
	}
}

type testResultError struct {
	Result
}

func (e *testResultError) Error() string { return e.String() }

func TestIsExperimental(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &testResultError{Result: Experimental(ResultErrorAbsentUser)})

	if !IsExperimental(err, ResultErrorAbsentUser) || IsExperimental(err, ResultErrorUserUnknown) {
		t.Fatal("IsExperimental did not see the wrapped result")
	}

	if r, ok := ResultOf(err); !ok || r != Experimental(ResultErrorAbsentUser) {
		t.Fatalf("ResultOf = %+v, %v", r, ok)
	}

	if IsExperimental(errors.New("plain"), ResultErrorAbsentUser) {
		t.Fatal("plain error reported as experimental")
	}

	if (Result{Code: 2002}).Success() != true || Experimental(ResultErrorAbsentUser).Success() {
		t.Fatal("Success classification")
	}
}

func TestNewErrorAnswerKeepsResultErrors(t *testing.T) {
	req := &diameter.Message{Flags: diameter.FlagRequest}
	err := fmt.Errorf("wrapped: %w", &testResultError{Result: Experimental(ResultErrorUserUnknown)})

	if r, parseErr := ParseResult(NewErrorAnswer(req, testIdentity, err)); parseErr != nil || !r.IsExperimental(ResultErrorUserUnknown) {
		t.Fatalf("result = %+v, %v", r, parseErr)
	}
}

func TestAbsentUserDiagnosticString(t *testing.T) {
	for d, want := range map[AbsentUserDiagnostic]string{
		AbsentUserNoPagingResponseMSC:        "NO_PAGING_RESPONSE_VIA_THE_MSC",
		AbsentUserIMSIDetached:               "IMSI_DETACHED",
		AbsentUserRoamingRestriction:         "ROAMING_RESTRICTION",
		AbsentUserDeregisteredNonGPRS:        "DEREGISTERED_IN_THE_HLR_FOR_NON_GPRS",
		AbsentUserPurgedNonGPRS:              "MS_PURGED_FOR_NON_GPRS",
		AbsentUserNoPagingResponseSGSN:       "NO_PAGING_RESPONSE_VIA_THE_SGSN",
		AbsentUserGPRSDetached:               "GPRS_DETACHED",
		AbsentUserDeregisteredGPRS:           "DEREGISTERED_IN_THE_HLR_FOR_GPRS",
		AbsentUserPurgedGPRS:                 "MS_PURGED_FOR_GPRS",
		AbsentUserUnidentifiedSubscriberMSC:  "UNIDENTIFIED_SUBSCRIBER_VIA_THE_MSC",
		AbsentUserUnidentifiedSubscriberSGSN: "UNIDENTIFIED_SUBSCRIBER_VIA_THE_SGSN",
		AbsentUserDeregisteredIMS:            "DEREGISTERED_IN_THE_HSS_HLR_FOR_IMS",
		AbsentUserNoResponseIPSMGW:           "NO_RESPONSE_VIA_THE_IP_SM_GW",
		AbsentUserTemporarilyUnavailable:     "THE_MS_IS_TEMPORARILY_UNAVAILABLE",
		14:                                   "AbsentUserDiagnostic(14)",
	} {
		if got := d.String(); got != want {
			t.Errorf("AbsentUserDiagnostic(%d).String() = %q, want %q", uint32(d), got, want)
		}
	}
}
