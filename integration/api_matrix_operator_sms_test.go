// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

type smsState struct {
	SMS   client.GetOperatorSMSResponse
	Peers []client.SMSCPeer
}

func readSMSState(ctx context.Context, t *testing.T, c *client.Client) smsState {
	t.Helper()

	op, err := c.GetOperator(ctx)
	if err != nil {
		t.Fatalf("get operator: %v", err)
	}

	list, err := c.ListSMSCPeers(ctx)
	if err != nil {
		t.Fatalf("list SMSC peers: %v", err)
	}

	peers := make([]client.SMSCPeer, 0, len(list.Items))

	for _, p := range list.Items {
		p.ID = ""
		p.Status = nil
		peers = append(peers, p)
	}

	return smsState{SMS: op.SMS, Peers: peers}
}

func cleanupSMS(ctx context.Context, t *testing.T, c *client.Client) {
	if err := c.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{}); err != nil {
		t.Logf("cleanup: disable sms: %v", err)
	}

	list, err := c.ListSMSCPeers(ctx)
	if err != nil {
		t.Logf("cleanup: list SMSC peers: %v", err)
		return
	}

	for _, p := range list.Items {
		if err := c.DeleteSMSCPeer(ctx, p.ID); err != nil {
			t.Logf("cleanup: delete SMSC peer %s: %v", p.ID, err)
		}
	}
}

func runOperatorSMSMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	t.Cleanup(func() { cleanupSMS(ctx, t, c) })

	var peerID string

	first := []client.SMSCPeer{{Address: "192.0.2.10", Port: 3868, ServiceCentres: []string{"+15550000000"}}}
	updated := []client.SMSCPeer{{Address: "2001:db8::10", Port: 3869, DiameterIdentity: "smsc.example.org", ServiceCentres: []string{"+15550000001"}}}

	cases := []struct {
		name  string
		apply func() error
		want  smsState
	}{
		{
			name: "enable",
			apply: func() error {
				return c.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{Enabled: true, SMSNumber: "+15550001111"})
			},
			want: smsState{SMS: client.GetOperatorSMSResponse{Enabled: true, SMSNumber: "+15550001111"}, Peers: []client.SMSCPeer{}},
		},
		{
			name: "add_SMSC",
			apply: func() error {
				peer, err := c.CreateSMSCPeer(ctx, &client.SMSCPeerOptions{Address: "192.0.2.10", ServiceCentres: []string{"+15550000000"}})
				if err == nil {
					peerID = peer.ID
				}

				return err
			},
			want: smsState{SMS: client.GetOperatorSMSResponse{Enabled: true, SMSNumber: "+15550001111"}, Peers: first},
		},
		{
			name: "update_SMSC",
			apply: func() error {
				return c.UpdateSMSCPeer(ctx, peerID, &client.SMSCPeerOptions{Address: "2001:db8::10", Port: 3869, DiameterIdentity: "smsc.example.org", ServiceCentres: []string{"+15550000001"}})
			},
			want: smsState{SMS: client.GetOperatorSMSResponse{Enabled: true, SMSNumber: "+15550001111"}, Peers: updated},
		},
		{
			name: "disable",
			apply: func() error {
				return c.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{SMSNumber: "+15550001111"})
			},
			want: smsState{SMS: client.GetOperatorSMSResponse{SMSNumber: "+15550001111"}, Peers: updated},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.apply(); err != nil {
				t.Fatalf("update sms: %v", err)
			}

			if got := readSMSState(ctx, t, c); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("sms: got %+v, want %+v", got, tc.want)
			}

			assertSMSCPeer(ctx, t, c, tc.want)
		})
	}
}

func assertSMSCPeer(ctx context.Context, t *testing.T, c *client.Client, want smsState) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for {
		status, err := c.GetDiameterStatus(ctx)
		if err != nil {
			t.Fatalf("get diameter status: %v", err)
		}

		if status.Host == "" || status.Realm == "" {
			t.Fatalf("diameter identity missing: %+v", status)
		}

		matched := len(status.Peers) == 0 && (!want.SMS.Enabled || len(want.Peers) == 0)

		if want.SMS.Enabled && len(status.Peers) == 1 && len(want.Peers) == 1 {
			p, w := status.Peers[0], want.Peers[0]
			matched = p.Role == "smsc" && p.Address == w.Address && p.Port == w.Port && p.State != "open"
		}

		if matched {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("diameter peers = %+v, want the SMSC peers %+v (enabled=%v)", status.Peers, want.Peers, want.SMS.Enabled)
		}

		time.Sleep(200 * time.Millisecond)
	}
}
