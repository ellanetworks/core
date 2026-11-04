// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const udmProfile = `{"nfInstanceId": "00000000-0000-0000-0000-0000000000%s1", "nfType": "UDM", "nfStatus": "REGISTERED", "fqdn": "udm-%s.5gc.mnc001.mcc001.3gppnetwork.org",
  "udmInfo": {"groupId": "%s"},
  "nfServices": [{"serviceInstanceId": "u", "serviceName": "nudm-uecm", "versions": [{"apiVersionInUri": "v1", "apiFullVersion": "1.0.0"}], "scheme": "http", "nfServiceStatus": "REGISTERED"}]}`

func udmRequest(supi string) DiscoveryRequest {
	return DiscoveryRequest{PartnerID: "p", HomePLMN: home, ServingPLMN: serving, Target: wire.NFTypeUDM, Requester: wire.NFTypeAMF, Service: wire.ServiceNudmUECM, RoutingIndicator: "0001", Supi: supi}
}

func TestDiscoveryTrustsTheNRFFilteringAndCachesPerQuery(t *testing.T) {
	f := &fakeN32{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		group := "a"
		if r.URL.Query().Get("supi") == "imsi-001010000000600" || r.URL.Query().Get("group-id-list") == "b" {
			group = "b"
		}

		_, _ = fmt.Fprintf(w, `{"validityPeriod": 60, "nfInstances": [`+udmProfile+`]}`, group, group, group)
	})}

	d := NewDiscovery(f)

	c, err := d.Discover(context.Background(), udmRequest("imsi-001010000000600"))
	if err != nil || c.GroupID != "b" {
		t.Fatalf("candidate %+v, %v", c, err)
	}

	if got := f.last.Load().URL.Query().Get("supi"); got != "imsi-001010000000600" {
		t.Fatalf("discovery without the SUPI: %q", got)
	}

	c, err = d.Discover(context.Background(), udmRequest("imsi-001010000000007"))
	if err != nil || c.GroupID != "a" || f.calls.Load() != 2 {
		t.Fatalf("another SUPI: %+v, %v, %d calls", c, err, f.calls.Load())
	}

	pinned := udmRequest("imsi-001010000000600")
	pinned.Pinned = "00000000-0000-0000-0000-0000000000b1"

	if c, err = d.Discover(context.Background(), pinned); err != nil || c.NfInstanceID != pinned.Pinned || f.calls.Load() != 2 {
		t.Fatalf("pinned: %+v, %v, %d calls", c, err, f.calls.Load())
	}

	grouped := udmRequest("imsi-001010000000600")
	grouped.GroupID = "b"

	if _, err := d.Discover(context.Background(), grouped); err != nil || f.calls.Load() != 3 {
		t.Fatalf("grouped: %v, %d calls", err, f.calls.Load())
	}

	if got := f.last.Load().URL.Query().Get("group-id-list"); got != "b" {
		t.Fatalf("discovery without the group: %q", got)
	}
}

func TestDiscoveryDerivesTheNRFFromTheHomePLMN(t *testing.T) {
	f := &fakeN32{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(searchBody))
	})}

	req := ausfRequest()
	req.HomePLMN = wire.PlmnID{Mcc: "001", Mnc: "02"}

	_, _ = NewDiscovery(f).Discover(context.Background(), req)

	if host := f.last.Load().URL.Host; host != "nrf.5gc.mnc002.mcc001.3gppnetwork.org" {
		t.Fatalf("NRF %s", host)
	}
}

func TestClassifySDMChange(t *testing.T) {
	supi := "imsi-001010000000001"
	am := "https://udm.example" + wire.SDMAmDataPath(supi)
	sorPath, upuPath, nssai, root := "/sorInfo", "/upuInfo", "/nssai", ""
	sor := []byte(`{"ackInd": true, "sorMacIausf": "00112233445566778899aabbccddeeff", "countersor": "0001", "provisioningTime": "2026-11-04T10:00:00Z"}`)

	for name, tc := range map[string]struct {
		items []wire.NotifyItem
		want  SDMChange
	}{
		"sor only":      {[]wire.NotifyItem{{ResourceID: am, Changes: []wire.ChangeItem{{Op: wire.ChangeReplace, Path: &sorPath, NewValue: sor}}}}, SDMChange{SoR: &wire.SorInfo{}}},
		"upu":           {[]wire.NotifyItem{{ResourceID: am, Changes: []wire.ChangeItem{{Op: wire.ChangeAdd, Path: &upuPath}}}}, SDMChange{UPU: true}},
		"slices":        {[]wire.NotifyItem{{ResourceID: am, Changes: []wire.ChangeItem{{Op: wire.ChangeReplace, Path: &nssai}}}}, SDMChange{Refresh: true}},
		"whole am-data": {[]wire.NotifyItem{{ResourceID: wire.SDMAmDataPath(supi), Changes: []wire.ChangeItem{{Op: wire.ChangeReplace, Path: &root, NewValue: []byte(`{"sorInfo": ` + string(sor) + `}`)}}}}, SDMChange{SoR: &wire.SorInfo{}, Refresh: true}},
		"smf-select":    {[]wire.NotifyItem{{ResourceID: wire.SDMSmfSelectDataPath(supi), Changes: []wire.ChangeItem{{Op: wire.ChangeReplace, Path: &nssai}}}}, SDMChange{Refresh: true}},
		"no items":      {nil, SDMChange{Refresh: true}},
	} {
		got := ClassifySDMChange(supi, wire.ModificationNotification{NotifyItems: tc.items})
		if (got.SoR != nil) != (tc.want.SoR != nil) || got.UPU != tc.want.UPU || got.Refresh != tc.want.Refresh {
			t.Errorf("%s: %+v, want %+v", name, got, tc.want)
		}
	}
}
