// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sh

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	testPublic     = "sip:001010000000001@ims.mnc001.mcc001.3gppnetwork.org"
	testTel        = "tel:+15551230002"
	testMSISDN     = "15551230002"
	testExternal   = "device1@ext.example.org"
	testPrivate    = "001010000000001@ims.mnc001.mcc001.3gppnetwork.org"
	testWildcarded = "sip:chatroom-!.*!@ims.mnc001.mcc001.3gppnetwork.org"
	testRealm      = "ims.mnc001.mcc001.3gppnetwork.org"
)

var (
	asIdentity  = diameter.Identity{OriginHost: "ipsmgw.ims.mnc001.mcc001.3gppnetwork.org", OriginRealm: testRealm}
	hssIdentity = diameter.Identity{OriginHost: "hss.ims.mnc001.mcc001.3gppnetwork.org", OriginRealm: testRealm}
	asEnvelope  = tgpp.Envelope{
		SessionID:        "ipsmgw.ims.mnc001.mcc001.3gppnetwork.org;1;1",
		Origin:           asIdentity,
		DestinationRealm: testRealm,
	}
)

var mandatoryBit = map[uint32]bool{
	AVPPublicIdentity:           true,
	AVPWildcardedPublicIdentity: false,
	AVPWildcardedIMPU:           false,
	AVPUserIdentity:             true,
	AVPMSISDN:                   true,
	AVPUserData:                 true,
	AVPDataReference:            true,
	AVPServiceIndication:        true,
	AVPRepositoryDataID:         false,
	AVPSequenceNumber:           false,
	AVPExternalIdentifier:       true,
	tgpp.AVPFeatureListID:       false,
	tgpp.AVPFeatureList:         false,
}

var groupedCodes = map[uint32]bool{
	AVPUserIdentity:           true,
	AVPRepositoryDataID:       true,
	tgpp.AVPSupportedFeatures: true,
}

func roundTrip(t *testing.T, m *diameter.Message) *diameter.Message {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := diameter.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return out
}

func mustMessage(t testing.TB) func(*diameter.Message, error) *diameter.Message {
	return func(m *diameter.Message, err error) *diameter.Message {
		t.Helper()

		if err != nil {
			t.Fatal(err)
		}

		return m
	}
}

func avpResult(t *testing.T, err error) uint32 {
	t.Helper()

	var avpErr *diameter.AVPError
	if !errors.As(err, &avpErr) {
		t.Fatalf("err = %v, want an AVP error", err)
	}

	return avpErr.ResultCode
}

func profileUpdateRequest() *diameter.Message {
	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   CommandProfileUpdate,
		ApplicationID: ApplicationID,
		AVPs:          append(asEnvelope.AVPs(), vendorSpecificApplicationID()),
	}
}

func without(m *diameter.Message, code, vendorID uint32) *diameter.Message {
	out := *m
	out.AVPs = nil

	for _, a := range m.AVPs {
		if a.Code != code || a.VendorID != vendorID {
			out.AVPs = append(out.AVPs, a)
		}
	}

	return &out
}

func with(m *diameter.Message, avps ...diameter.AVP) *diameter.Message {
	out := *m
	out.AVPs = append(append([]diameter.AVP(nil), m.AVPs...), avps...)

	return &out
}

func requireVendorSpecificApplicationID(t *testing.T, m *diameter.Message) {
	t.Helper()

	vsai, ok := m.Find(diameter.AVPVendorSpecificApplicationID, 0)
	if !ok {
		t.Fatal("no Vendor-Specific-Application-Id")
	}

	inner, err := vsai.Grouped()
	if err != nil {
		t.Fatal(err)
	}

	app, ok := diameter.Find(inner, diameter.AVPAuthApplicationID, 0)
	if v, _ := app.Unsigned32(); !ok || v != ApplicationID {
		t.Fatalf("Auth-Application-Id = %d", v)
	}
}

func checkFlags(t *testing.T, avps []diameter.AVP) {
	t.Helper()

	for _, a := range avps {
		if a.VendorID != tgpp.VendorID {
			continue
		}

		mandatory, known := mandatoryBit[a.Code]

		switch {
		case a.Code == tgpp.AVPSupportedFeatures:
		case !known:
			t.Errorf("AVP %d not in the flag table", a.Code)
		case (a.Flags&diameter.AVPFlagMandatory != 0) != mandatory:
			t.Errorf("AVP %d M bit = %v, want %v", a.Code, a.Flags&diameter.AVPFlagMandatory != 0, mandatory)
		}

		if groupedCodes[a.Code] {
			inner, err := a.Grouped()
			if err != nil {
				t.Fatal(err)
			}

			checkFlags(t, inner)
		}
	}
}

func TestEnumNames(t *testing.T) {
	for _, tt := range []struct {
		value fmt.Stringer
		want  string
	}{
		{DataRepositoryData, "RepositoryData"},
		{DataSCSCFName, "S-CSCFName"},
		{DataSMSRegistrationInfo, "SMSRegistrationInfo"},
		{DataASRegistrationInfo, "ASRegistrationInfo"},
		{DataReference(1), "DataReference(1)"},
		{DataReference(20), "DataReference(20)"},
		{DataReference(37), "DataReference(37)"},
		{Features(0), "0"},
		{FeatureNotifEff | FeatureUpdateEffEnhance, "Notif-Eff|Update-Eff-Enhance"},
		{FeatureAdditionalMSISDN | Features(0x30), "Additional-MSISDN|0x30"},
	} {
		if got := tt.value.String(); got != tt.want {
			t.Errorf("%#v.String() = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestDataReferenceValid(t *testing.T) {
	for ref, want := range map[DataReference]bool{
		DataRepositoryData: true, DataIMSPublicIdentity: true, DataDSAI: true, DataServiceLevelTraceInfo: true,
		DataSMSRegistrationInfo: true, DataASRegistrationInfo: true,
		1: false, 9: false, 20: false, 37: false,
	} {
		if got := ref.Valid(); got != want {
			t.Errorf("%d.Valid() = %v", ref, got)
		}
	}
}

func TestUserIdentityRoundTrip(t *testing.T) {
	for name, u := range map[string]UserIdentity{
		"SIP URI":             {PublicIdentity: testPublic},
		"tel URI":             {PublicIdentity: testTel},
		"MSISDN":              {MSISDN: testMSISDN},
		"external identifier": {ExternalIdentifier: testExternal},
	} {
		t.Run(name, func(t *testing.T) {
			a, err := userIdentityAVP(u)
			if err != nil {
				t.Fatal(err)
			}

			checkFlags(t, []diameter.AVP{a})

			got, err := parseUserIdentity(a)
			if err != nil || got != u {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestUserIdentityValidation(t *testing.T) {
	for name, u := range map[string]UserIdentity{
		"none":                         {},
		"two":                          {PublicIdentity: testPublic, MSISDN: testMSISDN},
		"public identity not a URI":    {PublicIdentity: "alice"},
		"MSISDN not digits":            {MSISDN: "+15551230002"},
		"MSISDN too long":              {MSISDN: "1234567890123456"},
		"external identifier no realm": {ExternalIdentifier: "device1"},
		"external identifier two @":    {ExternalIdentifier: "a@b@c"},
	} {
		if _, err := userIdentityAVP(u); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseUserIdentityErrors(t *testing.T) {
	public := diameter.UTF8String(AVPPublicIdentity, diameter.AVPFlagMandatory, tgpp.VendorID, testPublic)
	msisdn := diameter.OctetString(AVPMSISDN, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{0x51})

	for name, a := range map[string]diameter.AVP{
		"not grouped": diameter.OctetString(AVPUserIdentity, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{1}),
		"empty":       diameter.Grouped(AVPUserIdentity, diameter.AVPFlagMandatory, tgpp.VendorID),
		"two":         diameter.Grouped(AVPUserIdentity, diameter.AVPFlagMandatory, tgpp.VendorID, public, msisdn),
		"bad public": diameter.Grouped(AVPUserIdentity, diameter.AVPFlagMandatory, tgpp.VendorID,
			diameter.UTF8String(AVPPublicIdentity, diameter.AVPFlagMandatory, tgpp.VendorID, "alice")),
		"bad MSISDN": diameter.Grouped(AVPUserIdentity, diameter.AVPFlagMandatory, tgpp.VendorID,
			diameter.OctetString(AVPMSISDN, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{0xab})),
		"bad external identifier": diameter.Grouped(AVPUserIdentity, diameter.AVPFlagMandatory, tgpp.VendorID,
			diameter.UTF8String(AVPExternalIdentifier, diameter.AVPFlagMandatory, tgpp.VendorID, "device1")),
	} {
		if _, err := parseUserIdentity(a); avpResult(t, err) != diameter.ResultInvalidAVPValue {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestRequiresUnsupportedFeatures(t *testing.T) {
	req := mustMessage(t)(NewProfileUpdateRequest(asEnvelope, ProfileUpdateRequest{
		User: UserIdentity{PublicIdentity: testPublic}, DataReferences: []DataReference{DataSMSRegistrationInfo},
		UserData: []byte("<Sh-Data/>"), Features: FeatureUpdateEff, FeaturesRequired: true,
	}))

	if !RequiresUnsupportedFeatures(req, FeatureNotifEff) {
		t.Error("Update-Eff required but not supported")
	}

	if RequiresUnsupportedFeatures(req, FeatureNotifEff|FeatureUpdateEff) {
		t.Error("Update-Eff required and supported")
	}
}

func TestShResultNames(t *testing.T) {
	for code, want := range map[uint32]string{
		tgpp.ResultUserDataNotAvailable:          "DIAMETER_USER_DATA_NOT_AVAILABLE",
		tgpp.ResultPriorUpdateInProgress:         "DIAMETER_PRIOR_UPDATE_IN_PROGRESS",
		tgpp.ResultErrorUserDataCannotBeModified: "DIAMETER_ERROR_USER_DATA_CANNOT_BE_MODIFIED",
		tgpp.ResultErrorDSAINotAvailable:         "DIAMETER_ERROR_DSAI_NOT_AVAILABLE",
	} {
		if got := tgpp.Experimental(code).Name(); got != want {
			t.Errorf("%d name = %q, want %q", code, got, want)
		}
	}
}
