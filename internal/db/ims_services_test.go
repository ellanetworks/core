// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func registerIFC() db.IMSInitialFilterCriterion {
	return db.IMSInitialFilterCriterion{
		TriggerPoint: db.IMSTriggerPoint{
			ServicePointTriggers: []db.IMSServicePointTrigger{
				{Groups: []int32{0}, Method: ptr("REGISTER"), RegistrationTypes: []db.IMSRegistrationType{db.IMSRegistrationTypeInitial, db.IMSRegistrationTypeDeregistration}},
			},
		},
		DefaultHandling:         db.IMSDefaultHandlingSessionContinued,
		IncludeRegisterRequest:  true,
		IncludeRegisterResponse: true,
	}
}

func originatingIFC() db.IMSInitialFilterCriterion {
	return db.IMSInitialFilterCriterion{
		TriggerPoint: db.IMSTriggerPoint{
			ConditionTypeCNF: true,
			ServicePointTriggers: []db.IMSServicePointTrigger{
				{Groups: []int32{0}, SessionCase: ptr(db.IMSSessionCaseOriginatingRegistered)},
				{Groups: []int32{1}, SIPHeader: &db.IMSHeaderMatch{Header: "P-Asserted-Service", Content: ptr(`^urn:urn-7:3gpp-service\.ims\.icsi\.mcptt$`)}},
				{Groups: []int32{2}, ConditionNegated: true, SessionDescription: &db.IMSSessionDescription{Line: "m", Content: ptr("video")}},
				{Groups: []int32{2}, RequestURI: ptr(`^conf\.example\.org$`)},
			},
		},
		DefaultHandling: db.IMSDefaultHandlingSessionTerminated,
	}
}

func TestIMSApplicationServersEndToEnd(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	as := &db.IMSApplicationServer{
		Name:                    "mcptt",
		ServerName:              "sip:mcptt.ims.example.org",
		PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org", "sip:conf-factory@mcptt.ims.example.org"},
	}

	if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := database.GetIMSApplicationServer(ctx, "mcptt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	want := db.IMSApplicationServer{
		ID:                      as.ID,
		Name:                    "mcptt",
		ServerName:              "sip:mcptt.ims.example.org",
		PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org", "sip:conf-factory@mcptt.ims.example.org"},
	}

	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("get = %+v, want %+v", *got, want)
	}

	if err := database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{Name: "mcptt", ServerName: "sip:other.example.org"}); !errors.Is(err, db.ErrAlreadyExists) {
		t.Fatalf("create with a taken name: err = %v, want ErrAlreadyExists", err)
	}

	other := &db.IMSApplicationServer{Name: "other", ServerName: "sip:other.example.org", PublicServiceIdentities: []string{"sip:conf-factory@mcptt.ims.example.org"}}
	if err := database.CreateIMSApplicationServer(ctx, other); !errors.Is(err, db.ErrPSIConflict) {
		t.Fatalf("create with a PSI hosted elsewhere: err = %v, want ErrPSIConflict", err)
	}

	if _, err := database.GetIMSApplicationServer(ctx, "other"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("a failed create left the application server behind: err = %v", err)
	}

	as.ServerName = "sip:mcptt2.ims.example.org"
	as.PublicServiceIdentities = []string{"tel:+15550100"}

	if err := database.UpdateIMSApplicationServer(ctx, as); err != nil {
		t.Fatalf("update: %v", err)
	}

	servers, err := database.ListIMSApplicationServers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want.ServerName = "sip:mcptt2.ims.example.org"
	want.PublicServiceIdentities = []string{"tel:+15550100"}

	if !reflect.DeepEqual(servers, []db.IMSApplicationServer{want}) {
		t.Fatalf("list = %+v, want %+v", servers, []db.IMSApplicationServer{want})
	}

	if err := database.UpdateIMSApplicationServer(ctx, &db.IMSApplicationServer{Name: "missing", ServerName: "sip:x.example.org"}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("update missing: err = %v, want ErrNotFound", err)
	}

	if err := database.DeleteIMSApplicationServer(ctx, "mcptt"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if err := database.DeleteIMSApplicationServer(ctx, "mcptt"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("delete again: err = %v, want ErrNotFound", err)
	}

	if err := database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{Name: "again", ServerName: "sip:a.example.org", PublicServiceIdentities: []string{"tel:+15550100"}}); err != nil {
		t.Fatalf("the PSIs of a deleted application server are still taken: %v", err)
	}
}

func TestIMSApplicationServerValidate(t *testing.T) {
	cases := map[string]db.IMSApplicationServer{
		"no name":            {ServerName: "sip:as.example.org"},
		"server name":        {Name: "as", ServerName: "as.example.org"},
		"tel server name":    {Name: "as", ServerName: "tel:+15550100"},
		"psi scheme":         {Name: "as", ServerName: "sip:as.example.org", PublicServiceIdentities: []string{"http://example.org"}},
		"psi listed twice":   {Name: "as", ServerName: "sip:as.example.org", PublicServiceIdentities: []string{"sip:a@example.org", "sip:a@example.org"}},
		"bare scheme as":     {Name: "as", ServerName: "sip:"},
		"bare scheme psi":    {Name: "as", ServerName: "sip:as.example.org", PublicServiceIdentities: []string{"tel:"}},
		"server name space":  {Name: "as", ServerName: "sip:as.example.org "},
		"psi bad wildcard":   {Name: "as", ServerName: "sip:as.example.org", PublicServiceIdentities: []string{"sip:!([!@example.org"}},
		"empty psi":          {Name: "as", ServerName: "sip:as.example.org", PublicServiceIdentities: []string{""}},
		"missing everything": {},
	}

	for name, as := range cases {
		t.Run(name, func(t *testing.T) {
			if err := as.Validate(); err == nil {
				t.Fatalf("Validate accepted %+v", as)
			}
		})
	}

	ok := db.IMSApplicationServer{Name: "as", ServerName: "SIPS:as.example.org", PublicServiceIdentities: []string{"sip:!.*!@as.example.org", "tel:+1555"}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate rejected %+v: %v", ok, err)
	}
}

func TestIMSServicesEndToEnd(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	as := &db.IMSApplicationServer{Name: "mcptt", ServerName: "sip:mcptt.ims.example.org"}
	if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
		t.Fatalf("create application server: %v", err)
	}

	mcptt := &db.IMSService{Name: "mcptt", Kind: db.IMSServiceKindMCPTT, ApplicationServerID: as.ID}
	if err := database.CreateIMSService(ctx, mcptt); err != nil {
		t.Fatalf("create mcptt service: %v", err)
	}

	custom := &db.IMSService{
		Name:                  "stand-in",
		Kind:                  db.IMSServiceKindCustom,
		ApplicationServerID:   as.ID,
		InitialFilterCriteria: []db.IMSInitialFilterCriterion{registerIFC(), originatingIFC()},
	}

	if err := database.CreateIMSService(ctx, custom); err != nil {
		t.Fatalf("create custom service: %v", err)
	}

	got, err := database.GetIMSService(ctx, "stand-in")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if !reflect.DeepEqual(got, custom) {
		t.Fatalf("get = %+v, want %+v", got, custom)
	}

	if err := database.CreateIMSService(ctx, &db.IMSService{Name: "orphan", Kind: db.IMSServiceKindMCPTT, ApplicationServerID: "missing"}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("create with an unknown application server: err = %v, want ErrNotFound", err)
	}

	if err := database.CreateIMSService(ctx, &db.IMSService{Name: "mcptt", Kind: db.IMSServiceKindMCPTT, ApplicationServerID: as.ID}); !errors.Is(err, db.ErrAlreadyExists) {
		t.Fatalf("create with a taken name: err = %v, want ErrAlreadyExists", err)
	}

	custom.InitialFilterCriteria = []db.IMSInitialFilterCriterion{originatingIFC()}
	if err := database.UpdateIMSService(ctx, custom); err != nil {
		t.Fatalf("update: %v", err)
	}

	services, err := database.ListIMSServices(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if !reflect.DeepEqual(services, []db.IMSService{*mcptt, *custom}) {
		t.Fatalf("list = %+v, want %+v", services, []db.IMSService{*mcptt, *custom})
	}

	custom.Kind = db.IMSServiceKindMCPTT
	custom.InitialFilterCriteria = nil

	if err := database.UpdateIMSService(ctx, custom); err != nil {
		t.Fatalf("update to mcptt: %v", err)
	}

	if got, err := database.GetIMSService(ctx, "stand-in"); err != nil || len(got.InitialFilterCriteria) != 0 {
		t.Fatalf("get after update to mcptt = %+v, err %v; want no iFCs", got, err)
	}

	if err := database.UpdateIMSService(ctx, &db.IMSService{Name: "missing", Kind: db.IMSServiceKindMCPTT, ApplicationServerID: as.ID}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("update missing: err = %v, want ErrNotFound", err)
	}

	if err := database.DeleteIMSApplicationServer(ctx, "mcptt"); !errors.Is(err, db.ErrInUse) {
		t.Fatalf("delete an application server with services: err = %v, want ErrInUse", err)
	}

	if err := database.DeleteIMSService(ctx, "stand-in"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if err := database.DeleteIMSService(ctx, "stand-in"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("delete again: err = %v, want ErrNotFound", err)
	}
}

func TestProfileIMSServices(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	as := &db.IMSApplicationServer{Name: "as", ServerName: "sip:as.example.org"}
	if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
		t.Fatalf("create application server: %v", err)
	}

	a := &db.IMSService{Name: "a", Kind: db.IMSServiceKindMCPTT, ApplicationServerID: as.ID}
	b := &db.IMSService{Name: "b", Kind: db.IMSServiceKindCustom, ApplicationServerID: as.ID, InitialFilterCriteria: []db.IMSInitialFilterCriterion{registerIFC()}}

	for _, svc := range []*db.IMSService{a, b} {
		if err := database.CreateIMSService(ctx, svc); err != nil {
			t.Fatalf("create service %s: %v", svc.Name, err)
		}
	}

	profile := &db.Profile{Name: "mc", UeAmbrUplink: "1 Gbps", UeAmbrDownlink: "1 Gbps", Allow4G: true, Allow5G: true}
	if err := database.CreateProfile(ctx, profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	if err := database.SetProfileIMSServices(ctx, profile.ID, []string{b.ID, a.ID}); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := database.ListProfileIMSServices(ctx, profile.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if !reflect.DeepEqual(got, []db.IMSService{*b, *a}) {
		t.Fatalf("list = %+v, want b then a", got)
	}

	if err := database.SetProfileIMSServices(ctx, profile.ID, []string{a.ID, b.ID}); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	if got, _ := database.ListProfileIMSServices(ctx, profile.ID); len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("list after reorder = %+v, want a then b", got)
	}

	if err := database.SetProfileIMSServices(ctx, profile.ID, []string{a.ID, a.ID}); err == nil {
		t.Fatal("a service listed twice was accepted")
	}

	if err := database.SetProfileIMSServices(ctx, profile.ID, []string{"missing"}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("set with an unknown service: err = %v, want ErrNotFound", err)
	}

	if got, _ := database.ListProfileIMSServices(ctx, profile.ID); len(got) != 2 {
		t.Fatalf("a failed set changed the list: %+v", got)
	}

	if err := database.SetProfileIMSServices(ctx, "missing", []string{a.ID}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("set on an unknown profile: err = %v, want ErrNotFound", err)
	}

	if err := database.DeleteIMSService(ctx, "a"); !errors.Is(err, db.ErrInUse) {
		t.Fatalf("delete an assigned service: err = %v, want ErrInUse", err)
	}

	if err := database.DeleteProfile(ctx, "mc"); err != nil {
		t.Fatalf("delete profile: %v", err)
	}

	if err := database.DeleteIMSService(ctx, "a"); err != nil {
		t.Fatalf("delete a service whose profile was deleted: %v", err)
	}
}

func TestIMSServiceValidate(t *testing.T) {
	spt := func(mutate func(*db.IMSServicePointTrigger)) db.IMSService {
		s := db.IMSServicePointTrigger{Groups: []int32{0}, Method: ptr("INVITE")}
		mutate(&s)

		return db.IMSService{
			Name:                "s",
			Kind:                db.IMSServiceKindCustom,
			ApplicationServerID: "as",
			InitialFilterCriteria: []db.IMSInitialFilterCriterion{{
				TriggerPoint:    db.IMSTriggerPoint{ServicePointTriggers: []db.IMSServicePointTrigger{s}},
				DefaultHandling: db.IMSDefaultHandlingSessionContinued,
			}},
		}
	}

	cases := map[string]db.IMSService{
		"no name":                {Kind: db.IMSServiceKindMCPTT, ApplicationServerID: "as"},
		"no application server":  {Name: "s", Kind: db.IMSServiceKindMCPTT},
		"unknown kind":           {Name: "s", Kind: "mmtel", ApplicationServerID: "as"},
		"mcptt with iFCs":        {Name: "s", Kind: db.IMSServiceKindMCPTT, ApplicationServerID: "as", InitialFilterCriteria: []db.IMSInitialFilterCriterion{registerIFC()}},
		"custom without iFCs":    {Name: "s", Kind: db.IMSServiceKindCustom, ApplicationServerID: "as"},
		"no group":               spt(func(s *db.IMSServicePointTrigger) { s.Groups = nil }),
		"negative group":         spt(func(s *db.IMSServicePointTrigger) { s.Groups = []int32{-1} }),
		"no condition":           spt(func(s *db.IMSServicePointTrigger) { s.Method = nil }),
		"two conditions":         spt(func(s *db.IMSServicePointTrigger) { s.RequestURI = ptr("sip:a@example.org") }),
		"empty method":           spt(func(s *db.IMSServicePointTrigger) { s.Method = ptr("") }),
		"empty header":           spt(func(s *db.IMSServicePointTrigger) { s.Method = nil; s.SIPHeader = &db.IMSHeaderMatch{} }),
		"empty sdp line":         spt(func(s *db.IMSServicePointTrigger) { s.Method = nil; s.SessionDescription = &db.IMSSessionDescription{} }),
		"empty request URI":      spt(func(s *db.IMSServicePointTrigger) { s.Method = nil; s.RequestURI = ptr("") }),
		"request URI not an ERE": spt(func(s *db.IMSServicePointTrigger) { s.Method = nil; s.RequestURI = ptr("([") }),
		"header content not an ERE": spt(func(s *db.IMSServicePointTrigger) {
			s.Method = nil
			s.SIPHeader = &db.IMSHeaderMatch{Header: "P-Asserted-Service", Content: ptr("(")}
		}),
		"control character in method": spt(func(s *db.IMSServicePointTrigger) { s.Method = ptr("INV\x01ITE") }),
		"unknown session case": spt(func(s *db.IMSServicePointTrigger) {
			s.Method = nil
			s.SessionCase = ptr(db.IMSSessionCase("ORIGINATING"))
		}),
		"registration type INVITE": spt(func(s *db.IMSServicePointTrigger) {
			s.RegistrationTypes = []db.IMSRegistrationType{db.IMSRegistrationTypeInitial}
		}),
		"unknown registration type": spt(func(s *db.IMSServicePointTrigger) {
			s.Method = ptr("REGISTER")
			s.RegistrationTypes = []db.IMSRegistrationType{"RE_REGISTRATION"}
		}),
		"registration type twice": spt(func(s *db.IMSServicePointTrigger) {
			s.Method = ptr("REGISTER")
			s.RegistrationTypes = []db.IMSRegistrationType{db.IMSRegistrationTypeInitial, db.IMSRegistrationTypeInitial}
		}),
		"three registration types": spt(func(s *db.IMSServicePointTrigger) {
			s.Method = ptr("REGISTER")
			s.RegistrationTypes = []db.IMSRegistrationType{db.IMSRegistrationTypeInitial, db.IMSRegistrationTypeReRegistration, db.IMSRegistrationTypeDeregistration}
		}),
	}

	noSPT := spt(func(*db.IMSServicePointTrigger) {})
	noSPT.InitialFilterCriteria[0].TriggerPoint.ServicePointTriggers = nil
	cases["no service point trigger"] = noSPT

	badHandling := spt(func(*db.IMSServicePointTrigger) {})
	badHandling.InitialFilterCriteria[0].DefaultHandling = "CONTINUE"
	cases["unknown default handling"] = badHandling

	for name, svc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := svc.Validate(); err == nil {
				t.Fatalf("Validate accepted %+v", svc)
			}
		})
	}

	ok := db.IMSService{Name: "s", Kind: db.IMSServiceKindCustom, ApplicationServerID: "as", InitialFilterCriteria: []db.IMSInitialFilterCriterion{registerIFC(), originatingIFC()}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate rejected the MCPTT iFCs: %v", err)
	}
}

func TestIMSApplicationServerCanonicalForms(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	as := &db.IMSApplicationServer{
		Name:                    "as",
		ServerName:              "SIP:AS.Example.org;lr",
		PublicServiceIdentities: []string{"sip:Conf@Example.org", "tel:+1-555-0100", "sip:!.*!@MCPTT.Example.org"},
	}

	if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := database.GetIMSApplicationServer(ctx, "as")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	want := []string{"sip:!.*!@mcptt.example.org", "sip:Conf@example.org", "tel:+15550100"}
	if got.ServerName != "sip:as.example.org;lr" || !reflect.DeepEqual(got.PublicServiceIdentities, want) {
		t.Fatalf("stored = %+v, want server name sip:as.example.org;lr and PSIs %v", got, want)
	}

	for _, conflicting := range []db.IMSApplicationServer{
		{Name: "same-uri", ServerName: "SIP:as.EXAMPLE.org;lr"},
		{Name: "equivalent-psi", ServerName: "sip:b.example.org", PublicServiceIdentities: []string{"tel:+15550100"}},
		{Name: "overlapping-wildcard", ServerName: "sip:c.example.org", PublicServiceIdentities: []string{"sip:grp-!.*!@mcptt.example.org"}},
	} {
		err := database.CreateIMSApplicationServer(ctx, &conflicting)
		if !errors.Is(err, db.ErrPSIConflict) && !errors.Is(err, db.ErrAlreadyExists) {
			t.Errorf("create %s: err = %v, want a conflict", conflicting.Name, err)
		}
	}

	if err := database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{
		Name: "inside-wildcard", ServerName: "sip:d.example.org", PublicServiceIdentities: []string{"sip:conf-1@mcptt.example.org"},
	}); err != nil {
		t.Fatalf("a distinct PSI inside another application server's wildcard was refused: %v", err)
	}
}

func TestIMSPublicServiceIdentityAndMSISDNExclude(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	subscriber := &db.Subscriber{
		Imsi: "001010000000001", SequenceNumber: "000000000001", PermanentKey: "6f30087629feb0b089783c81d0ae09b5",
		Opc: "21a7e1897dfb481d62439142cdf1b6ee", ProfileID: profile.ID, Msisdn: "15550100",
	}
	if err := database.CreateSubscriber(ctx, subscriber); err != nil {
		t.Fatalf("create subscriber: %v", err)
	}

	err = database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{Name: "as", ServerName: "sip:as.example.org", PublicServiceIdentities: []string{"tel:+1-555-0100"}})
	if !errors.Is(err, db.ErrPSIConflict) {
		t.Fatalf("a PSI equal to a subscriber's tel URI: err = %v, want ErrPSIConflict", err)
	}

	if err := database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{Name: "as", ServerName: "sip:as.example.org", PublicServiceIdentities: []string{"tel:+15550199"}}); err != nil {
		t.Fatalf("create: %v", err)
	}

	subscriber.Msisdn = "15550199"
	if err := database.UpdateSubscriberProfile(ctx, subscriber); !errors.Is(err, db.ErrMSISDNInUse) {
		t.Fatalf("an MSISDN equal to a PSI: err = %v, want ErrMSISDNInUse", err)
	}
}
