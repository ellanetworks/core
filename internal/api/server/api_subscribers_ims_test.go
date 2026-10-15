// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/hss"
)

type fakeIMSSubscribers struct {
	subscription *hss.IMSSubscription
	err          error
}

func (f *fakeIMSSubscribers) Termination(context.Context, string) (*hss.Termination, error) {
	return nil, nil
}

func (f *fakeIMSSubscribers) Terminate(context.Context, *hss.Termination) error { return nil }

func (f *fakeIMSSubscribers) Subscription(context.Context, string) (*hss.IMSSubscription, error) {
	return f.subscription, f.err
}

func newIMSTestEnv(t *testing.T, ims *fakeIMSSubscribers) (testEnv, *http.Client, string) {
	t.Helper()

	testdb, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("open database: %s", err)
	}

	env, err := buildTestEnvWithHSS(testdb, ims)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	t.Cleanup(env.Server.Close)

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	if _, _, err := createProfile(env.Server.URL, client, token, &CreateProfileParams{Name: TestProfileName, UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	if _, _, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{Name: "ims", MTU: 1400, IPv4Pool: "10.60.0.0/24", DNS: "8.8.8.8"}); err != nil {
		t.Fatalf("create data network: %s", err)
	}

	if status, _, err := createPolicy(env.Server.URL, client, token, &CreatePolicyParams{
		Name: "voice", ProfileName: TestProfileName, SliceName: DefaultSliceName, DataNetworkName: "ims",
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 5, Arp: 15,
	}); err != nil || status != http.StatusCreated {
		t.Fatalf("create policy: status %d, err %v", status, err)
	}

	if _, _, err := createSubscriber(env.Server.URL, client, token, &CreateSubscriberParams{
		Imsi: Imsi, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: TestProfileName,
	}); err != nil {
		t.Fatalf("create subscriber: %s", err)
	}

	return env, client, token
}

func TestGetSubscriberShowsTheIMSSubscription(t *testing.T) {
	ims := &fakeIMSSubscribers{subscription: &hss.IMSSubscription{
		PrivateIdentity: Imsi + "@ims.mnc001.mcc001.3gppnetwork.org",
		SCSCFName:       "sip:scscf.ims.mnc001.mcc001.3gppnetwork.org:5080",
		PublicIdentities: []hss.PublicIdentity{
			{Identity: "tel:+15551230001", UserState: hss.UserRegistered},
			{Identity: "sip:" + Imsi + "@ims.mnc001.mcc001.3gppnetwork.org", Barred: true, UserState: hss.UserRegistered},
		},
	}}

	env, client, token := newIMSTestEnv(t, ims)

	status, resp, err := getSubscriber(env.Server.URL, client, token, Imsi)
	if err != nil || status != http.StatusOK {
		t.Fatalf("get subscriber: status %d, err %v", status, err)
	}

	want := &IMSSubscription{
		PrivateIdentity: Imsi + "@ims.mnc001.mcc001.3gppnetwork.org",
		SCSCFName:       "sip:scscf.ims.mnc001.mcc001.3gppnetwork.org:5080",
		PublicIdentities: []PublicIdentity{
			{Identity: "tel:+15551230001", UserState: "registered"},
			{Identity: "sip:" + Imsi + "@ims.mnc001.mcc001.3gppnetwork.org", Barred: true, UserState: "registered"},
		},
	}

	if !reflect.DeepEqual(resp.Result.IMS, want) {
		t.Fatalf("ims = %+v, want %+v", resp.Result.IMS, want)
	}
}

func TestGetSubscriberWithoutAnIMSSubscription(t *testing.T) {
	env, client, token := newIMSTestEnv(t, &fakeIMSSubscribers{})

	status, resp, err := getSubscriber(env.Server.URL, client, token, Imsi)
	if err != nil || status != http.StatusOK || resp.Result.IMS != nil {
		t.Fatalf("get subscriber: status %d, ims %+v, err %v", status, resp.Result.IMS, err)
	}
}

func TestGetSubscriberWhenTheIMSSubscriptionCannotBeRead(t *testing.T) {
	env, client, token := newIMSTestEnv(t, &fakeIMSSubscribers{err: errors.New("database closed")})

	status, resp, err := getSubscriber(env.Server.URL, client, token, Imsi)
	if err != nil || status != http.StatusInternalServerError || resp.Error != "Failed to retrieve the IMS subscription" {
		t.Fatalf("get subscriber: status %d, response %+v, err %v", status, resp, err)
	}
}
