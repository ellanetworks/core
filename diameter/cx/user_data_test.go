// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"encoding/xml"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func ptr[T any](v T) *T {
	return &v
}

func fullSubscription() IMSSubscription {
	return IMSSubscription{
		PrivateIdentity: testPrivate,
		IMSI:            "001010000000001",
		ServiceProfiles: []ServiceProfile{
			{
				PublicIdentities: []ProfileIdentity{
					{Identity: testPublic, Barred: true},
					{Identity: testTel, DisplayName: "Alice", AliasIdentityGroupID: "1"},
					{Identity: "sip:!.*!@example.org", Type: IdentityWildcardedIMPU, WildcardedIMPU: "sip:!.*!@example.org"},
					{Identity: "sip:conf-1@example.org", Type: IdentityWildcardedPSI, WildcardedPSI: "sip:conf-!.*!@example.org"},
				},
				SubscribedMediaProfileID: ptr(int32(0)),
				InitialFilterCriteria: []InitialFilterCriteria{
					{
						Priority: 0,
						Trigger: &TriggerPoint{ServicePointTriggers: []ServicePointTrigger{
							{Groups: []int32{0}, Method: ptr("REGISTER"), RegistrationTypes: []RegistrationType{RegistrationTypeInitial, RegistrationTypeDeregistration}},
							{Groups: []int32{1}, SIPHeader: &HeaderMatch{Header: "Event", Content: ptr(".*presence.*")}},
							{Negated: true, Groups: []int32{1, 2}, SessionCase: ptr(SessionCaseTerminatingUnregistered)},
							{Groups: []int32{3}, RequestURI: ptr("sip:voicemail@example.org")},
							{Groups: []int32{4}, SessionDescription: &SessionDescriptionMatch{Line: "m", Content: ptr("audio.*")}},
						}},
						ApplicationServer: ApplicationServer{
							ServerName:              "sip:as.example.org:5060",
							DefaultHandling:         ptr(DefaultHandlingSessionTerminated),
							ServiceInfo:             ptr("mmtel"),
							IncludeRegisterRequest:  true,
							IncludeRegisterResponse: true,
						},
						ProfilePart: ptr(ProfilePartRegistered),
					},
					{Priority: 1, ApplicationServer: ApplicationServer{ServerName: "sip:fallback.example.org"}},
				},
				SharedIFCSetIDs: []int32{3, 4},
			},
			{PublicIdentities: []ProfileIdentity{{Identity: "sip:bob@example.org", Type: IdentityDistinctPSI}}},
		},
	}
}

func TestUserDataRoundTrip(t *testing.T) {
	for name, s := range map[string]IMSSubscription{
		"full":    fullSubscription(),
		"minimal": {PrivateIdentity: testPrivate, ServiceProfiles: []ServiceProfile{{PublicIdentities: []ProfileIdentity{{Identity: testPublic}}}}},
	} {
		t.Run(name, func(t *testing.T) {
			b, err := MarshalUserData(s)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.HasPrefix(string(b), `<?xml version="1.0" encoding="UTF-8"?>`) {
				t.Fatalf("no XML declaration: %s", b)
			}

			got, err := ParseUserData(b)
			if err != nil || !reflect.DeepEqual(got, s) {
				t.Fatalf("round trip = %+v, %v\n%s", got, err, b)
			}
		})
	}
}

func TestUserDataBooleansAreNumeric(t *testing.T) {
	b, err := MarshalUserData(fullSubscription())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"<BarringIndication>1</BarringIndication>", "<ConditionTypeCNF>0</ConditionTypeCNF>", "<ConditionNegated>1</ConditionNegated>"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s", want)
		}
	}
}

const peerProfile = `<?xml version="1.0" encoding="UTF-8"?>
<IMSSubscription xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <PrivateID> alice@ims.example.org </PrivateID>
  <ServiceProfile>
    <PublicIdentity>
      <BarringIndication>true</BarringIndication>
      <Identity>sip:alice@ims.example.org</Identity>
      <Extension><IdentityType>0</IdentityType></Extension>
    </PublicIdentity>
    <PublicIdentity><Identity>tel:+15551230002</Identity></PublicIdentity>
    <InitialFilterCriteria>
      <Priority>0</Priority>
      <TriggerPoint>
        <ConditionTypeCNF>false</ConditionTypeCNF>
        <SPT><ConditionNegated>0</ConditionNegated><Group>0</Group><Method>PUBLISH</Method><Extension></Extension></SPT>
        <SPT><Group>0</Group><SIPHeader><Header>Event</Header><Content>.*presence.*</Content></SIPHeader></SPT>
      </TriggerPoint>
      <ApplicationServer><ServerName>sip:presence.example.org:6060</ServerName><DefaultHandling>0</DefaultHandling></ApplicationServer>
      <Extension><Vendor>ignored</Vendor></Extension>
    </InitialFilterCriteria>
    <Proprietary xmlns="urn:example">ignored</Proprietary>
  </ServiceProfile>
</IMSSubscription>`

func TestParsePeerUserData(t *testing.T) {
	s, err := ParseUserData([]byte(peerProfile))
	if err != nil {
		t.Fatal(err)
	}

	want := IMSSubscription{
		PrivateIdentity: "alice@ims.example.org",
		ServiceProfiles: []ServiceProfile{{
			PublicIdentities: []ProfileIdentity{
				{Identity: "sip:alice@ims.example.org", Barred: true},
				{Identity: "tel:+15551230002"},
			},
			InitialFilterCriteria: []InitialFilterCriteria{{
				Trigger: &TriggerPoint{ServicePointTriggers: []ServicePointTrigger{
					{Groups: []int32{0}, Method: ptr("PUBLISH")},
					{Groups: []int32{0}, SIPHeader: &HeaderMatch{Header: "Event", Content: ptr(".*presence.*")}},
				}},
				ApplicationServer: ApplicationServer{ServerName: "sip:presence.example.org:6060", DefaultHandling: ptr(DefaultHandlingSessionContinued)},
			}},
		}},
	}

	if !reflect.DeepEqual(s, want) {
		t.Fatalf("ParseUserData = %+v", s)
	}
}

func TestUserDataValidation(t *testing.T) {
	for name, mutate := range map[string]func(*IMSSubscription){
		"no PrivateID":      func(s *IMSSubscription) { s.PrivateIdentity = "" },
		"no profile":        func(s *IMSSubscription) { s.ServiceProfiles = nil },
		"no identity":       func(s *IMSSubscription) { s.ServiceProfiles[1].PublicIdentities = nil },
		"empty identity":    func(s *IMSSubscription) { s.ServiceProfiles[1].PublicIdentities[0].Identity = " " },
		"identity type":     func(s *IMSSubscription) { s.ServiceProfiles[1].PublicIdentities[0].Type = 5 },
		"negative media":    func(s *IMSSubscription) { s.ServiceProfiles[0].SubscribedMediaProfileID = ptr(int32(-1)) },
		"negative shared":   func(s *IMSSubscription) { s.ServiceProfiles[0].SharedIFCSetIDs = []int32{-1} },
		"negative priority": func(s *IMSSubscription) { s.ServiceProfiles[0].InitialFilterCriteria[1].Priority = -1 },
		"no AS": func(s *IMSSubscription) {
			s.ServiceProfiles[0].InitialFilterCriteria[1].ApplicationServer.ServerName = ""
		},
		"default handling": func(s *IMSSubscription) {
			s.ServiceProfiles[0].InitialFilterCriteria[1].ApplicationServer.DefaultHandling = ptr(DefaultHandling(2))
		},
		"profile part": func(s *IMSSubscription) {
			s.ServiceProfiles[0].InitialFilterCriteria[1].ProfilePart = ptr(ProfilePart(2))
		},
		"empty trigger":      func(s *IMSSubscription) { s.ServiceProfiles[0].InitialFilterCriteria[1].Trigger = &TriggerPoint{} },
		"two conditions":     func(s *IMSSubscription) { spt(s).RequestURI = ptr("sip:x@example.org") },
		"no condition":       func(s *IMSSubscription) { spt(s).Method = nil },
		"no group":           func(s *IMSSubscription) { spt(s).Groups = nil },
		"negative group":     func(s *IMSSubscription) { spt(s).Groups = []int32{-1} },
		"session case":       func(s *IMSSubscription) { spt(s).Method, spt(s).SessionCase = nil, ptr(SessionCase(5)) },
		"registration type":  func(s *IMSSubscription) { spt(s).RegistrationTypes = []RegistrationType{3} },
		"three registration": func(s *IMSSubscription) { spt(s).RegistrationTypes = []RegistrationType{0, 1, 2} },
		"padded PrivateID":   func(s *IMSSubscription) { s.PrivateIdentity = " " + testPrivate },
		"padded identity":    func(s *IMSSubscription) { s.ServiceProfiles[1].PublicIdentities[0].Identity += "\n" },
		"padded AS": func(s *IMSSubscription) {
			s.ServiceProfiles[0].InitialFilterCriteria[1].ApplicationServer.ServerName += " "
		},
		"padded wildcard":   func(s *IMSSubscription) { s.ServiceProfiles[0].PublicIdentities[2].WildcardedIMPU += " " },
		"control in name":   func(s *IMSSubscription) { s.ServiceProfiles[0].PublicIdentities[1].DisplayName = "x\x01y" },
		"control in method": func(s *IMSSubscription) { spt(s).Method = ptr("INV\x00ITE") },
		"control in header": func(s *IMSSubscription) { spt(s).Method, spt(s).SIPHeader = nil, &HeaderMatch{Header: "Event\x1b"} },
		"control in content": func(s *IMSSubscription) {
			spt(s).Method, spt(s).SessionDescription = nil, &SessionDescriptionMatch{Line: "m", Content: ptr("\x7f\x02")}
		},
		"control in IMSI": func(s *IMSSubscription) { s.IMSI = "00101\x0b" },
		"invalid UTF-8":   func(s *IMSSubscription) { s.ServiceProfiles[0].PublicIdentities[1].AliasIdentityGroupID = "\xff" },
		"noncharacter": func(s *IMSSubscription) {
			s.ServiceProfiles[0].InitialFilterCriteria[0].ApplicationServer.ServiceInfo = ptr("\uffff")
		},
	} {
		s := fullSubscription()
		mutate(&s)

		if _, err := MarshalUserData(s); !errors.Is(err, ErrInvalidUserData) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func spt(s *IMSSubscription) *ServicePointTrigger {
	return &s.ServiceProfiles[0].InitialFilterCriteria[0].Trigger.ServicePointTriggers[0]
}

func TestParseUserDataRejects(t *testing.T) {
	for name, doc := range map[string]string{
		"not XML":             "IMSSubscription",
		"other root":          "<ServiceProfile/>",
		"unterminated":        "<IMSSubscription><PrivateID>a</PrivateID>",
		"no profile":          "<IMSSubscription><PrivateID>a</PrivateID></IMSSubscription>",
		"bad boolean":         "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><BarringIndication>yes</BarringIndication><Identity>sip:a@b</Identity></PublicIdentity></ServiceProfile></IMSSubscription>",
		"bad priority":        "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity><InitialFilterCriteria><Priority>x</Priority></InitialFilterCriteria></ServiceProfile></IMSSubscription>",
		"registration type":   "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity><InitialFilterCriteria><Priority>0</Priority><TriggerPoint><ConditionTypeCNF>0</ConditionTypeCNF><SPT><Group>0</Group><Method>INVITE</Method><Extension><RegistrationType>-1</RegistrationType></Extension></SPT></TriggerPoint><ApplicationServer><ServerName>sip:as@b</ServerName></ApplicationServer></InitialFilterCriteria></ServiceProfile></IMSSubscription>",
		"trailing element":    "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity></ServiceProfile></IMSSubscription><IMSSubscription/>",
		"trailing text":       "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity></ServiceProfile></IMSSubscription>junk",
		"unterminated tail":   "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity></ServiceProfile></IMSSubscription><garbage",
		"priority overflow":   "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity><InitialFilterCriteria><Priority>99999999999</Priority><ApplicationServer><ServerName>sip:as@b</ServerName></ApplicationServer></InitialFilterCriteria></ServiceProfile></IMSSubscription>",
		"no priority":         "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity><InitialFilterCriteria><ApplicationServer><ServerName>sip:as@b</ServerName></ApplicationServer></InitialFilterCriteria></ServiceProfile></IMSSubscription>",
		"no ConditionTypeCNF": "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity><InitialFilterCriteria><Priority>0</Priority><TriggerPoint><SPT><Group>0</Group><Method>INVITE</Method></SPT></TriggerPoint><ApplicationServer><ServerName>sip:as@b</ServerName></ApplicationServer></InitialFilterCriteria></ServiceProfile></IMSSubscription>",
		"control character":   "<IMSSubscription><PrivateID>a\x01</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity></ServiceProfile></IMSSubscription>",
		"entity expansion":    `<!DOCTYPE x [<!ENTITY a "aaaa">]><IMSSubscription><PrivateID>&a;</PrivateID></IMSSubscription>`,
	} {
		if _, err := ParseUserData([]byte(doc)); !errors.Is(err, ErrInvalidUserData) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

const fullSubscriptionXML = `<IMSSubscription>
<PrivateID>001010000000001@ims.mnc001.mcc001.3gppnetwork.org</PrivateID>
<ServiceProfile>
<PublicIdentity>
<BarringIndication>1</BarringIndication>
<Identity>sip:001010000000001@ims.mnc001.mcc001.3gppnetwork.org</Identity>
</PublicIdentity>
<PublicIdentity>
<Identity>tel:+15551230002</Identity>
<Extension>
<IdentityType>0</IdentityType>
<Extension>
<DisplayName>Alice</DisplayName>
<AliasIdentityGroupID>1</AliasIdentityGroupID>
</Extension>
</Extension>
</PublicIdentity>
<PublicIdentity>
<Identity>sip:!.*!@example.org</Identity>
<Extension>
<IdentityType>4</IdentityType>
<Extension>
<Extension>
<WildcardedIMPU>sip:!.*!@example.org</WildcardedIMPU>
</Extension>
</Extension>
</Extension>
</PublicIdentity>
<PublicIdentity>
<Identity>sip:conf-1@example.org</Identity>
<Extension>
<IdentityType>2</IdentityType>
<WildcardedPSI>sip:conf-!.*!@example.org</WildcardedPSI>
</Extension>
</PublicIdentity>
<CoreNetworkServicesAuthorization>
<SubscribedMediaProfileId>0</SubscribedMediaProfileId>
</CoreNetworkServicesAuthorization>
<InitialFilterCriteria>
<Priority>0</Priority>
<TriggerPoint>
<ConditionTypeCNF>0</ConditionTypeCNF>
<SPT>
<ConditionNegated>0</ConditionNegated>
<Group>0</Group>
<Method>REGISTER</Method>
<Extension>
<RegistrationType>0</RegistrationType>
<RegistrationType>2</RegistrationType>
</Extension>
</SPT>
<SPT>
<ConditionNegated>0</ConditionNegated>
<Group>1</Group>
<SIPHeader>
<Header>Event</Header>
<Content>.*presence.*</Content>
</SIPHeader>
</SPT>
<SPT>
<ConditionNegated>1</ConditionNegated>
<Group>1</Group>
<Group>2</Group>
<SessionCase>2</SessionCase>
</SPT>
<SPT>
<ConditionNegated>0</ConditionNegated>
<Group>3</Group>
<RequestURI>sip:voicemail@example.org</RequestURI>
</SPT>
<SPT>
<ConditionNegated>0</ConditionNegated>
<Group>4</Group>
<SessionDescription>
<Line>m</Line>
<Content>audio.*</Content>
</SessionDescription>
</SPT>
</TriggerPoint>
<ApplicationServer>
<ServerName>sip:as.example.org:5060</ServerName>
<DefaultHandling>1</DefaultHandling>
<ServiceInfo>mmtel</ServiceInfo>
<Extension>
<IncludeRegisterRequest>
</IncludeRegisterRequest>
<IncludeRegisterResponse>
</IncludeRegisterResponse>
</Extension>
</ApplicationServer>
<ProfilePartIndicator>0</ProfilePartIndicator>
</InitialFilterCriteria>
<InitialFilterCriteria>
<Priority>1</Priority>
<ApplicationServer>
<ServerName>sip:fallback.example.org</ServerName>
</ApplicationServer>
</InitialFilterCriteria>
<Extension>
<SharedIFCSetID>3</SharedIFCSetID>
<SharedIFCSetID>4</SharedIFCSetID>
</Extension>
</ServiceProfile>
<ServiceProfile>
<PublicIdentity>
<Identity>sip:bob@example.org</Identity>
<Extension>
<IdentityType>1</IdentityType>
</Extension>
</PublicIdentity>
</ServiceProfile>
<Extension>
<IMSI>001010000000001</IMSI>
</Extension>
</IMSSubscription>`

func TestUserDataWireFormat(t *testing.T) {
	b, err := MarshalUserData(fullSubscription())
	if err != nil {
		t.Fatal(err)
	}

	if want := xml.Header + strings.ReplaceAll(fullSubscriptionXML, "\n", ""); string(b) != want {
		t.Fatalf("MarshalUserData =\n%s\nwant\n%s", b, want)
	}
}

func TestParseUserDataAcceptsTrailingMarkup(t *testing.T) {
	doc := `<?xml version="1.0"?><IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity>` +
		`</PublicIdentity></ServiceProfile></IMSSubscription>` + "\n  <!-- generated -->\n<?pi x?>\n"

	if _, err := ParseUserData([]byte(doc)); err != nil {
		t.Fatal(err)
	}
}

func TestUserDataWhitespace(t *testing.T) {
	doc := "<IMSSubscription><PrivateID>\n a@b \n</PrivateID><ServiceProfile><PublicIdentity><Identity> sip:a@b </Identity>" +
		"</PublicIdentity></ServiceProfile><Extension><IMSI> 001010000000001 </IMSI></Extension></IMSSubscription>"

	s, err := ParseUserData([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	if s.PrivateIdentity != "a@b" || s.ServiceProfiles[0].PublicIdentities[0].Identity != "sip:a@b" || s.IMSI != " 001010000000001 " {
		t.Fatalf("ParseUserData = %+v", s)
	}

	b, err := MarshalUserData(s)
	if err != nil {
		t.Fatal(err)
	}

	again, err := ParseUserData(b)
	if err != nil || !reflect.DeepEqual(again, s) {
		t.Fatalf("round trip = %+v, %v", again, err)
	}
}

func TestDistinctIdentityTypeOmitted(t *testing.T) {
	b, err := MarshalUserData(IMSSubscription{PrivateIdentity: testPrivate, ServiceProfiles: []ServiceProfile{{PublicIdentities: []ProfileIdentity{
		{Identity: testPublic},
		{Identity: testTel, Type: IdentityDistinctPSI},
	}}}})
	if err != nil {
		t.Fatal(err)
	}

	if got := strings.Count(string(b), "<IdentityType>"); got != 1 || !strings.Contains(string(b), "<IdentityType>1</IdentityType>") {
		t.Fatalf("IdentityType emitted %d times:\n%s", got, b)
	}
}
