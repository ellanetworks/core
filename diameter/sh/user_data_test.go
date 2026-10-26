// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sh

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestShDataRoundTrip(t *testing.T) {
	supported, unsupported := true, false

	for name, d := range map[string]ShData{
		"empty":                {},
		"registration":         {SMSRegistration: &SMSRegistrationInfo{IPSMGWNumber: "15550000050"}},
		"deregistration":       {SMSRegistration: &SMSRegistrationInfo{}},
		"SC address":           {SMSRegistration: &SMSRegistrationInfo{IPSMGWNumber: "15550000050", SCAddress: "15550000000"}},
		"SBI supported":        {SMSRegistration: &SMSRegistrationInfo{IPSMGWNumber: "15550000050", SBISupported: &supported}},
		"SBI not supported":    {SMSRegistration: &SMSRegistrationInfo{IPSMGWNumber: "15550000050", SBISupported: &unsupported}},
		"SC address and SBI":   {SMSRegistration: &SMSRegistrationInfo{IPSMGWNumber: "15550000050", SCAddress: "15550000000", SBISupported: &supported}},
		"deregistration w/ SC": {SMSRegistration: &SMSRegistrationInfo{SCAddress: "15550000000"}},
	} {
		t.Run(name, func(t *testing.T) {
			b, err := MarshalShData(d)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseShData(b)
			if err != nil || !reflect.DeepEqual(got, d) {
				t.Fatalf("round trip of %s = %+v, %v", b, got, err)
			}
		})
	}
}

func TestMarshalShDataLayout(t *testing.T) {
	supported := true

	b, err := MarshalShData(ShData{SMSRegistration: &SMSRegistrationInfo{IPSMGWNumber: "15550000050", SCAddress: "15550000000", SBISupported: &supported}})
	if err != nil {
		t.Fatal(err)
	}

	want := `<Sh-Data><Sh-IMS-Data><Extension><Extension><Extension><SMSRegistrationInfo>` +
		`<IP-SM-GW-Number>15550000050</IP-SM-GW-Number><Extension><SCAddress>15550000000</SCAddress>` +
		`<Extension><IP-SM-GW-SBISupportIndication>1</IP-SM-GW-SBISupportIndication></Extension></Extension>` +
		`</SMSRegistrationInfo></Extension></Extension></Extension></Sh-IMS-Data></Sh-Data>`

	if !strings.HasSuffix(string(b), want) {
		t.Fatalf("Sh-Data = %s", b)
	}

	b, err = MarshalShData(ShData{SMSRegistration: &SMSRegistrationInfo{}})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(b), "<SMSRegistrationInfo><IP-SM-GW-Number></IP-SM-GW-Number></SMSRegistrationInfo>") {
		t.Fatalf("deregistration Sh-Data = %s", b)
	}
}

func TestParseShDataFromHSS(t *testing.T) {
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<Sh-Data xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <PublicIdentifiers><IMSPublicIdentity>tel:+15551230002</IMSPublicIdentity></PublicIdentifiers>
  <Sh-IMS-Data>
    <SCSCFName>sip:scscf.example.org</SCSCFName>
    <Extension>
      <PSIActivation>1</PSIActivation>
      <Extension>
        <Extension>
          <IPv4Address>10.0.0.1</IPv4Address>
          <SMSRegistrationInfo>
            <IP-SM-GW-Number> 15550000050 </IP-SM-GW-Number>
            <Extension><SCAddress>15550000000</SCAddress></Extension>
          </SMSRegistrationInfo>
        </Extension>
      </Extension>
    </Extension>
  </Sh-IMS-Data>
</Sh-Data>
<!-- trailing comment -->`

	got, err := ParseShData([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	want := ShData{SMSRegistration: &SMSRegistrationInfo{IPSMGWNumber: "15550000050", SCAddress: "15550000000"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed %+v", got)
	}
}

func smsRegistrationDocument(inner string) string {
	return `<Sh-Data><Sh-IMS-Data><Extension><Extension><Extension><SMSRegistrationInfo>` + inner +
		`</SMSRegistrationInfo></Extension></Extension></Extension></Sh-IMS-Data></Sh-Data>`
}

func TestParseShDataStrictness(t *testing.T) {
	supported, unsupported := true, false

	for name, tt := range map[string]struct {
		doc  string
		want SMSRegistrationInfo
	}{
		"self-closing number": {smsRegistrationDocument(`<IP-SM-GW-Number/>`), SMSRegistrationInfo{}},
		"foreign-namespace number ignored": {
			smsRegistrationDocument(`<IP-SM-GW-Number>1</IP-SM-GW-Number><o:IP-SM-GW-Number xmlns:o="urn:o">2</o:IP-SM-GW-Number>`),
			SMSRegistrationInfo{IPSMGWNumber: "1"},
		},
		"foreign-namespace extension ignored": {
			smsRegistrationDocument(`<IP-SM-GW-Number>1</IP-SM-GW-Number><o:Extension xmlns:o="urn:o"><SCAddress>9</SCAddress></o:Extension>`),
			SMSRegistrationInfo{IPSMGWNumber: "1"},
		},
		"SBI without SC address": {
			smsRegistrationDocument(`<IP-SM-GW-Number>1</IP-SM-GW-Number><Extension><Extension><IP-SM-GW-SBISupportIndication>1</IP-SM-GW-SBISupportIndication></Extension></Extension>`),
			SMSRegistrationInfo{IPSMGWNumber: "1", SBISupported: &supported},
		},
		"SBI with leading zero": {
			smsRegistrationDocument(`<IP-SM-GW-Number>1</IP-SM-GW-Number><Extension><Extension><IP-SM-GW-SBISupportIndication>01</IP-SM-GW-SBISupportIndication></Extension></Extension>`),
			SMSRegistrationInfo{IPSMGWNumber: "1", SBISupported: &supported},
		},
		"SBI with sign": {
			smsRegistrationDocument(`<IP-SM-GW-Number>1</IP-SM-GW-Number><Extension><Extension><IP-SM-GW-SBISupportIndication>+0</IP-SM-GW-SBISupportIndication></Extension></Extension>`),
			SMSRegistrationInfo{IPSMGWNumber: "1", SBISupported: &unsupported},
		},
	} {
		got, err := ParseShData([]byte(tt.doc))
		if err != nil || got.SMSRegistration == nil || !reflect.DeepEqual(*got.SMSRegistration, tt.want) {
			t.Errorf("%s: parsed %+v, %v", name, got.SMSRegistration, err)
		}
	}
}

func TestParseShDataInDefaultNamespace(t *testing.T) {
	doc := `<!DOCTYPE Sh-Data><Sh-Data xmlns="urn:example"><Sh-IMS-Data><Extension><Extension><Extension><SMSRegistrationInfo>` +
		`<IP-SM-GW-Number>15550000050</IP-SM-GW-Number></SMSRegistrationInfo></Extension></Extension></Extension></Sh-IMS-Data></Sh-Data>`

	got, err := ParseShData([]byte(doc))
	if err != nil || got.SMSRegistration == nil || got.SMSRegistration.IPSMGWNumber != "15550000050" {
		t.Fatalf("parsed %+v, %v", got.SMSRegistration, err)
	}
}

func TestParseShDataErrors(t *testing.T) {
	registration := func(inner string) string {
		return `<Sh-Data><Sh-IMS-Data><Extension><Extension><Extension><SMSRegistrationInfo>` + inner +
			`</SMSRegistrationInfo></Extension></Extension></Extension></Sh-IMS-Data></Sh-Data>`
	}

	for name, doc := range map[string]string{
		"empty":             ``,
		"not XML":           `Sh-Data`,
		"wrong root":        `<IMSSubscription/>`,
		"unclosed":          `<Sh-Data>`,
		"trailing element":  `<Sh-Data/><Sh-Data/>`,
		"trailing text":     `<Sh-Data/>x`,
		"no number":         registration(``),
		"number not digits": registration(`<IP-SM-GW-Number>+15550000050</IP-SM-GW-Number>`),
		"number too long":   registration(`<IP-SM-GW-Number>1234567890123456</IP-SM-GW-Number>`),
		"bad SC address": registration(`<IP-SM-GW-Number>1</IP-SM-GW-Number>` +
			`<Extension><SCAddress>sc</SCAddress></Extension>`),
		"bad SBI indication": registration(`<IP-SM-GW-Number>1</IP-SM-GW-Number>` +
			`<Extension><Extension><IP-SM-GW-SBISupportIndication>2</IP-SM-GW-SBISupportIndication></Extension></Extension>`),
		"empty SBI indication": registration(`<IP-SM-GW-Number>1</IP-SM-GW-Number>` +
			`<Extension><Extension><IP-SM-GW-SBISupportIndication/></Extension></Extension>`),
		"two numbers":         registration(`<IP-SM-GW-Number>1</IP-SM-GW-Number><IP-SM-GW-Number>2</IP-SM-GW-Number>`),
		"two SC addresses":    registration(`<IP-SM-GW-Number>1</IP-SM-GW-Number><Extension><SCAddress>1</SCAddress><SCAddress>2</SCAddress></Extension>`),
		"number with element": registration(`<IP-SM-GW-Number>1<x>2</x>3</IP-SM-GW-Number>`),
		"number with NBSP":    registration("<IP-SM-GW-Number>\u00a01</IP-SM-GW-Number>"),
		"two registrations": `<Sh-Data><Sh-IMS-Data><Extension><Extension><Extension>` +
			`<SMSRegistrationInfo><IP-SM-GW-Number>1</IP-SM-GW-Number></SMSRegistrationInfo><SMSRegistrationInfo/>` +
			`</Extension></Extension></Extension></Sh-IMS-Data></Sh-Data>`,
		"two Sh-IMS-Data":     `<Sh-Data><Sh-IMS-Data/><Sh-IMS-Data/></Sh-Data>`,
		"content before root": `x<Sh-Data/>`,
	} {
		if _, err := ParseShData([]byte(doc)); !errors.Is(err, ErrInvalidUserData) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestMarshalShDataValidation(t *testing.T) {
	for name, r := range map[string]SMSRegistrationInfo{
		"number not digits": {IPSMGWNumber: "+15550000050"},
		"SC address":        {IPSMGWNumber: "1", SCAddress: "x"},
	} {
		if _, err := MarshalShData(ShData{SMSRegistration: &r}); !errors.Is(err, ErrInvalidUserData) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
