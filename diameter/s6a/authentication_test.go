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

func testAIA() AuthenticationInformation {
	return AuthenticationInformation{
		HSS:      hssIdentity,
		Features: Features{List1: FeatureODBAllAPN},
		EUTRAN:   []EUTRANVector{testEUTRANVector(1), testEUTRANVector(2)},
		UTRAN: []UTRANVector{{
			ItemNumber: 3, RAND: make([]byte, 16), XRES: make([]byte, 16), AUTN: make([]byte, 16), CK: make([]byte, 16), IK: make([]byte, 16),
		}},
		GERAN:       []GERANVector{{ItemNumber: 4, RAND: make([]byte, 16), SRES: make([]byte, 4), Kc: make([]byte, 8)}},
		UEUsageType: ptr(UEUsageType(128)),
	}
}

func TestEUTRANVectorEncoding(t *testing.T) {
	v := EUTRANVector{
		ItemNumber: 1,
		RAND:       bytes.Repeat([]byte{0x11}, 16),
		XRES:       bytes.Repeat([]byte{0x22}, 8),
		AUTN:       bytes.Repeat([]byte{0x33}, 16),
		KASME:      bytes.Repeat([]byte{0x44}, 32),
	}

	want := "00000586c0000094000028af" +
		"0000058bc0000010000028af00000001" +
		"000005a7c000001c000028af" + "11111111111111111111111111111111" +
		"000005a8c0000014000028af" + "2222222222222222" +
		"000005a9c000001c000028af" + "33333333333333333333333333333333" +
		"000005aac000002c000028af" + "4444444444444444444444444444444444444444444444444444444444444444"

	a := must[diameter.AVP](t)(encodeVector(AVPEUTRANVector, v.ItemNumber, v.fields()))
	if got := encodeAVP(t, a); !bytes.Equal(got, mustHex(t, want)) {
		t.Fatalf("E-UTRAN-Vector =\n%x\nwant\n%s", got, want)
	}

	var back EUTRANVector
	if err := decodeVector(a, &back.ItemNumber, back.fields()); err != nil || !reflect.DeepEqual(back, v) {
		t.Fatalf("parse = %+v, %v", back, err)
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
		"combined": {
			IMSI: testIMSI, VisitedPLMN: visitedPLMN,
			EUTRAN:     &RequestedAuthenticationInfo{NumberOfVectors: 1, Resync: &Resync{RAND: make([]byte, 16), AUTS: make([]byte, 14)}},
			UTRANGERAN: &RequestedAuthenticationInfo{ImmediateResponsePreferred: true},
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
		"short AUTS":        func(r *AuthenticationInformationRequest) { r.EUTRAN.Resync.AUTS = r.EUTRAN.Resync.AUTS[:13] },
		"short RAND":        func(r *AuthenticationInformationRequest) { r.EUTRAN.Resync.RAND = nil },
		"bad PLMN":          func(r *AuthenticationInformationRequest) { r.VisitedPLMN = tgpp.PLMN{MCC: "001"} },
		"bad IMSI":          func(r *AuthenticationInformationRequest) { r.IMSI = "" },
		"undefined flag":    func(r *AuthenticationInformationRequest) { r.Flags = 1 << 3 },
		"nothing requested": func(r *AuthenticationInformationRequest) { r.EUTRAN = nil },
		"resync twice": func(r *AuthenticationInformationRequest) {
			r.UTRANGERAN = &RequestedAuthenticationInfo{Resync: r.EUTRAN.Resync}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := testAIR()
			mutate(&r)

			if _, err := NewAuthenticationInformationRequest(mmeEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	if _, err := NewAuthenticationInformationRequest(tgpp.Envelope{}, testAIR()); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("empty envelope: err = %v", err)
	}
}

func TestParseAuthenticationInformationRequestErrors(t *testing.T) {
	base := must[*diameter.Message](t)(NewAuthenticationInformationRequest(mmeEnvelope, testAIR()))
	resync := vendorOctets(AVPReSynchronizationInfo, make([]byte, 30))

	tests := map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no Visited-PLMN-Id": {without(base, AVPVisitedPLMNID, tgpp.VendorID), diameter.ResultMissingAVP},
		"no User-Name":       {without(base, diameter.AVPUserName, 0), diameter.ResultMissingAVP},
		"no Destination":     {without(base, diameter.AVPDestinationRealm, 0), diameter.ResultMissingAVP},
		"nothing requested":  {without(base, AVPRequestedEUTRANAuthenticationInfo, tgpp.VendorID), diameter.ResultMissingAVP},
		"bad IMSI":           {replace(base, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "12")), diameter.ResultInvalidAVPValue},
		"bad PLMN":           {replace(base, vendorOctets(AVPVisitedPLMNID, []byte{0xff, 0xff, 0xff})), diameter.ResultInvalidAVPValue},
		"short resync": {replace(base, vendorGroup(AVPRequestedEUTRANAuthenticationInfo,
			vendorOctets(AVPReSynchronizationInfo, make([]byte, 29)))), diameter.ResultInvalidAVPValue},
		"short vectors": {replace(base, vendorGroup(AVPRequestedEUTRANAuthenticationInfo,
			vendorOctets(AVPNumberOfRequestedVectors, []byte{1}))), diameter.ResultInvalidAVPLength},
		"unknown M-bit inside": {replace(base, vendorGroup(AVPRequestedEUTRANAuthenticationInfo, vendorU32(4242, 1))), diameter.ResultAVPUnsupported},
		"twice EUTRAN":         {with(base, vendorGroup(AVPRequestedEUTRANAuthenticationInfo)), diameter.ResultAVPOccursTooManyTimes},
		"short AIR-Flags":      {replace(base, vendorOctets(AVPAIRFlags, []byte{1})), diameter.ResultInvalidAVPLength},
		"resync in both": {with(replace(base, vendorGroup(AVPRequestedEUTRANAuthenticationInfo, resync)),
			vendorGroup(AVPRequestedUTRANGERANAuthenticationInfo, resync)), diameter.ResultUnableToComply},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseAuthenticationInformationRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d (%v)", code, tt.want, err)
			}
		})
	}

	_, err := ParseAuthenticationInformationRequest(with(base, vendorGroup(AVPRequestedUTRANGERANAuthenticationInfo, vendorU32(4242, 1))))

	failed := avpError(t, err).AVPs[0]
	if failed.Code != AVPRequestedUTRANGERANAuthenticationInfo {
		t.Fatalf("Failed-AVP = %+v", failed)
	}

	if inner := must[[]diameter.AVP](t)(failed.Grouped()); len(inner) != 1 || inner[0].Code != 4242 {
		t.Fatalf("nested Failed-AVP = %+v", inner)
	}

	ans := NewErrorAnswer(base, hssIdentity, &diameter.AVPError{ResultCode: diameter.ResultUnableToComply}, Features{})

	var re *ResultError
	if _, err := ParseAuthenticationInformationAnswer(ans); !errors.As(err, &re) || re.Code != diameter.ResultUnableToComply {
		t.Fatalf("resync answer err = %v", err)
	}
}

func TestAuthenticationInformationAnswerRoundTrip(t *testing.T) {
	req := must[*diameter.Message](t)(NewAuthenticationInformationRequest(mmeEnvelope, testAIR()))

	for name, a := range map[string]AuthenticationInformation{
		"all vectors":   testAIA(),
		"single vector": {HSS: hssIdentity, EUTRAN: []EUTRANVector{testEUTRANVector(0)}},
		"UE usage type only": {
			HSS: hssIdentity, Features: Features{List2: FeatureDedicatedCoreNetworks}, UEUsageType: ptr(UEUsageType(7)),
		},
	} {
		t.Run(name, func(t *testing.T) {
			ans := must[*diameter.Message](t)(NewAuthenticationInformationAnswer(req, hssIdentity, a))
			requireNoVendorSpecificApplicationID(t, ans)

			if _, ok := ans.Find(AVPAuthenticationInfo, tgpp.VendorID); ok != (a.vectors() > 0) {
				t.Fatalf("Authentication-Info present = %v", ok)
			}

			got, err := ParseAuthenticationInformationAnswer(roundTrip(t, ans))
			if err != nil || !reflect.DeepEqual(got, a) {
				t.Fatalf("round trip =\n%+v, %v\nwant\n%+v", got, err, a)
			}
		})
	}
}

func TestAuthenticationInformationAnswerWithoutVectors(t *testing.T) {
	req := request(CommandAuthenticationInformation, mmeEnvelope)

	for name, avps := range map[string][]diameter.AVP{
		"no Authentication-Info":    {optionalU32(AVPUEUsageType, 3)},
		"empty Authentication-Info": {vendorGroup(AVPAuthenticationInfo), optionalU32(AVPUEUsageType, 3)},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseAuthenticationInformationAnswer(success(req, avps...))
			if err != nil || got.vectors() != 0 || got.UEUsageType == nil || *got.UEUsageType != 3 {
				t.Fatalf("parsed = %+v, %v", got, err)
			}
		})
	}
}

func TestAuthenticationDataUnavailableCarriesUEUsageType(t *testing.T) {
	air := must[*diameter.Message](t)(NewAuthenticationInformationRequest(mmeEnvelope, testAIR()))
	ulr := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, testULR()))
	unavailable := &ResultError{Result: tgpp.Experimental(tgpp.ResultAuthenticationDataUnavailable), UEUsageType: ptr(UEUsageType(130))}

	ans := roundTrip(t, NewErrorAnswer(air, hssIdentity, unavailable, Features{}))

	var re *ResultError
	if _, err := ParseAuthenticationInformationAnswer(ans); !errors.As(err, &re) || !re.Transient() ||
		re.UEUsageType == nil || *re.UEUsageType != 130 {
		t.Fatalf("err = %v (%+v)", err, re)
	}

	for name, tt := range map[string]struct {
		req *diameter.Message
		err *ResultError
	}{
		"ULA":          {ulr, unavailable},
		"user unknown": {air, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown), UEUsageType: ptr(UEUsageType(1))}},
	} {
		if _, ok := NewErrorAnswer(tt.req, hssIdentity, tt.err, Features{}).Find(AVPUEUsageType, tgpp.VendorID); ok {
			t.Errorf("%s carries UE-Usage-Type", name)
		}
	}
}

func TestAuthenticationInformationAnswerValidation(t *testing.T) {
	req := request(CommandAuthenticationInformation, mmeEnvelope)
	v := testEUTRANVector(0)

	for name, a := range map[string]AuthenticationInformation{
		"nothing":     {},
		"short KASME": {EUTRAN: []EUTRANVector{{RAND: make([]byte, 16), XRES: make([]byte, 8), AUTN: make([]byte, 16), KASME: make([]byte, 16)}}},
		"long XRES":   {EUTRAN: []EUTRANVector{{RAND: make([]byte, 16), XRES: make([]byte, 17), AUTN: make([]byte, 16), KASME: make([]byte, 32)}}},
		"short SRES":  {GERAN: []GERANVector{{RAND: make([]byte, 16), SRES: make([]byte, 3), Kc: make([]byte, 8)}}},
		"short CK":    {UTRAN: []UTRANVector{{RAND: make([]byte, 16), XRES: make([]byte, 8), AUTN: make([]byte, 16), CK: make([]byte, 15), IK: make([]byte, 16)}}},
		"unnumbered":  {EUTRAN: []EUTRANVector{v, testEUTRANVector(2)}},
		"unnumbered mixed": {
			EUTRAN: []EUTRANVector{testEUTRANVector(1)},
			GERAN:  []GERANVector{{RAND: make([]byte, 16), SRES: make([]byte, 4), Kc: make([]byte, 8)}},
		},
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
		!re.IsExperimental(tgpp.ResultAuthenticationDataUnavailable) || !re.Transient() || re.UEUsageType != nil {
		t.Fatalf("4181: err = %v", err)
	}

	vector := must[diameter.AVP](t)(encodeVector(AVPEUTRANVector, 1, ptr(testEUTRANVector(1)).fields()))
	inner := must[[]diameter.AVP](t)(vector.Grouped())

	for name, avps := range map[string][]diameter.AVP{
		"no Authentication-Info": nil,
		"empty info":             {vendorGroup(AVPAuthenticationInfo)},
		"bad info":               {vendorOctets(AVPAuthenticationInfo, []byte{1})},
		"bad vector":             {vendorGroup(AVPAuthenticationInfo, vendorOctets(AVPUTRANVector, []byte{1}))},
		"bad item number":        {vendorGroup(AVPAuthenticationInfo, vendorGroup(AVPGERANVector, vendorOctets(AVPItemNumber, []byte{1})))},
		"no KASME":               {vendorGroup(AVPAuthenticationInfo, vendorGroup(AVPEUTRANVector, inner[:4]...))},
		"short RAND":             {vendorGroup(AVPAuthenticationInfo, vendorGroup(AVPEUTRANVector, append(inner[2:], vendorOctets(AVPRAND, make([]byte, 15)))...))},
		"UE usage 256":           {vendorGroup(AVPAuthenticationInfo, vector), optionalU32(AVPUEUsageType, 256)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseAuthenticationInformationAnswer(success(req, avps...)); !errors.Is(err, ErrMalformedAnswer) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	got, err := ParseAuthenticationInformationAnswer(success(req, vendorGroup(AVPAuthenticationInfo, vector, diameter.Unsigned32(9, 0, 0, 1))))
	if err != nil || len(got.EUTRAN) != 1 {
		t.Fatalf("foreign AVP in Authentication-Info: %+v, %v", got, err)
	}
}
