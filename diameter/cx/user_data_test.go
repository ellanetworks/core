// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func ptr[T any](v T) *T {
	return &v
}

func fullSubscription() IMSSubscription {
	return IMSSubscription{
		PrivateID: testPrivate,
		IMSI:      "001010000000001",
		ServiceProfiles: []ServiceProfile{
			{
				PublicIdentities: []PublicIdentity{
					{Identity: testPublic, Barred: true},
					{Identity: testTel, DisplayName: "Alice", AliasIdentityGroupID: "1"},
					{Identity: "sip:!.*!@example.org", Type: IdentityWildcardedIMPU, WildcardedIMPU: "sip:!.*!@example.org"},
					{Identity: "sip:conf-1@example.org", Type: IdentityWildcardedPSI, WildcardedPSI: "sip:conf-!.*!@example.org"},
				},
				SubscribedMediaProfileID: ptr(0),
				InitialFilterCriteria: []InitialFilterCriteria{
					{
						Priority: 0,
						Trigger: &TriggerPoint{ServicePointTriggers: []ServicePointTrigger{
							{Groups: []int{0}, Method: ptr("REGISTER"), RegistrationTypes: []uint8{RegistrationTypeInitial, RegistrationTypeDeregistration}},
							{Groups: []int{1}, SIPHeader: &HeaderMatch{Header: "Event", Content: ptr(".*presence.*")}},
							{Negated: true, Groups: []int{1, 2}, SessionCase: ptr(SessionCaseTerminatingUnregistered)},
							{Groups: []int{3}, RequestURI: ptr("sip:voicemail@example.org")},
							{Groups: []int{4}, SessionDescription: &SessionDescriptionMatch{Line: "m", Content: ptr("audio.*")}},
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
				SharedIFCSetIDs: []int{3, 4},
			},
			{PublicIdentities: []PublicIdentity{{Identity: "sip:bob@example.org", Type: IdentityDistinctPSI}}},
		},
	}
}

func TestUserDataRoundTrip(t *testing.T) {
	for name, s := range map[string]IMSSubscription{
		"full":    fullSubscription(),
		"minimal": {PrivateID: testPrivate, ServiceProfiles: []ServiceProfile{{PublicIdentities: []PublicIdentity{{Identity: testPublic}}}}},
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
		PrivateID: "alice@ims.example.org",
		ServiceProfiles: []ServiceProfile{{
			PublicIdentities: []PublicIdentity{
				{Identity: "sip:alice@ims.example.org", Barred: true},
				{Identity: "tel:+15551230002"},
			},
			InitialFilterCriteria: []InitialFilterCriteria{{
				Trigger: &TriggerPoint{ServicePointTriggers: []ServicePointTrigger{
					{Groups: []int{0}, Method: ptr("PUBLISH")},
					{Groups: []int{0}, SIPHeader: &HeaderMatch{Header: "Event", Content: ptr(".*presence.*")}},
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
		"no PrivateID":      func(s *IMSSubscription) { s.PrivateID = "" },
		"no profile":        func(s *IMSSubscription) { s.ServiceProfiles = nil },
		"no identity":       func(s *IMSSubscription) { s.ServiceProfiles[1].PublicIdentities = nil },
		"empty identity":    func(s *IMSSubscription) { s.ServiceProfiles[1].PublicIdentities[0].Identity = " " },
		"identity type":     func(s *IMSSubscription) { s.ServiceProfiles[1].PublicIdentities[0].Type = 5 },
		"negative media":    func(s *IMSSubscription) { s.ServiceProfiles[0].SubscribedMediaProfileID = ptr(-1) },
		"negative shared":   func(s *IMSSubscription) { s.ServiceProfiles[0].SharedIFCSetIDs = []int{-1} },
		"negative priority": func(s *IMSSubscription) { s.ServiceProfiles[0].InitialFilterCriteria[1].Priority = -1 },
		"no AS": func(s *IMSSubscription) {
			s.ServiceProfiles[0].InitialFilterCriteria[1].ApplicationServer.ServerName = ""
		},
		"default handling": func(s *IMSSubscription) {
			s.ServiceProfiles[0].InitialFilterCriteria[1].ApplicationServer.DefaultHandling = ptr(uint8(2))
		},
		"profile part":       func(s *IMSSubscription) { s.ServiceProfiles[0].InitialFilterCriteria[1].ProfilePart = ptr(uint8(2)) },
		"empty trigger":      func(s *IMSSubscription) { s.ServiceProfiles[0].InitialFilterCriteria[1].Trigger = &TriggerPoint{} },
		"two conditions":     func(s *IMSSubscription) { spt(s).RequestURI = ptr("sip:x@example.org") },
		"no condition":       func(s *IMSSubscription) { spt(s).Method = nil },
		"no group":           func(s *IMSSubscription) { spt(s).Groups = nil },
		"negative group":     func(s *IMSSubscription) { spt(s).Groups = []int{-1} },
		"session case":       func(s *IMSSubscription) { spt(s).Method, spt(s).SessionCase = nil, ptr(uint8(5)) },
		"registration type":  func(s *IMSSubscription) { spt(s).RegistrationTypes = []uint8{3} },
		"three registration": func(s *IMSSubscription) { spt(s).RegistrationTypes = []uint8{0, 1, 2} },
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
		"not XML":           "IMSSubscription",
		"other root":        "<ServiceProfile/>",
		"unterminated":      "<IMSSubscription><PrivateID>a</PrivateID>",
		"no profile":        "<IMSSubscription><PrivateID>a</PrivateID></IMSSubscription>",
		"bad boolean":       "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><BarringIndication>yes</BarringIndication><Identity>sip:a@b</Identity></PublicIdentity></ServiceProfile></IMSSubscription>",
		"bad priority":      "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity><InitialFilterCriteria><Priority>x</Priority></InitialFilterCriteria></ServiceProfile></IMSSubscription>",
		"registration type": "<IMSSubscription><PrivateID>a</PrivateID><ServiceProfile><PublicIdentity><Identity>sip:a@b</Identity></PublicIdentity><InitialFilterCriteria><Priority>0</Priority><TriggerPoint><ConditionTypeCNF>0</ConditionTypeCNF><SPT><Group>0</Group><Method>INVITE</Method><Extension><RegistrationType>-1</RegistrationType></Extension></SPT></TriggerPoint><ApplicationServer><ServerName>sip:as@b</ServerName></ApplicationServer></InitialFilterCriteria></ServiceProfile></IMSSubscription>",
		"entity expansion":  `<!DOCTYPE x [<!ENTITY a "aaaa">]><IMSSubscription><PrivateID>&a;</PrivateID></IMSSubscription>`,
	} {
		if _, err := ParseUserData([]byte(doc)); !errors.Is(err, ErrInvalidUserData) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func xsdValidator(t *testing.T) func(schema, doc string) error {
	t.Helper()

	if path, err := exec.LookPath("xmllint"); err == nil {
		return func(schema, doc string) error {
			out, err := exec.CommandContext(t.Context(), path, "--noout", "--schema", schema, doc).CombinedOutput()
			if err != nil {
				return errors.New(string(out))
			}

			return nil
		}
	}

	if python, err := exec.LookPath("python3"); err == nil && exec.CommandContext(t.Context(), python, "-c", "import lxml").Run() == nil {
		const script = `import sys
from lxml import etree
s = etree.XMLSchema(etree.parse(sys.argv[1]))
d = etree.parse(sys.argv[2])
if not s.validate(d):
    sys.exit(str(s.error_log))`

		return func(schema, doc string) error {
			out, err := exec.CommandContext(t.Context(), python, "-c", script, schema, doc).CombinedOutput()
			if err != nil {
				return errors.New(string(out))
			}

			return nil
		}
	}

	return nil
}

func TestUserDataValidatesAgainstSchemas(t *testing.T) {
	dir := os.Getenv("CX_XSD_DIR")
	if dir == "" {
		t.Skip("CX_XSD_DIR not set to a directory holding CxData_Type_Rel19.xsd and CxDataType_Rel7.xsd")
	}

	validate := xsdValidator(t)
	if validate == nil {
		t.Skip("neither xmllint nor python3 with lxml is available")
	}

	rel7 := fullSubscription()
	rel7.ServiceProfiles[0].PublicIdentities = slices.DeleteFunc(rel7.ServiceProfiles[0].PublicIdentities, func(id PublicIdentity) bool {
		return id.Type > IdentityWildcardedPSI
	})

	docs := map[string]struct {
		s       IMSSubscription
		schemas []string
	}{
		"full":    {fullSubscription(), []string{"CxData_Type_Rel19.xsd"}},
		"rel7":    {rel7, []string{"CxData_Type_Rel19.xsd", "CxDataType_Rel7.xsd"}},
		"minimal": {IMSSubscription{PrivateID: testPrivate, ServiceProfiles: []ServiceProfile{{PublicIdentities: []PublicIdentity{{Identity: testPublic}}}}}, []string{"CxData_Type_Rel19.xsd", "CxDataType_Rel7.xsd"}},
	}

	for name, d := range docs {
		s := d.s

		b, err := MarshalUserData(s)
		if err != nil {
			t.Fatal(err)
		}

		doc := filepath.Join(t.TempDir(), name+".xml")
		if err := os.WriteFile(doc, b, 0o600); err != nil {
			t.Fatal(err)
		}

		for _, schema := range d.schemas {
			if err := validate(filepath.Join(dir, schema), doc); err != nil {
				t.Errorf("%s against %s: %v\n%s", name, schema, err, b)
			}
		}
	}
}
