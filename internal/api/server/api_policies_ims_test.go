// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"testing"
)

func TestPoliciesOnTheIMSDataNetworkUse5QI5(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL

	for _, dn := range []CreateDataNetworkParams{
		{Name: "ims", MTU: 1400, IPv4Pool: "10.60.0.0/24", DNS: "8.8.8.8"},
		{Name: "internet", MTU: 1400, IPv4Pool: "10.61.0.0/24", DNS: "8.8.8.8"},
	} {
		if _, _, err := createDataNetwork(url, client, token, &dn); err != nil {
			t.Fatalf("create data network %s: %s", dn.Name, err)
		}
	}

	for _, name := range []string{"voice", "data"} {
		if _, _, err := createProfile(url, client, token, &CreateProfileParams{Name: name, UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
			t.Fatalf("create profile %s: %s", name, err)
		}
	}

	policy := func(name, profile, dn string, fiveQI int32) *CreatePolicyParams {
		return &CreatePolicyParams{
			Name: name, ProfileName: profile, SliceName: DefaultSliceName, DataNetworkName: dn,
			SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: fiveQI, Arp: 15,
		}
	}

	update := func(p *CreatePolicyParams) *UpdatePolicyParams {
		return &UpdatePolicyParams{
			ProfileName: p.ProfileName, SliceName: p.SliceName, DataNetworkName: p.DataNetworkName,
			SessionAmbrUplink: p.SessionAmbrUplink, SessionAmbrDownlink: p.SessionAmbrDownlink, Var5qi: p.Var5qi, Arp: p.Arp,
		}
	}

	const refused = "5QI 9 is not valid on the ims data network; IMS signalling uses 5QI 5"

	status, resp, err := createPolicy(url, client, token, policy("voice-9", "voice", "ims", 9))
	if err != nil || status != http.StatusBadRequest || resp.Error != refused {
		t.Fatalf("create on ims with 5QI 9: status %d, response %+v, err %v", status, resp, err)
	}

	if status, resp, err := createPolicy(url, client, token, policy("voice", "voice", "ims", 5)); err != nil || status != http.StatusCreated {
		t.Fatalf("create on ims with 5QI 5: status %d, response %+v, err %v", status, resp, err)
	}

	status, resp, err = editPolicy(url, client, "voice", token, update(policy("voice", "voice", "ims", 9)))
	if err != nil || status != http.StatusBadRequest || resp.Error != refused {
		t.Fatalf("update on ims to 5QI 9: status %d, response %+v, err %v", status, resp, err)
	}

	if status, resp, err := createPolicy(url, client, token, policy("data", "data", "internet", 9)); err != nil || status != http.StatusCreated {
		t.Fatalf("create on internet with 5QI 9: status %d, response %+v, err %v", status, resp, err)
	}

	status, resp, err = editPolicy(url, client, "data", token, update(policy("data", "data", "ims", 9)))
	if err != nil || status != http.StatusBadRequest || resp.Error != refused {
		t.Fatalf("move a 5QI 9 policy onto ims: status %d, response %+v, err %v", status, resp, err)
	}

	if status, resp, err := editPolicy(url, client, "data", token, update(policy("data", "data", "ims", 5))); err != nil || status != http.StatusOK {
		t.Fatalf("move a policy onto ims with 5QI 5: status %d, response %+v, err %v", status, resp, err)
	}
}
