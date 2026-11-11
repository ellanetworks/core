// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	customIMSHomeDomain  = "ims.example.com"
	homeDomainProbeSCSCF = "sip:scscf." + probeIMSPeerRealm
)

type homeDomainUE struct {
	kind string
	sub  scenarios.SubscriberSpec
	impi func(derived string) string
}

func setEllaIMSDomain(ctx context.Context, t *testing.T, domain string) {
	t.Helper()

	var current map[string]any
	if err := callIMS(ctx, http.MethodGet, "/api/v1/operator", nil, &current); err != nil {
		t.Fatalf("get the operator settings of Ella IMS: %v", err)
	}

	original, ok := current["domain"]
	if !ok {
		t.Logf("Ella IMS (%s) has no domain setting: only Ella Core is set to %s", ellaIMSImage(), domain)
		return
	}

	current["domain"] = domain

	var updated map[string]any
	if err := callIMS(ctx, http.MethodPut, "/api/v1/operator", current, &updated); err != nil {
		t.Fatalf("set the domain of Ella IMS to %s: %v", domain, err)
	}

	if updated["domain"] != domain {
		t.Fatalf("domain of Ella IMS = %v, want %q", updated["domain"], domain)
	}

	t.Cleanup(func() {
		current["domain"] = original
		if err := sendIMS(ctx, http.MethodPut, "/api/v1/operator", current); err != nil {
			t.Logf("cleanup: restore the domain of Ella IMS: %v", err)
		}
	})
}

func ellaIMSImage() string {
	if image := os.Getenv("ELLA_IMS_IMAGE"); image != "" {
		return image
	}

	return "ghcr.io/ellanetworks/ella-ims:latest"
}

func setEllaCoreIMSHomeDomain(ctx context.Context, t *testing.T, cl *client.Client, domain string) string {
	t.Helper()

	current, err := cl.GetIMSSettings(ctx)
	if err != nil {
		t.Fatalf("get IMS settings: %v", err)
	}

	if err := cl.UpdateIMSSettings(ctx, &client.UpdateIMSSettingsOptions{PCSCFAddresses: current.PCSCFAddresses, HomeDomain: domain, AdvertiseVoice: current.AdvertiseVoice, RestrictPeers: current.RestrictPeers}); err != nil {
		t.Fatalf("set the IMS home domain to %s: %v", domain, err)
	}

	t.Cleanup(func() {
		latest, err := cl.GetIMSSettings(ctx)
		if err != nil {
			t.Logf("cleanup: get IMS settings: %v", err)
			return
		}

		if err := cl.UpdateIMSSettings(ctx, &client.UpdateIMSSettingsOptions{PCSCFAddresses: latest.PCSCFAddresses, HomeDomain: current.HomeDomain, AdvertiseVoice: latest.AdvertiseVoice, RestrictPeers: latest.RestrictPeers}); err != nil {
			t.Logf("cleanup: restore the IMS home domain: %v", err)
		}
	})

	updated, err := cl.GetIMSSettings(ctx)
	if err != nil {
		t.Fatalf("get IMS settings: %v", err)
	}

	switch {
	case updated.HomeDomain != domain:
		t.Fatalf("IMS home domain = %q, want %q", updated.HomeDomain, domain)
	case updated.DerivedHomeDomain == "" || updated.DerivedHomeDomain == domain:
		t.Fatalf("derived IMS home domain = %q, want the domain derived from the PLMN", updated.DerivedHomeDomain)
	case !slices.Equal(updated.PCSCFAddresses, current.PCSCFAddresses):
		t.Fatalf("P-CSCF addresses = %v after setting the home domain, want %v", updated.PCSCFAddresses, current.PCSCFAddresses)
	}

	return updated.DerivedHomeDomain
}

func coreIMSRealm(ctx context.Context, t *testing.T, cl *client.Client) string {
	t.Helper()

	status, err := cl.GetDiameterStatus(ctx)
	if err != nil {
		t.Fatalf("get the Diameter status: %v", err)
	}

	for _, r := range status.ServedRealms {
		if strings.HasPrefix(r, "ims.mnc") {
			return r
		}
	}

	t.Fatalf("Ella Core serves no IMS realm: %v", status.ServedRealms)

	return ""
}

func runIMSHomeDomainChecks(ctx context.Context, t *testing.T, cl *client.Client, imsHost string) {
	setEllaIMSDomain(ctx, t, customIMSHomeDomain)
	derived := setEllaCoreIMSHomeDomain(ctx, t, cl, customIMSHomeDomain)

	if strings.EqualFold(derived, customIMSHomeDomain) {
		t.Fatalf("derived domain %q equals the configured domain", derived)
	}

	ues := []homeDomainUE{
		{kind: "usim", sub: scenarios.IMSSubscriber("001018950000001", "+15558950001"), impi: func(derived string) string { return "001018950000001@" + derived }},
		{kind: "isim", sub: scenarios.IMSSubscriber("001018950000002", "+15558950002"), impi: func(string) string { return "001018950000002@" + customIMSHomeDomain }},
	}

	fx := fixture.New(t, ctx, cl)
	fx.Apply(scenarios.IMSFixture(scenarios.Env{}, ues[0].sub, ues[1].sub))

	createIMSPeer(ctx, t, cl, &client.CreateAllowedIMSPeerOptions{Name: probeIMSPeerName + "-cx", Realm: probeIMSPeerRealm, Addresses: anySourceAddress})

	if err := waitForIMSPeerLink(ctx, cl, imsHost, ellaIMSPeerName); err != nil {
		t.Fatalf("Ella IMS after setting the home domain: %v", err)
	}

	realm := coreIMSRealm(ctx, t, cl)

	probe, err := dialDiameterProbeUntil(ctx, "cx."+probeIMSPeerRealm, probeIMSPeerRealm, diameter.ResultSuccess)
	if err != nil {
		t.Fatalf("Cx probe: %v", err)
	}

	defer probe.close()

	for _, ue := range ues {
		t.Run(ue.kind, func(t *testing.T) {
			if err := checkHomeDomainRegistration(probe, realm, derived, ue.impi(derived), ue.sub); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("home_domain_realm", func(t *testing.T) {
		status, err := cl.GetDiameterStatus(ctx)
		if err != nil {
			t.Fatalf("get the Diameter status: %v", err)
		}

		if !slices.Contains(status.ServedRealms, customIMSHomeDomain) {
			t.Fatalf("served realms = %v, want %s among them", status.ServedRealms, customIMSHomeDomain)
		}

		ue := ues[1]
		impu := "sip:+" + strings.TrimPrefix(ue.sub.MSISDN, "+") + "@" + customIMSHomeDomain

		uaa, err := userAuthorization(probe, customIMSHomeDomain, ue.impi(derived), impu)
		if err != nil {
			t.Fatalf("UAR to the home domain realm: %v", err)
		}

		if !uaa.Result.Experimental && uaa.Result.Code != diameter.ResultSuccess {
			t.Fatalf("UAR to the home domain realm: %s, want it answered by the HSS", uaa.Result)
		}
	})
}

func checkHomeDomainRegistration(p *diameterProbe, realm, derived, impi string, sub scenarios.SubscriberSpec) error {
	msisdn := strings.TrimPrefix(sub.MSISDN, "+")
	impu := "sip:+" + msisdn + "@" + customIMSHomeDomain
	derivedIMPU := "sip:+" + msisdn + "@" + derived

	uaa, err := userAuthorization(p, realm, impi, impu)
	if err != nil {
		return err
	}

	if !uaa.Result.Experimental || uaa.Result.Code != tgpp.ResultFirstRegistration {
		return fmt.Errorf("UAR of %s for %s: %s, want DIAMETER_FIRST_REGISTRATION", impi, impu, uaa.Result)
	}

	marReq, err := cx.NewMultimediaAuthRequest(p.envelope(realm), cx.MultimediaAuthRequest{
		PrivateIdentity: impi,
		PublicIdentity:  impu,
		ServerName:      homeDomainProbeSCSCF,
		NumberOfItems:   1,
		Scheme:          cx.SchemeDigestAKAv1MD5,
	})
	if err != nil {
		return err
	}

	marAns, err := p.request(marReq)
	if err != nil {
		return fmt.Errorf("MAR of %s: %w", impi, err)
	}

	maa, err := cx.ParseMultimediaAuthAnswer(marAns)
	if err != nil {
		return fmt.Errorf("MAA of %s: %w", impi, err)
	}

	switch {
	case maa.PrivateIdentity != impi:
		return fmt.Errorf("MAA private identity = %q, want %q echoed", maa.PrivateIdentity, impi)
	case len(maa.Items) == 0 || maa.Items[0].AKA == nil:
		return fmt.Errorf("MAA of %s has no AKA vector: %+v", impi, maa.Items)
	}

	saa, err := serverAssignment(p, realm, impi, impu, cx.AssignmentRegistration)
	if err != nil {
		return err
	}

	userData := string(saa.UserData)

	for _, want := range []string{
		impu,
		"tel:+" + msisdn,
		"sip:" + sub.IMSI + "@" + derived,
	} {
		if !strings.Contains(userData, want) {
			return fmt.Errorf("SAA user data of %s does not include %s: %s", impi, want, userData)
		}
	}

	for _, unwanted := range []string{derivedIMPU, "sip:" + sub.IMSI + "@" + customIMSHomeDomain} {
		if strings.Contains(userData, unwanted) {
			return fmt.Errorf("SAA user data of %s includes %s", impi, unwanted)
		}
	}

	if saa.PrivateIdentity != "" && saa.PrivateIdentity != impi {
		return fmt.Errorf("SAA private identity = %q, want %q echoed", saa.PrivateIdentity, impi)
	}

	uaa, err = userAuthorization(p, realm, impi, derivedIMPU)
	if err != nil {
		return err
	}

	if !uaa.Result.Experimental || uaa.Result.Code != tgpp.ResultErrorUserUnknown {
		return fmt.Errorf("UAR of %s for %s in the derived domain: %s, want DIAMETER_ERROR_USER_UNKNOWN", impi, derivedIMPU, uaa.Result)
	}

	temporaryIMPU := "sip:" + sub.IMSI + "@" + derived

	uaa, err = userAuthorization(p, realm, impi, temporaryIMPU)
	if err != nil {
		return err
	}

	if !uaa.Result.Experimental || uaa.Result.Code != tgpp.ResultSubsequentRegistration || uaa.ServerName != homeDomainProbeSCSCF {
		return fmt.Errorf("UAR of %s for %s: %s server %q, want DIAMETER_SUBSEQUENT_REGISTRATION on %s", impi, temporaryIMPU, uaa.Result, uaa.ServerName, homeDomainProbeSCSCF)
	}

	if _, err := serverAssignment(p, realm, impi, impu, cx.AssignmentUserDeregistration); err != nil {
		return err
	}

	return nil
}

func userAuthorization(p *diameterProbe, realm, impi, impu string) (cx.UserAuthorization, error) {
	req, err := cx.NewUserAuthorizationRequest(p.envelope(realm), cx.UserAuthorizationRequest{
		PrivateIdentity: impi,
		PublicIdentity:  impu,
		VisitedNetwork:  customIMSHomeDomain,
	})
	if err != nil {
		return cx.UserAuthorization{}, err
	}

	ans, err := p.request(req)
	if err != nil {
		return cx.UserAuthorization{}, fmt.Errorf("UAR of %s for %s: %w", impi, impu, err)
	}

	uaa, err := cx.ParseUserAuthorizationAnswer(ans)

	var resultErr *cx.ResultError
	if errors.As(err, &resultErr) {
		return cx.UserAuthorization{Result: resultErr.Result, Features: resultErr.Features}, nil
	}

	if err != nil {
		return cx.UserAuthorization{}, fmt.Errorf("UAA of %s for %s: %w", impi, impu, err)
	}

	return uaa, nil
}

func serverAssignment(p *diameterProbe, realm, impi, impu string, kind cx.AssignmentType) (cx.ServerAssignment, error) {
	req, err := cx.NewServerAssignmentRequest(p.envelope(realm), cx.ServerAssignmentRequest{
		PrivateIdentity:  impi,
		PublicIdentities: []string{impu},
		ServerName:       homeDomainProbeSCSCF,
		Type:             kind,
	})
	if err != nil {
		return cx.ServerAssignment{}, err
	}

	ans, err := p.request(req)
	if err != nil {
		return cx.ServerAssignment{}, fmt.Errorf("SAR %s of %s: %w", kind, impi, err)
	}

	saa, err := cx.ParseServerAssignmentAnswer(ans)
	if err != nil {
		return cx.ServerAssignment{}, fmt.Errorf("SAA %s of %s: %w", kind, impi, err)
	}

	return saa, nil
}
