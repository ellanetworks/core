// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

type smsState struct {
	SMS   client.GetOperatorSMSResponse
	SMSCs []client.SMSC
}

func readSMSState(ctx context.Context, t *testing.T, c *client.Client) smsState {
	t.Helper()

	op, err := c.GetOperator(ctx)
	if err != nil {
		t.Fatalf("get operator: %v", err)
	}

	list, err := c.ListSMSCs(ctx)
	if err != nil {
		t.Fatalf("list SMSCs: %v", err)
	}

	smscs := make([]client.SMSC, 0, len(list.Items))

	for _, s := range list.Items {
		for i := range s.Nodes {
			s.Nodes[i].Status = nil
		}

		smscs = append(smscs, s)
	}

	return smsState{SMS: op.SMS, SMSCs: smscs}
}

func cleanupSMS(ctx context.Context, t *testing.T, c *client.Client) {
	if err := c.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{SMSNumber: "+15550001111"}); err != nil {
		t.Logf("cleanup: restore the SMS number: %v", err)
	}

	list, err := c.ListSMSCs(ctx)
	if err != nil {
		t.Logf("cleanup: list SMSCs: %v", err)
		return
	}

	for _, s := range list.Items {
		if err := c.DeleteSMSC(ctx, s.Name); err != nil {
			t.Logf("cleanup: delete SMSC %s: %v", s.Name, err)
		}
	}
}

func runOperatorSMSMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	t.Cleanup(func() { cleanupSMS(ctx, t, c) })

	first := []client.SMSC{{Name: "main", ServiceCentres: []string{"+15550000000"}, Nodes: []client.SMSCNode{{DiameterIdentity: "smsc-192-0-2-10.example.org", Address: "192.0.2.10", Port: 3868}}}}
	updated := []client.SMSC{{Name: "main", ServiceCentres: []string{"+15550000001"}, Nodes: []client.SMSCNode{
		{DiameterIdentity: "smsc-2001-db8--10.example.org", Address: "2001:db8::10", Port: 3869},
		{DiameterIdentity: "smsc-2001-db8--11.example.org", Address: "2001:db8::11", Port: 3868},
	}}}

	cases := []struct {
		name  string
		apply func() error
		want  smsState
	}{
		{
			name: "set_number",
			apply: func() error {
				return c.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{SMSNumber: "+15550009999"})
			},
			want: smsState{SMS: client.GetOperatorSMSResponse{SMSNumber: "+15550009999"}, SMSCs: []client.SMSC{}},
		},
		{
			name: "add_SMSC",
			apply: func() error {
				_, err := c.CreateSMSC(ctx, &client.CreateSMSCOptions{
					Name:           "main",
					ServiceCentres: []string{"+15550000000"},
					Nodes:          []client.SMSCNodeOptions{{DiameterIdentity: "smsc-192-0-2-10.example.org", Address: "192.0.2.10"}},
				})

				return err
			},
			want: smsState{SMS: client.GetOperatorSMSResponse{SMSNumber: "+15550009999"}, SMSCs: first},
		},
		{
			name: "update_SMSC",
			apply: func() error {
				return c.UpdateSMSC(ctx, "main", &client.UpdateSMSCOptions{
					ServiceCentres: []string{"+15550000001"},
					Nodes: []client.SMSCNodeOptions{
						{DiameterIdentity: "smsc-2001-db8--10.example.org", Address: "2001:db8::10", Port: 3869},
						{DiameterIdentity: "smsc-2001-db8--11.example.org", Address: "2001:db8::11"},
					},
				})
			},
			want: smsState{SMS: client.GetOperatorSMSResponse{SMSNumber: "+15550009999"}, SMSCs: updated},
		},
		{
			name: "clear_number",
			apply: func() error {
				return c.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{})
			},
			want: smsState{SMSCs: updated},
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

			assertSMSCNodes(ctx, t, c, tc.want)
		})
	}
}

func assertSMSCNodes(ctx context.Context, t *testing.T, c *client.Client, want smsState) {
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

		var nodes []client.SMSCNode
		for _, s := range want.SMSCs {
			nodes = append(nodes, s.Nodes...)
		}

		matched := len(status.Peers) == len(nodes)

		for _, w := range nodes {
			matched = matched && slices.ContainsFunc(status.Peers, func(p client.DiameterPeer) bool {
				return p.Role == "smsc" && p.Address == w.Address && p.Port == w.Port && p.State != "open"
			})
		}

		if matched {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("diameter peers = %+v, want the SMSC nodes %+v", status.Peers, nodes)
		}

		time.Sleep(200 * time.Millisecond)
	}
}
