// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata")

const mcpttServerName = "sip:mcptt.ims.mnc001.mcc001.3gppnetwork.org"

func mcpttIFCs() []db.IMSInitialFilterCriterion {
	register, originating := "REGISTER", db.IMSSessionCaseOriginatingRegistered
	header, content := "^P-Asserted-Service$", `^urn:urn-7:3gpp-service\.ims\.icsi\.mcptt$`

	return []db.IMSInitialFilterCriterion{
		{
			Trigger:                 &db.IMSTrigger{ConditionsMatch: db.IMSConditionsMatchAll, Groups: []db.IMSConditionGroup{{Conditions: []db.IMSCondition{{Method: &register}}}}},
			DefaultHandling:         new(db.IMSDefaultHandlingSessionContinued),
			IncludeRegisterRequest:  true,
			IncludeRegisterResponse: true,
		},
		{
			Trigger: &db.IMSTrigger{
				ConditionsMatch: db.IMSConditionsMatchAll,
				Groups: []db.IMSConditionGroup{{Conditions: []db.IMSCondition{
					{SessionCase: &originating},
					{SIPHeader: &db.IMSHeaderMatch{Header: header, Content: &content}},
				}}},
			},
			DefaultHandling: new(db.IMSDefaultHandlingSessionTerminated),
		},
	}
}

func mcpttFilterCriteria() []cx.InitialFilterCriteria {
	var out []cx.InitialFilterCriteria

	for i, ifc := range mcpttIFCs() {
		out = append(out, ifc.CX(int32(i), mcpttServerName))
	}

	return out
}

func TestServerAssignmentCarriesTheProfileFilterCriteria(t *testing.T) {
	store := newFakeIMSStore()
	store.filterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttFilterCriteria()}

	ans := serverAssignment(t, store, cx.ServerAssignmentRequest{
		PrivateIdentity: impiOf(testIMSI), PublicIdentities: []string{"tel:+" + testMSISDN}, ServerName: testSCSCF, Type: cx.AssignmentUnregisteredUser,
	})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	sa, err := cx.ParseServerAssignmentAnswer(ans)
	if err != nil {
		t.Fatalf("parse SAA: %v", err)
	}

	golden := filepath.Join("testdata", "mcptt_user_data.xml")
	if *updateGolden {
		if err := os.WriteFile(golden, sa.UserData, 0o600); err != nil {
			t.Fatalf("write %s: %v", golden, err)
		}
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read %s: %v", golden, err)
	}

	if !bytes.Equal(sa.UserData, want) {
		t.Fatalf("User-Data =\n%s\nwant (validated against CxData_Type_Rel19.xsd)\n%s", sa.UserData, want)
	}

	profile, err := cx.ParseUserData(sa.UserData)
	if err != nil {
		t.Fatalf("parse User-Data: %v", err)
	}

	if got := profile.ServiceProfiles[0].InitialFilterCriteria; !reflect.DeepEqual(got, mcpttFilterCriteria()) {
		t.Fatalf("iFCs = %+v, want %+v", got, mcpttFilterCriteria())
	}
}

func TestLocationInfoForUnregisteredTerminatingServices(t *testing.T) {
	sessionCase := func(c cx.SessionCase) *cx.SessionCase { return &c }
	method := func(m string) *string { return &m }

	notRegistered := tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered)
	unregisteredService := tgpp.Experimental(tgpp.ResultUnregisteredService)

	for _, tc := range []struct {
		name string
		ifcs []cx.InitialFilterCriteria
		want tgpp.Result
	}{
		{name: "no iFCs", want: notRegistered},
		{name: "MCPTT iFCs", ifcs: mcpttFilterCriteria(), want: notRegistered},
		{name: "unconditional iFC", ifcs: []cx.InitialFilterCriteria{{}}, want: unregisteredService},
		{name: "unconditional iFC of the registered profile part", ifcs: []cx.InitialFilterCriteria{{ProfilePart: new(cx.ProfilePartRegistered)}}, want: notRegistered},
		{name: "unconditional iFC of the unregistered profile part", ifcs: []cx.InitialFilterCriteria{{ProfilePart: new(cx.ProfilePartUnregistered)}}, want: unregisteredService},
		{
			name: "terminating unregistered session case",
			ifcs: []cx.InitialFilterCriteria{{Trigger: &cx.TriggerPoint{ServicePointTriggers: []cx.ServicePointTrigger{{Groups: []int32{0}, SessionCase: sessionCase(cx.SessionCaseTerminatingUnregistered)}}}}},
			want: unregisteredService,
		},
		{
			name: "negated terminating unregistered session case",
			ifcs: []cx.InitialFilterCriteria{{Trigger: &cx.TriggerPoint{ServicePointTriggers: []cx.ServicePointTrigger{{Negated: true, Groups: []int32{0}, SessionCase: sessionCase(cx.SessionCaseTerminatingUnregistered)}}}}},
			want: notRegistered,
		},
		{
			name: "negated originating registered session case",
			ifcs: []cx.InitialFilterCriteria{{Trigger: &cx.TriggerPoint{ServicePointTriggers: []cx.ServicePointTrigger{{Negated: true, Groups: []int32{0}, SessionCase: sessionCase(cx.SessionCaseOriginatingRegistered)}}}}},
			want: unregisteredService,
		},
		{
			name: "INVITE without a session case",
			ifcs: []cx.InitialFilterCriteria{{Trigger: &cx.TriggerPoint{ServicePointTriggers: []cx.ServicePointTrigger{{Groups: []int32{0}, Method: method("INVITE")}}}}},
			want: unregisteredService,
		},
		{
			name: "terminating registered AND INVITE",
			ifcs: []cx.InitialFilterCriteria{{Trigger: &cx.TriggerPoint{ConjunctiveNormalForm: true, ServicePointTriggers: []cx.ServicePointTrigger{
				{Groups: []int32{0}, SessionCase: sessionCase(cx.SessionCaseTerminatingRegistered)},
				{Groups: []int32{1}, Method: method("INVITE")},
			}}}},
			want: notRegistered,
		},
		{
			name: "terminating registered OR INVITE",
			ifcs: []cx.InitialFilterCriteria{{Trigger: &cx.TriggerPoint{ServicePointTriggers: []cx.ServicePointTrigger{
				{Groups: []int32{0}, SessionCase: sessionCase(cx.SessionCaseTerminatingRegistered)},
				{Groups: []int32{1}, Method: method("INVITE")},
			}}}},
			want: unregisteredService,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			store.filterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: tc.ifcs}

			requireResult(t, locationInfo(t, store, cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN}), tc.want)
		})
	}
}
