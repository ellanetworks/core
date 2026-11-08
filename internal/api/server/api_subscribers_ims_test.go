// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/hss"
)

type fakeIMSSubscribers struct {
	subscription  *hss.IMSSubscription
	err           error
	deregisterErr error
	deregistered  []string
}

func (f *fakeIMSSubscribers) Deregister(_ context.Context, imsi string) error {
	f.deregistered = append(f.deregistered, imsi)

	return f.deregisterErr
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
			{Identity: "tel:+15551230001", RegistrationSet: hss.MMTel, UserState: hss.UserRegistered},
			{Identity: "sip:" + Imsi + "@ims.mnc001.mcc001.3gppnetwork.org", Barred: true, RegistrationSet: hss.MMTel, UserState: hss.UserRegistered},
			{Identity: "sip:mcptt-15551230001@ims.mnc001.mcc001.3gppnetwork.org", RegistrationSet: hss.MCPTT, UserState: hss.UserNotRegistered},
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
			{Identity: "tel:+15551230001", RegistrationSet: "mmtel", UserState: "registered"},
			{Identity: "sip:" + Imsi + "@ims.mnc001.mcc001.3gppnetwork.org", Barred: true, RegistrationSet: "mmtel", UserState: "registered"},
			{Identity: "sip:mcptt-15551230001@ims.mnc001.mcc001.3gppnetwork.org", RegistrationSet: "mcptt", UserState: "not_registered"},
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

func TestGetSubscriberShowsTheRegisteredIPSMGW(t *testing.T) {
	env, client, token := newIMSTestEnv(t, &fakeIMSSubscribers{})

	if err := env.DB.PutIPSMGWRegistration(context.Background(), db.IPSMGWRegistration{
		IMSI: Imsi, Number: "15550000100", DiameterHost: "ims.ims.mnc001.mcc001.3gppnetwork.org", DiameterRealm: "ims.mnc001.mcc001.3gppnetwork.org",
	}); err != nil {
		t.Fatalf("register IP-SM-GW: %v", err)
	}

	status, resp, err := getSubscriber(env.Server.URL, client, token, Imsi)
	if err != nil || status != http.StatusOK {
		t.Fatalf("get subscriber: status %d, err %v", status, err)
	}

	want := &IPSMGW{Number: "+15550000100", DiameterHost: "ims.ims.mnc001.mcc001.3gppnetwork.org", DiameterRealm: "ims.mnc001.mcc001.3gppnetwork.org"}
	if !reflect.DeepEqual(resp.Result.IPSMGW, want) {
		t.Fatalf("ip_sm_gw = %+v, want %+v", resp.Result.IPSMGW, want)
	}
}

func TestGetSubscriberWithoutAnIPSMGW(t *testing.T) {
	env, client, token := newIMSTestEnv(t, &fakeIMSSubscribers{})

	status, resp, err := getSubscriber(env.Server.URL, client, token, Imsi)
	if err != nil || status != http.StatusOK || resp.Result.IPSMGW != nil {
		t.Fatalf("get subscriber: status %d, ip_sm_gw %+v, err %v", status, resp.Result.IPSMGW, err)
	}
}

type DeregisterIMSResponse struct {
	Result struct {
		Message      string `json:"message"`
		Acknowledged bool   `json:"acknowledged"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func deregisterSubscriberIMS(url string, client *http.Client, token string, imsi string) (int, *DeregisterIMSResponse, error) {
	return apiDo[DeregisterIMSResponse](client, "POST", url+"/api/v1/subscribers/"+imsi+"/ims/deregister", token, nil)
}

func TestDeregisterSubscriberIMS(t *testing.T) {
	for _, tc := range []struct {
		name        string
		err         error
		imsi        string
		wantStatus  int
		wantMessage string
		wantAck     bool
		wantCalled  bool
	}{
		{name: "acknowledged", imsi: Imsi, wantStatus: http.StatusOK, wantMessage: "IMS registration cleared", wantAck: true, wantCalled: true},
		{
			name: "unacknowledged", imsi: Imsi, err: fmt.Errorf("%w: no route", hss.ErrTerminationUnacknowledged), wantStatus: http.StatusOK,
			wantMessage: "IMS registration cleared; the S-CSCF did not acknowledge the de-registration", wantCalled: true,
		},
		{name: "not cleared", imsi: Imsi, err: errors.New("commit failed"), wantStatus: http.StatusInternalServerError, wantCalled: true},
		{name: "unknown subscriber", imsi: "001019999999999", wantStatus: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ims := &fakeIMSSubscribers{deregisterErr: tc.err}
			env, client, token := newIMSTestEnv(t, ims)

			status, resp, err := deregisterSubscriberIMS(env.Server.URL, client, token, tc.imsi)
			if err != nil || status != tc.wantStatus {
				t.Fatalf("deregister: status %d, want %d, err %v", status, tc.wantStatus, err)
			}

			if tc.wantMessage != "" && (resp.Result.Message != tc.wantMessage || resp.Result.Acknowledged != tc.wantAck) {
				t.Fatalf("result = %+v, want message %q, acknowledged %t", resp.Result, tc.wantMessage, tc.wantAck)
			}

			if called := len(ims.deregistered) == 1 && ims.deregistered[0] == tc.imsi; called != tc.wantCalled {
				t.Fatalf("deregistered = %v, want called %t", ims.deregistered, tc.wantCalled)
			}
		})
	}
}
