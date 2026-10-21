// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/hss"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata")

const mcpttServerName = "sip:mcptt.ims.mnc001.mcc001.3gppnetwork.org"

func mcpttFilterCriteria() []cx.InitialFilterCriteria {
	svc := db.IMSService{Name: "mcptt", Kind: db.IMSServiceKindMCPTT}

	var out []cx.InitialFilterCriteria

	for i, ifc := range svc.FilterCriteria() {
		out = append(out, ifc.CX(int32(i), mcpttServerName))
	}

	return out
}

func TestServerAssignmentCarriesTheProfileFilterCriteria(t *testing.T) {
	store := newFakeIMSStore()
	store.subscribers[testIMSI].FilterCriteria = mcpttFilterCriteria()

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
	terminating := cx.SessionCaseTerminatingUnregistered

	for _, tc := range []struct {
		name    string
		negated bool
		want    tgpp.Result
	}{
		{name: "terminating unregistered iFC", want: tgpp.Experimental(tgpp.ResultUnregisteredService)},
		{name: "negated terminating unregistered iFC", negated: true, want: tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			store.subscribers[testIMSI].FilterCriteria = []cx.InitialFilterCriteria{{
				Trigger:           &cx.TriggerPoint{ServicePointTriggers: []cx.ServicePointTrigger{{Negated: tc.negated, Groups: []int32{0}, SessionCase: &terminating}}},
				ApplicationServer: cx.ApplicationServer{ServerName: "sip:voicemail.example.org"},
			}}

			requireResult(t, locationInfo(t, store, cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN}), tc.want)
		})
	}
}

func TestPushProfile(t *testing.T) {
	store := newFakeIMSStore()

	if err := newHSS(store).PushProfile(context.Background(), testIMSI); err != nil {
		t.Fatalf("PushProfile without a registration: %v, want nothing to push", err)
	}

	store.registrations[testIMSI] = hss.Registration{State: hss.Unregistered, ServerName: testSCSCF, OriginHost: cscfIdentity.OriginHost, OriginRealm: cscfIdentity.OriginRealm}

	if err := newHSS(store).PushProfile(context.Background(), testIMSI); !errors.Is(err, hss.ErrDiameterUnavailable) {
		t.Fatalf("PushProfile to an unregistered user's S-CSCF: %v, want %v", err, hss.ErrDiameterUnavailable)
	}
}
