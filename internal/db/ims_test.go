// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/internal/db"
)

func registerIFC(asID string) db.IMSInitialFilterCriterion {
	return db.IMSInitialFilterCriterion{
		ApplicationServerID: asID,
		Trigger: &db.IMSTrigger{
			ConditionsMatch: db.IMSConditionsMatchAll,
			Groups: []db.IMSConditionGroup{{Conditions: []db.IMSCondition{
				{Method: ptr("REGISTER"), RegistrationTypes: []db.IMSRegistrationType{db.IMSRegistrationTypeInitial, db.IMSRegistrationTypeDeregistration}},
			}}},
		},
		DefaultHandling:         ptr(db.IMSDefaultHandlingSessionContinued),
		ServiceInfo:             ptr("<mcptt-info/>"),
		IncludeRegisterRequest:  true,
		IncludeRegisterResponse: true,
	}
}

func originatingIFC(asID string) db.IMSInitialFilterCriterion {
	return db.IMSInitialFilterCriterion{
		ApplicationServerID: asID,
		Trigger: &db.IMSTrigger{
			ConditionsMatch: db.IMSConditionsMatchAny,
			Groups: []db.IMSConditionGroup{
				{Conditions: []db.IMSCondition{{SessionCase: ptr(db.IMSSessionCaseOriginatingRegistered)}}},
				{Conditions: []db.IMSCondition{{SIPHeader: &db.IMSHeaderMatch{Header: "P-Asserted-Service", Content: ptr(`^urn:urn-7:3gpp-service\.ims\.icsi\.mcptt$`)}}}},
				{Conditions: []db.IMSCondition{
					{Negated: true, SessionDescription: &db.IMSSessionDescription{Line: "m", Content: ptr("video")}},
					{RequestURI: ptr(`^conf\.example\.org$`)},
				}},
			},
		},
		ProfilePart:     ptr(db.IMSProfilePartRegistered),
		DefaultHandling: ptr(db.IMSDefaultHandlingSessionTerminated),
	}
}

func unconditionalIFC(asID string) db.IMSInitialFilterCriterion {
	return db.IMSInitialFilterCriterion{ApplicationServerID: asID, ProfilePart: ptr(db.IMSProfilePartUnregistered)}
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

func TestProfileIMSFilterCriteria_TS29228_B22(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	mcptt := &db.IMSApplicationServer{Name: "mcptt", ServerName: "sip:mcptt.ims.example.org"}
	conf := &db.IMSApplicationServer{Name: "conf", ServerName: "sip:conf.ims.example.org"}

	for _, as := range []*db.IMSApplicationServer{mcptt, conf} {
		if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
			t.Fatalf("create application server %s: %v", as.Name, err)
		}
	}

	profile := &db.Profile{Name: "mc", UeAmbrUplink: "1 Gbps", UeAmbrDownlink: "1 Gbps", Allow4G: true, Allow5G: true}
	if err := database.CreateProfile(ctx, profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	if got, err := database.ListProfileIMSFilterCriteria(ctx, profile.ID); err != nil || len(got) != 0 {
		t.Fatalf("iFCs of a new profile = %+v, %v; want none", got, err)
	}

	want := []db.IMSInitialFilterCriterion{registerIFC(mcptt.ID), originatingIFC(mcptt.ID), unconditionalIFC(conf.ID)}
	if err := database.SetProfileIMSFilterCriteria(ctx, profile.ID, want); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := database.ListProfileIMSFilterCriteria(ctx, profile.ID)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("list = %+v, %v; want %+v in order", got, err, want)
	}

	reordered := []db.IMSInitialFilterCriterion{unconditionalIFC(conf.ID), registerIFC(mcptt.ID)}
	if err := database.SetProfileIMSFilterCriteria(ctx, profile.ID, reordered); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	if got, _ := database.ListProfileIMSFilterCriteria(ctx, profile.ID); !reflect.DeepEqual(got, reordered) {
		t.Fatalf("list after reorder = %+v, want %+v", got, reordered)
	}

	if err := database.SetProfileIMSFilterCriteria(ctx, profile.ID, []db.IMSInitialFilterCriterion{registerIFC("missing")}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("set with an unknown application server: err = %v, want ErrNotFound", err)
	}

	if got, _ := database.ListProfileIMSFilterCriteria(ctx, profile.ID); len(got) != 2 {
		t.Fatalf("a failed set changed the list: %+v", got)
	}

	if err := database.SetProfileIMSFilterCriteria(ctx, "missing", []db.IMSInitialFilterCriterion{registerIFC(mcptt.ID)}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("set on an unknown profile: err = %v, want ErrNotFound", err)
	}

	if profiles, err := database.ProfilesUsingIMSApplicationServer(ctx, mcptt.ID); err != nil || !reflect.DeepEqual(profiles, []string{profile.ID}) {
		t.Fatalf("profiles using mcptt = %v, %v; want %s", profiles, err, profile.ID)
	}

	if err := database.DeleteIMSApplicationServer(ctx, "mcptt"); !errors.Is(err, db.ErrInUse) {
		t.Fatalf("delete an application server a profile uses: err = %v, want ErrInUse", err)
	}

	if err := database.SetProfileIMSFilterCriteria(ctx, profile.ID, nil); err != nil {
		t.Fatalf("clear: %v", err)
	}

	if err := database.DeleteIMSApplicationServer(ctx, "mcptt"); err != nil {
		t.Fatalf("delete an unused application server: %v", err)
	}

	if err := database.SetProfileIMSFilterCriteria(ctx, profile.ID, []db.IMSInitialFilterCriterion{unconditionalIFC(conf.ID)}); err != nil {
		t.Fatalf("set: %v", err)
	}

	if err := database.DeleteProfile(ctx, "mc"); err != nil {
		t.Fatalf("delete a profile with iFCs: %v", err)
	}

	if err := database.DeleteIMSApplicationServer(ctx, "conf"); err != nil {
		t.Fatalf("delete an application server whose profile was deleted: %v", err)
	}
}

func TestIMSFilterCriteriaValidate(t *testing.T) {
	spt := func(mutate func(*db.IMSCondition)) []db.IMSInitialFilterCriterion {
		s := db.IMSCondition{Method: ptr("INVITE")}
		mutate(&s)

		return []db.IMSInitialFilterCriterion{{
			ApplicationServerID: "as",
			Trigger:             &db.IMSTrigger{ConditionsMatch: db.IMSConditionsMatchAll, Groups: []db.IMSConditionGroup{{Conditions: []db.IMSCondition{s}}}},
			DefaultHandling:     ptr(db.IMSDefaultHandlingSessionContinued),
		}}
	}

	cases := map[string][]db.IMSInitialFilterCriterion{
		"no application server":  {registerIFC("")},
		"no condition":           spt(func(s *db.IMSCondition) { s.Method = nil }),
		"two conditions":         spt(func(s *db.IMSCondition) { s.RequestURI = ptr("sip:a@example.org") }),
		"empty method":           spt(func(s *db.IMSCondition) { s.Method = ptr("") }),
		"empty header":           spt(func(s *db.IMSCondition) { s.Method = nil; s.SIPHeader = &db.IMSHeaderMatch{} }),
		"empty sdp line":         spt(func(s *db.IMSCondition) { s.Method = nil; s.SessionDescription = &db.IMSSessionDescription{} }),
		"empty request URI":      spt(func(s *db.IMSCondition) { s.Method = nil; s.RequestURI = ptr("") }),
		"request URI not an ERE": spt(func(s *db.IMSCondition) { s.Method = nil; s.RequestURI = ptr("([") }),
		"header content not an ERE": spt(func(s *db.IMSCondition) {
			s.Method = nil
			s.SIPHeader = &db.IMSHeaderMatch{Header: "P-Asserted-Service", Content: ptr("(")}
		}),
		"control character in method": spt(func(s *db.IMSCondition) { s.Method = ptr("INV\x01ITE") }),
		"unknown session case": spt(func(s *db.IMSCondition) {
			s.Method = nil
			s.SessionCase = ptr(db.IMSSessionCase("ORIGINATING"))
		}),
		"registration type INVITE": spt(func(s *db.IMSCondition) {
			s.RegistrationTypes = []db.IMSRegistrationType{db.IMSRegistrationTypeInitial}
		}),
		"unknown registration type": spt(func(s *db.IMSCondition) {
			s.Method = ptr("REGISTER")
			s.RegistrationTypes = []db.IMSRegistrationType{"RE_REGISTRATION"}
		}),
		"registration type twice": spt(func(s *db.IMSCondition) {
			s.Method = ptr("REGISTER")
			s.RegistrationTypes = []db.IMSRegistrationType{db.IMSRegistrationTypeInitial, db.IMSRegistrationTypeInitial}
		}),
		"three registration types": spt(func(s *db.IMSCondition) {
			s.Method = ptr("REGISTER")
			s.RegistrationTypes = []db.IMSRegistrationType{db.IMSRegistrationTypeInitial, db.IMSRegistrationTypeReRegistration, db.IMSRegistrationTypeDeregistration}
		}),
	}

	noGroup := spt(func(*db.IMSCondition) {})
	noGroup[0].Trigger.Groups = nil
	cases["no group"] = noGroup

	emptyGroup := spt(func(*db.IMSCondition) {})
	emptyGroup[0].Trigger.Groups = append(emptyGroup[0].Trigger.Groups, db.IMSConditionGroup{})
	cases["empty group"] = emptyGroup

	noMatch := spt(func(*db.IMSCondition) {})
	noMatch[0].Trigger.ConditionsMatch = ""
	cases["no conditions match"] = noMatch

	badMatch := spt(func(*db.IMSCondition) {})
	badMatch[0].Trigger.ConditionsMatch = "some"
	cases["unknown conditions match"] = badMatch

	badHandling := spt(func(*db.IMSCondition) {})
	badHandling[0].DefaultHandling = ptr(db.IMSDefaultHandling("CONTINUE"))
	cases["unknown default handling"] = badHandling

	badPart := spt(func(*db.IMSCondition) {})
	badPart[0].ProfilePart = ptr(db.IMSProfilePart("BOTH"))
	cases["unknown profile part"] = badPart

	badInfo := spt(func(*db.IMSCondition) {})
	badInfo[0].ServiceInfo = ptr("info\x00")
	cases["control character in service info"] = badInfo

	for name, ifcs := range cases {
		t.Run(name, func(t *testing.T) {
			if err := db.ValidateIMSFilterCriteria(ifcs); err == nil {
				t.Fatalf("Validate accepted %+v", ifcs)
			}
		})
	}

	ok := []db.IMSInitialFilterCriterion{registerIFC("as"), originatingIFC("as"), unconditionalIFC("as")}
	if err := db.ValidateIMSFilterCriteria(ok); err != nil {
		t.Fatalf("Validate rejected iFCs TS 29.228 B.2.2 allows: %v", err)
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

	for _, disjoint := range []db.IMSApplicationServer{
		{Name: "other-prefix", ServerName: "sip:e.example.org", PublicServiceIdentities: []string{"sip:chat!.*!@example.org"}},
		{Name: "other-number-range", ServerName: "sip:f.example.org", PublicServiceIdentities: []string{"tel:+1666!.*!"}},
	} {
		if err := database.CreateIMSApplicationServer(ctx, &disjoint); err != nil {
			t.Errorf("create %s: %v, want it accepted", disjoint.Name, err)
		}
	}

	if err := database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{
		Name: "overlapping-number-range", ServerName: "sip:g.example.org", PublicServiceIdentities: []string{"tel:+16!.*!"},
	}); !errors.Is(err, db.ErrPSIConflict) {
		t.Errorf("create overlapping-number-range: err = %v, want ErrPSIConflict", err)
	}

	if err := database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{
		Name: "inside-wildcard", ServerName: "sip:d.example.org", PublicServiceIdentities: []string{"sip:conf-1@mcptt.example.org"},
	}); err != nil {
		t.Fatalf("a distinct PSI inside another application server's wildcard was refused: %v", err)
	}
}

func TestIMSApplicationServersFollowOperatorID(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	if err := database.UpdateOperatorID(ctx, "001", "01"); err != nil {
		t.Fatalf("set operator ID: %v", err)
	}

	servers := []*db.IMSApplicationServer{
		{
			Name:                    "mcptt",
			ServerName:              "sip:mcptt.ims.mnc001.mcc001.3gppnetwork.org",
			PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.mnc001.mcc001.3gppnetwork.org"},
		},
		{
			Name:                    "mmtel",
			ServerName:              "sip:ims.mnc001.mcc001.3gppnetwork.org:5070;lr",
			PublicServiceIdentities: []string{"sip:conf@example.org", "sip:conf@ims.mnc001.mcc001.3gppnetwork.org", "tel:+15550100"},
		},
		{
			Name:                    "external",
			ServerName:              "sip:as.example.org",
			PublicServiceIdentities: []string{"sip:svc@example.org"},
		},
	}

	for _, as := range servers {
		if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
			t.Fatalf("create %s: %v", as.Name, err)
		}
	}

	if err := database.UpdateOperatorID(ctx, "999", "010"); err != nil {
		t.Fatalf("update operator ID: %v", err)
	}

	got, err := database.ListIMSApplicationServers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := []db.IMSApplicationServer{
		{
			ID:                      servers[2].ID,
			Name:                    "external",
			ServerName:              "sip:as.example.org",
			PublicServiceIdentities: []string{"sip:svc@example.org"},
		},
		{
			ID:                      servers[0].ID,
			Name:                    "mcptt",
			ServerName:              "sip:mcptt.ims.mnc010.mcc999.3gppnetwork.org",
			PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.mnc010.mcc999.3gppnetwork.org"},
		},
		{
			ID:                      servers[1].ID,
			Name:                    "mmtel",
			ServerName:              "sip:ims.mnc010.mcc999.3gppnetwork.org:5070;lr",
			PublicServiceIdentities: []string{"sip:conf@example.org", "sip:conf@ims.mnc010.mcc999.3gppnetwork.org", "tel:+15550100"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("servers = %+v, want %+v", got, want)
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

	err = database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{Name: "range", ServerName: "sip:range.example.org", PublicServiceIdentities: []string{"tel:+1555!01.*!"}})
	if !errors.Is(err, db.ErrPSIConflict) {
		t.Fatalf("a wildcarded PSI covering a subscriber's tel URI: err = %v, want ErrPSIConflict", err)
	}

	if err := database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{Name: "range", ServerName: "sip:range.example.org", PublicServiceIdentities: []string{"tel:+1777!.*!"}}); err != nil {
		t.Fatalf("create range: %v", err)
	}

	subscriber.Msisdn = "17770001"
	if err := database.UpdateSubscriberProfile(ctx, subscriber); !errors.Is(err, db.ErrMSISDNInUse) {
		t.Fatalf("an MSISDN inside a wildcarded PSI: err = %v, want ErrMSISDNInUse", err)
	}
}

func TestIMSProfilePushTargets(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	as := &db.IMSApplicationServer{Name: "as", ServerName: "sip:as.example.org"}
	if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
		t.Fatalf("create application server: %v", err)
	}

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	if err := database.SetProfileIMSFilterCriteria(ctx, profile.ID, []db.IMSInitialFilterCriterion{registerIFC(as.ID)}); err != nil {
		t.Fatalf("set profile iFCs: %v", err)
	}

	profiles, err := database.ProfilesUsingIMSApplicationServer(ctx, as.ID)
	if err != nil || !reflect.DeepEqual(profiles, []string{profile.ID}) {
		t.Fatalf("profiles using the application server = %v, %v; want %s", profiles, err, profile.ID)
	}

	states := map[string]db.IMSRegistrationState{"001010000000001": db.IMSRegistered, "001010000000002": db.IMSUnregistered, "001010000000003": db.IMSNotRegistered}

	for _, imsi := range []string{"001010000000001", "001010000000002", "001010000000003", "001010000000004"} {
		sub := &db.Subscriber{Imsi: imsi, SequenceNumber: "000000000001", PermanentKey: "6f30087629feb0b089783c81d0ae09b5", Opc: "21a7e1897dfb481d62439142cdf1b6ee", ProfileID: profile.ID}
		if err := database.CreateSubscriber(ctx, sub); err != nil {
			t.Fatalf("create subscriber %s: %v", imsi, err)
		}

		state, ok := states[imsi]
		if !ok {
			continue
		}

		next := &db.IMSRegistration{IMSI: imsi, State: state, ServerName: "sip:scscf.example.org", AuthPending: state == db.IMSNotRegistered, OriginHost: "scscf.example.org", OriginRealm: "example.org"}
		if _, swapped, err := database.CompareAndSwapIMSRegistration(ctx, imsi, nil, next); err != nil || !swapped {
			t.Fatalf("store IMS registration of %s: swapped %t, %v", imsi, swapped, err)
		}
	}

	imsis, err := database.IMSRegisteredSubscribersOfProfiles(ctx, []string{profile.ID})
	if err != nil || !reflect.DeepEqual(imsis, []string{"001010000000001", "001010000000002"}) {
		t.Fatalf("IMS registered subscribers = %v, %v; want the registered and the unregistered one", imsis, err)
	}
}

func TestIMSPublicServiceIdentitiesCannotTakeSubscriberIdentities(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	op, err := database.GetOperator(ctx)
	if err != nil {
		t.Fatalf("get operator: %v", err)
	}

	mnc := op.Mnc
	if len(mnc) == 2 {
		mnc = "0" + mnc
	}

	domain := "ims.mnc" + mnc + ".mcc" + op.Mcc + ".3gppnetwork.org"

	for _, identity := range []string{"sip:001010000000001@" + domain, "sip:!.*!@" + domain, "sip:![0-9]+!@" + strings.ToUpper(domain), "sip:!9.*!@" + domain, "sip:00101!.*!@" + domain} {
		err := database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{Name: "as", ServerName: "sip:as.example.org", PublicServiceIdentities: []string{identity}})
		if !errors.Is(err, db.ErrPSIConflict) {
			t.Errorf("create with PSI %s: err = %v, want ErrPSIConflict", identity, err)
		}
	}

	for _, identity := range []string{"sip:conference@" + domain, "sip:conf-!.*!@" + domain, "sip:!.*!@mcptt." + domain} {
		as := &db.IMSApplicationServer{Name: "ok", ServerName: "sip:ok.example.org", PublicServiceIdentities: []string{identity}}
		if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
			t.Errorf("create with PSI %s: %v", identity, err)
		}

		if err := database.DeleteIMSApplicationServer(ctx, "ok"); err != nil {
			t.Fatalf("delete: %v", err)
		}
	}
}

func TestIMSTriggerCX_TS29228_AnnexC(t *testing.T) {
	invite, message, subscribe := "INVITE", "MESSAGE", "SUBSCRIBE"
	from := &db.IMSHeaderMatch{Header: "From", Content: ptr(`"joe"`)}
	groups := []db.IMSConditionGroup{
		{Conditions: []db.IMSCondition{{Method: &invite}}},
		{Conditions: []db.IMSCondition{{Method: &message}}},
		{Conditions: []db.IMSCondition{{Method: &subscribe}, {Negated: true, SIPHeader: from}}},
	}
	cxFrom := &cx.HeaderMatch{Header: "From", Content: ptr(`"joe"`)}
	wantSPTs := []cx.ServicePointTrigger{
		{Groups: []int32{0}, Method: &invite},
		{Groups: []int32{1}, Method: &message},
		{Groups: []int32{2}, Method: &subscribe},
		{Groups: []int32{2}, Negated: true, SIPHeader: cxFrom},
	}

	for _, tc := range []struct {
		match   db.IMSConditionsMatch
		wantCNF bool
	}{
		{match: db.IMSConditionsMatchAll, wantCNF: false},
		{match: db.IMSConditionsMatchAny, wantCNF: true},
	} {
		t.Run(string(tc.match), func(t *testing.T) {
			got := db.IMSTrigger{ConditionsMatch: tc.match, Groups: groups}.CX()

			want := &cx.TriggerPoint{ConjunctiveNormalForm: tc.wantCNF, ServicePointTriggers: wantSPTs}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("CX() = %+v, want %+v", got, want)
			}

			ifc := cx.InitialFilterCriteria{Trigger: got, ApplicationServer: cx.ApplicationServer{ServerName: "sip:as.example.org"}}
			if err := ifc.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}
