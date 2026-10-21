// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"testing"
)

func TestIMSDataNetworkAttribute(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL
	yes, no := true, false

	for _, dn := range []CreateDataNetworkParams{
		{Name: "ims", MTU: 1400, IPv4Pool: "10.60.0.0/24", DNS: "8.8.8.8"},
		{Name: "mcx", MTU: 1400, IPv4Pool: "10.61.0.0/24", DNS: "8.8.8.8", IMS: &yes},
		{Name: "enterprise", MTU: 1400, IPv4Pool: "10.62.0.0/24", DNS: "8.8.8.8"},
	} {
		if status, resp, err := createDataNetwork(url, client, token, &dn); err != nil || status != http.StatusCreated {
			t.Fatalf("create data network %s: status %d, response %+v, err %v", dn.Name, status, resp, err)
		}
	}

	for name, want := range map[string]bool{"ims": true, "mcx": true, "enterprise": false} {
		status, resp, err := getDataNetwork(url, client, token, name)
		if err != nil || status != http.StatusOK || resp.Result.IMS != want {
			t.Fatalf("get %s: status %d, ims %t, err %v; want ims %t", name, status, resp.Result.IMS, err, want)
		}
	}

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{Name: "mc", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	policy := func(name, dn string, fiveQI int32) *CreatePolicyParams {
		return &CreatePolicyParams{
			Name: name, ProfileName: "mc", SliceName: DefaultSliceName, DataNetworkName: dn,
			SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: fiveQI, Arp: 3,
		}
	}

	status, resp, err := createPolicy(url, client, token, policy("mcx-9", "mcx", 9))
	if err != nil || status != http.StatusBadRequest || resp.Error != "5QI 9 is not valid on IMS data network mcx; IMS signalling uses 5QI 5, or 69 for mission critical services" {
		t.Fatalf("create on mcx with 5QI 9: status %d, response %+v, err %v", status, resp, err)
	}

	if status, resp, err := createPolicy(url, client, token, policy("mcx", "mcx", 69)); err != nil || status != http.StatusCreated {
		t.Fatalf("create on mcx with 5QI 69: status %d, response %+v, err %v", status, resp, err)
	}

	if status, resp, err := createPolicy(url, client, token, policy("enterprise", "enterprise", 9)); err != nil || status != http.StatusCreated {
		t.Fatalf("create on enterprise with 5QI 9: status %d, response %+v, err %v", status, resp, err)
	}

	update := &UpdateDataNetworkParams{IPv4Pool: "10.61.0.0/24", DNS: "8.8.8.8", MTU: 1300}
	if status, resp, err := editDataNetwork(url, client, "mcx", token, update); err != nil || status != http.StatusOK {
		t.Fatalf("update mcx without ims: status %d, response %+v, err %v", status, resp, err)
	}

	if _, resp, err := getDataNetwork(url, client, token, "mcx"); err != nil || !resp.Result.IMS || resp.Result.MTU != 1300 {
		t.Fatalf("mcx after an update without ims = %+v, err %v; want IMS kept", resp.Result, err)
	}

	enable := &UpdateDataNetworkParams{IPv4Pool: "10.62.0.0/24", DNS: "8.8.8.8", MTU: 1400, IMS: &yes}

	status, editResp, err := editDataNetwork(url, client, "enterprise", token, enable)
	if err != nil || status != http.StatusBadRequest || editResp.Error != "5QI 9 is not valid on IMS data network enterprise; IMS signalling uses 5QI 5, or 69 for mission critical services" {
		t.Fatalf("enable IMS with a 5QI 9 policy: status %d, response %+v, err %v", status, editResp, err)
	}

	disable := &UpdateDataNetworkParams{IPv4Pool: "10.60.0.0/24", DNS: "8.8.8.8", MTU: 1400, IMS: &no}
	if status, resp, err := editDataNetwork(url, client, "ims", token, disable); err != nil || status != http.StatusOK {
		t.Fatalf("disable IMS on ims: status %d, response %+v, err %v", status, resp, err)
	}

	if _, resp, err := getDataNetwork(url, client, token, "ims"); err != nil || resp.Result.IMS {
		t.Fatalf("ims after disabling = %+v, err %v; want not IMS", resp.Result, err)
	}
}

func TestPolicyPreemption(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{Name: "mc", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	base := func(name string) *CreatePolicyParams {
		return &CreatePolicyParams{
			Name: name, ProfileName: "mc", SliceName: DefaultSliceName, DataNetworkName: "internet",
			SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 3,
		}
	}

	if status, resp, err := createPolicy(url, client, token, base("plain")); err != nil || status != http.StatusCreated {
		t.Fatalf("create without pre-emption: status %d, response %+v, err %v", status, resp, err)
	}

	if _, resp, err := getPolicy(url, client, token, "plain"); err != nil || resp.Result.PreemptionCapability != "not_preempt" || resp.Result.PreemptionVulnerability != "not_preemptable" {
		t.Fatalf("default pre-emption = %+v, err %v", resp.Result, err)
	}

	bad := base("bad")
	bad.PreemptionCapability = "shall_not_trigger"

	status, resp, err := createPolicy(url, client, token, bad)
	if err != nil || status != http.StatusBadRequest || resp.Error != `invalid preemption_capability - must be "may_preempt" or "not_preempt"` {
		t.Fatalf("create with an unknown capability: status %d, response %+v, err %v", status, resp, err)
	}

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{Name: "mc2", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	prioritized := base("prioritized")
	prioritized.ProfileName = "mc2"
	prioritized.PreemptionCapability = "may_preempt"
	prioritized.PreemptionVulnerability = "preemptable"

	if status, resp, err := createPolicy(url, client, token, prioritized); err != nil || status != http.StatusCreated {
		t.Fatalf("create with pre-emption: status %d, response %+v, err %v", status, resp, err)
	}

	update := &UpdatePolicyParams{
		ProfileName: "mc2", SliceName: DefaultSliceName, DataNetworkName: "internet",
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 2,
	}

	if status, resp, err := editPolicy(url, client, "prioritized", token, update); err != nil || status != http.StatusOK {
		t.Fatalf("update without pre-emption: status %d, response %+v, err %v", status, resp, err)
	}

	if _, resp, err := getPolicy(url, client, token, "prioritized"); err != nil || resp.Result.Arp != 2 || resp.Result.PreemptionCapability != "may_preempt" || resp.Result.PreemptionVulnerability != "preemptable" {
		t.Fatalf("pre-emption after an update without it = %+v, err %v; want it kept", resp.Result, err)
	}

	update.PreemptionVulnerability = "not_preemptable"
	if status, resp, err := editPolicy(url, client, "prioritized", token, update); err != nil || status != http.StatusOK {
		t.Fatalf("update vulnerability: status %d, response %+v, err %v", status, resp, err)
	}

	if _, resp, err := getPolicy(url, client, token, "prioritized"); err != nil || resp.Result.PreemptionCapability != "may_preempt" || resp.Result.PreemptionVulnerability != "not_preemptable" {
		t.Fatalf("pre-emption after updating vulnerability = %+v, err %v", resp.Result, err)
	}
}
