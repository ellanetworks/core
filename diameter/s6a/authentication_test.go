// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func testEUTRANVector(item uint32) EUTRANVector {
	return EUTRANVector{
		ItemNumber: item,
		RAND:       bytes.Repeat([]byte{byte(item)}, 16),
		XRES:       bytes.Repeat([]byte{0x22}, 8),
		AUTN:       bytes.Repeat([]byte{0x33}, 16),
		KASME:      bytes.Repeat([]byte{0x44}, 32),
	}
}

func testAIR() AuthenticationInformationRequest {
	return AuthenticationInformationRequest{
		IMSI: testIMSI,
		EUTRAN: &RequestedAuthenticationInfo{
			NumberOfVectors:            1,
			ImmediateResponsePreferred: true,
			Resync:                     &Resync{RAND: bytes.Repeat([]byte{1}, 16), AUTS: bytes.Repeat([]byte{2}, 14)},
		},
		VisitedPLMN: visitedPLMN,
		Flags:       AIRFlagSendUEUsageType,
	}
}

func TestAuthenticationInformationRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]AuthenticationInformationRequest{
		"resync": testAIR(),
		"plain": {
			IMSI: testIMSI, EUTRAN: &RequestedAuthenticationInfo{NumberOfVectors: 1}, VisitedPLMN: tgpp.PLMN{MCC: "001", MNC: "001"},
		},
		"UTRAN-GERAN": {
			IMSI: testIMSI, UTRANGERAN: &RequestedAuthenticationInfo{NumberOfVectors: 3}, VisitedPLMN: visitedPLMN,
			Features: Features{List2: FeatureSMSInSGSN},
		},
	} {
		t.Run(name, func(t *testing.T) {
			req := must[*diameter.Message](t)(NewAuthenticationInformationRequest(mmeEnvelope, r))
			requireNoVendorSpecificApplicationID(t, req)

			got, err := ParseAuthenticationInformationRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}

	req := must[*diameter.Message](t)(NewAuthenticationInformationRequest(mmeEnvelope, testAIR()))
	info, _ := req.Find(AVPRequestedEUTRANAuthenticationInfo, tgpp.VendorID)
	resync, _ := diameter.Find(must[[]diameter.AVP](t)(info.Grouped()), AVPReSynchronizationInfo, tgpp.VendorID)

	if want := append(bytes.Repeat([]byte{1}, 16), bytes.Repeat([]byte{2}, 14)...); !bytes.Equal(resync.Data, want) {
		t.Fatalf("Re-Synchronization-Info = %x", resync.Data)
	}
}

func TestAuthenticationInformationRequestValidation(t *testing.T) {
	for name, mutate := range map[string]func(*AuthenticationInformationRequest){
		"short AUTS":     func(r *AuthenticationInformationRequest) { r.EUTRAN.Resync.AUTS = r.EUTRAN.Resync.AUTS[:13] },
		"short RAND":     func(r *AuthenticationInformationRequest) { r.EUTRAN.Resync.RAND = nil },
		"bad PLMN":       func(r *AuthenticationInformationRequest) { r.VisitedPLMN = tgpp.PLMN{MCC: "001"} },
		"bad IMSI":       func(r *AuthenticationInformationRequest) { r.IMSI = "" },
		"undefined flag": func(r *AuthenticationInformationRequest) { r.Flags = 1 << 3 },
	} {
		t.Run(name, func(t *testing.T) {
			r := testAIR()
			mutate(&r)

			if _, err := NewAuthenticationInformationRequest(mmeEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestParseAuthenticationInformationRequestErrors(t *testing.T) {
	base := must[*diameter.Message](t)(NewAuthenticationInformationRequest(mmeEnvelope, testAIR()))

	tests := map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no Visited-PLMN-Id": {without(base, AVPVisitedPLMNID, tgpp.VendorID), diameter.ResultMissingAVP},
		"no User-Name":       {without(base, diameter.AVPUserName, 0), diameter.ResultMissingAVP},
		"no Destination":     {without(base, diameter.AVPDestinationRealm, 0), diameter.ResultMissingAVP},
		"short resync": {replace(base, vendorGroup(AVPRequestedEUTRANAuthenticationInfo,
			vendorOctets(AVPReSynchronizationInfo, make([]byte, 29)))), diameter.ResultInvalidAVPValue},
		"short vectors": {replace(base, vendorGroup(AVPRequestedEUTRANAuthenticationInfo,
			vendorOctets(AVPNumberOfRequestedVectors, []byte{1}))), diameter.ResultInvalidAVPLength},
		"twice EUTRAN": {with(base, vendorGroup(AVPRequestedEUTRANAuthenticationInfo)), diameter.ResultAVPOccursTooManyTimes},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseAuthenticationInformationRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d (%v)", code, tt.want, err)
			}
		})
	}
}

func TestAuthenticationInformationAnswerRoundTrip(t *testing.T) {
	req := must[*diameter.Message](t)(NewAuthenticationInformationRequest(mmeEnvelope, testAIR()))

	a := AuthenticationInformation{
		Features: Features{List1: FeatureODBAllAPN},
		EUTRAN:   []EUTRANVector{testEUTRANVector(1), testEUTRANVector(2)},
		UTRAN: []UTRANVector{{
			RAND: make([]byte, 16), XRES: make([]byte, 16), AUTN: make([]byte, 16), CK: make([]byte, 16), IK: make([]byte, 16),
		}},
		GERAN:       []GERANVector{{ItemNumber: 1, RAND: make([]byte, 16), SRES: make([]byte, 4), Kc: make([]byte, 8)}},
		UEUsageType: ptr(uint32(128)),
	}

	ans := must[*diameter.Message](t)(NewAuthenticationInformationAnswer(req, hssIdentity, a))
	requireNoVendorSpecificApplicationID(t, ans)

	got, err := ParseAuthenticationInformationAnswer(roundTrip(t, ans))
	if err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestAuthenticationInformationAnswerValidation(t *testing.T) {
	req := request(CommandAuthenticationInformation, mmeEnvelope)

	for name, a := range map[string]AuthenticationInformation{
		"no vectors":  {},
		"short KASME": {EUTRAN: []EUTRANVector{{RAND: make([]byte, 16), XRES: make([]byte, 8), AUTN: make([]byte, 16), KASME: make([]byte, 16)}}},
		"long XRES":   {EUTRAN: []EUTRANVector{{RAND: make([]byte, 16), XRES: make([]byte, 17), AUTN: make([]byte, 16), KASME: make([]byte, 32)}}},
		"short SRES":  {GERAN: []GERANVector{{RAND: make([]byte, 16), SRES: make([]byte, 3), Kc: make([]byte, 8)}}},
		"UE usage":    {EUTRAN: []EUTRANVector{testEUTRANVector(1)}, UEUsageType: ptr(uint32(300))},
	} {
		if _, err := NewAuthenticationInformationAnswer(req, hssIdentity, a); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseAuthenticationInformationAnswerErrors(t *testing.T) {
	req := request(CommandAuthenticationInformation, mmeEnvelope)

	unavailable := tgpp.NewExperimentalAnswer(req, hssIdentity, tgpp.ResultAuthenticationDataUnavailable)

	var re *ResultError
	if _, err := ParseAuthenticationInformationAnswer(roundTrip(t, unavailable)); !errors.As(err, &re) ||
		!re.IsExperimental(tgpp.ResultAuthenticationDataUnavailable) || !re.Transient() {
		t.Fatalf("4181: err = %v", err)
	}

	vector := must[diameter.AVP](t)(testEUTRANVector(1).avp())
	inner := must[[]diameter.AVP](t)(vector.Grouped())

	for name, avps := range map[string][]diameter.AVP{
		"no Authentication-Info": nil,
		"empty info":             {vendorGroup(AVPAuthenticationInfo)},
		"bad info":               {vendorOctets(AVPAuthenticationInfo, []byte{1})},
		"no KASME":               {vendorGroup(AVPAuthenticationInfo, vendorGroup(AVPEUTRANVector, inner[:4]...))},
		"short RAND":             {vendorGroup(AVPAuthenticationInfo, vendorGroup(AVPEUTRANVector, append(inner[2:], vendorOctets(AVPRAND, make([]byte, 15)))...))},
	} {
		t.Run(name, func(t *testing.T) {
			ans := NewAnswer(req, hssIdentity, tgpp.Result{Code: diameter.ResultSuccess}, Features{})
			ans.AVPs = append(ans.AVPs, avps...)

			if _, err := ParseAuthenticationInformationAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
