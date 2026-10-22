// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

type imsState struct {
	ApplicationServers    []client.IMSApplicationServer
	InitialFilterCriteria []client.IMSInitialFilterCriterion
}

func readIMSState(ctx context.Context, t *testing.T, c *client.Client, profile string) imsState {
	t.Helper()

	servers, err := c.ListIMSApplicationServers(ctx)
	if err != nil {
		t.Fatalf("list IMS application servers: %v", err)
	}

	ifcs, err := c.GetProfileIMSFilterCriteria(ctx, profile)
	if err != nil {
		t.Fatalf("get the initial filter criteria of profile %s: %v", profile, err)
	}

	state := imsState{ApplicationServers: []client.IMSApplicationServer{}, InitialFilterCriteria: ifcs.InitialFilterCriteria}

	for _, as := range servers.Items {
		if strings.HasPrefix(as.Name, apiMatrixName("")) {
			state.ApplicationServers = append(state.ApplicationServers, as)
		}
	}

	return state
}

func cleanupIMS(ctx context.Context, t *testing.T, c *client.Client, profile string) {
	if err := c.SetProfileIMSFilterCriteria(ctx, profile, nil); err != nil {
		t.Logf("cleanup: clear the initial filter criteria of profile %s: %v", profile, err)
	}

	if err := c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: profile}); err != nil {
		t.Logf("cleanup: delete profile %s: %v", profile, err)
	}

	if servers, err := c.ListIMSApplicationServers(ctx); err == nil {
		for _, as := range servers.Items {
			if strings.HasPrefix(as.Name, apiMatrixName("")) {
				if err := c.DeleteIMSApplicationServer(ctx, as.Name); err != nil {
					t.Logf("cleanup: delete IMS application server %s: %v", as.Name, err)
				}
			}
		}
	}
}

func imsMatrixMCPTTCriteria(as string) []client.IMSInitialFilterCriterion {
	register, originating, header, icsi := "REGISTER", "ORIGINATING_REGISTERED", "^P-Asserted-Service$", `^urn:urn-7:3gpp-service\.ims\.icsi\.mcptt$`

	return []client.IMSInitialFilterCriterion{
		{
			ApplicationServerName:   as,
			TriggerPoint:            &client.IMSTriggerPoint{SPT: []client.IMSServicePointTrigger{{Group: []int32{0}, Method: &register}}},
			DefaultHandling:         new("SESSION_CONTINUED"),
			IncludeRegisterRequest:  true,
			IncludeRegisterResponse: true,
		},
		{
			ApplicationServerName: as,
			TriggerPoint: &client.IMSTriggerPoint{ConditionTypeCNF: true, SPT: []client.IMSServicePointTrigger{
				{Group: []int32{0}, SessionCase: &originating},
				{Group: []int32{1}, SIPHeader: &client.IMSHeaderMatch{Header: header, Content: &icsi}},
			}},
			DefaultHandling: new("SESSION_TERMINATED"),
		},
	}
}

func imsMatrixConferenceCriterion(as, uri string) client.IMSInitialFilterCriterion {
	return client.IMSInitialFilterCriterion{
		ApplicationServerName: as,
		TriggerPoint:          &client.IMSTriggerPoint{SPT: []client.IMSServicePointTrigger{{Group: []int32{0}, RequestURI: &uri}}},
		DefaultHandling:       new("SESSION_TERMINATED"),
	}
}

func prioritized(ifcs ...client.IMSInitialFilterCriterion) []client.IMSInitialFilterCriterion {
	out := make([]client.IMSInitialFilterCriterion, len(ifcs))
	for i, ifc := range ifcs {
		ifc.Priority = i
		out[i] = ifc
	}

	return out
}

type imsMatrixStep struct {
	name   string
	writer int
	apply  func(c *client.Client) error
	want   imsState
}

func imsMatrixSteps(ctx context.Context, profile string) []imsMatrixStep {
	mcpttAS := apiMatrixName("ims-as-mcptt")
	confAS := apiMatrixName("ims-as-conf")

	asMCPTT := client.IMSApplicationServer{Name: mcpttAS, ServerName: "sip:mcptt.ims.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"}}
	asConf := client.IMSApplicationServer{Name: confAS, ServerName: "sip:conf.ims.example.org", PublicServiceIdentities: []string{}}
	asConfUpdated := client.IMSApplicationServer{Name: confAS, ServerName: "sip:conf2.ims.example.org", PublicServiceIdentities: []string{"sip:conference@ims.example.org"}}

	servers := []client.IMSApplicationServer{asConf, asMCPTT}
	serversUpdated := []client.IMSApplicationServer{asConfUpdated, asMCPTT}

	mcptt := imsMatrixMCPTTCriteria(mcpttAS)
	conf := imsMatrixConferenceCriterion(confAS, "^sip:conference@.*$")
	confUpdated := imsMatrixConferenceCriterion(confAS, "^sip:conf@.*$")
	none := []client.IMSInitialFilterCriterion{}

	return []imsMatrixStep{
		{
			name:   "create_application_servers",
			writer: 0,
			apply: func(c *client.Client) error {
				if _, err := c.CreateIMSApplicationServer(ctx, &client.CreateIMSApplicationServerOptions{Name: mcpttAS, ServerName: asMCPTT.ServerName, PublicServiceIdentities: asMCPTT.PublicServiceIdentities}); err != nil {
					return err
				}

				_, err := c.CreateIMSApplicationServer(ctx, &client.CreateIMSApplicationServerOptions{Name: confAS, ServerName: asConf.ServerName})

				return err
			},
			want: imsState{ApplicationServers: servers, InitialFilterCriteria: none},
		},
		{
			name:   "update_application_server",
			writer: 1,
			apply: func(c *client.Client) error {
				return c.UpdateIMSApplicationServer(ctx, confAS, &client.UpdateIMSApplicationServerOptions{ServerName: asConfUpdated.ServerName, PublicServiceIdentities: asConfUpdated.PublicServiceIdentities})
			},
			want: imsState{ApplicationServers: serversUpdated, InitialFilterCriteria: none},
		},
		{
			name:   "set_filter_criteria",
			writer: 2,
			apply: func(c *client.Client) error {
				return c.SetProfileIMSFilterCriteria(ctx, profile, append(append([]client.IMSInitialFilterCriterion{}, mcptt...), conf))
			},
			want: imsState{ApplicationServers: serversUpdated, InitialFilterCriteria: prioritized(append(append([]client.IMSInitialFilterCriterion{}, mcptt...), conf)...)},
		},
		{
			name:   "reorder_and_change_filter_criteria",
			writer: 0,
			apply: func(c *client.Client) error {
				return c.SetProfileIMSFilterCriteria(ctx, profile, append([]client.IMSInitialFilterCriterion{confUpdated}, mcptt...))
			},
			want: imsState{ApplicationServers: serversUpdated, InitialFilterCriteria: prioritized(append([]client.IMSInitialFilterCriterion{confUpdated}, mcptt...)...)},
		},
		{
			name:   "drop_a_filter_criterion",
			writer: 1,
			apply: func(c *client.Client) error {
				return c.SetProfileIMSFilterCriteria(ctx, profile, mcptt)
			},
			want: imsState{ApplicationServers: serversUpdated, InitialFilterCriteria: prioritized(mcptt...)},
		},
		{
			name:   "delete_application_server",
			writer: 2,
			apply: func(c *client.Client) error {
				return c.DeleteIMSApplicationServer(ctx, confAS)
			},
			want: imsState{ApplicationServers: []client.IMSApplicationServer{asMCPTT}, InitialFilterCriteria: prioritized(mcptt...)},
		},
	}
}

func createIMSMatrixProfile(ctx context.Context, t *testing.T, c *client.Client) string {
	t.Helper()

	profile := apiMatrixName("ims-profile")

	if err := c.CreateProfile(ctx, &client.CreateProfileOptions{Name: profile, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps"}); err != nil {
		t.Fatalf("create profile %s: %v", profile, err)
	}

	return profile
}

func runIMSMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	profile := createIMSMatrixProfile(ctx, t, c)

	t.Cleanup(func() { cleanupIMS(ctx, t, c, profile) })

	for _, step := range imsMatrixSteps(ctx, profile) {
		t.Run(step.name, func(t *testing.T) {
			if err := step.apply(c); err != nil {
				t.Fatalf("%s: %v", step.name, err)
			}

			if got := readIMSState(ctx, t, c, profile); !reflect.DeepEqual(got, step.want) {
				t.Fatalf("ims: got %+v, want %+v", got, step.want)
			}
		})
	}

	mcpttAS := apiMatrixName("ims-as-mcptt")

	t.Run("rejections", func(t *testing.T) {
		if _, err := c.CreateIMSApplicationServer(ctx, &client.CreateIMSApplicationServerOptions{Name: apiMatrixName("ims-as-clash"), ServerName: "sip:clash.ims.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"}}); err == nil {
			t.Fatal("an application server with a PSI hosted elsewhere was created")
		}

		if err := c.DeleteIMSApplicationServer(ctx, mcpttAS); err == nil {
			t.Fatal("an application server a profile uses was deleted")
		}

		if err := c.SetProfileIMSFilterCriteria(ctx, profile, []client.IMSInitialFilterCriterion{{ApplicationServerName: apiMatrixName("ims-as-missing")}}); err == nil {
			t.Fatal("a profile was given an initial filter criterion towards an unknown application server")
		}

		_, err := c.GetIMSApplicationServer(ctx, apiMatrixName("ims-as-missing"))
		assertNotFound(t, err, "IMS application server")
	})
}

func runIMSHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients
	profile := createIMSMatrixProfile(ctx, t, h.Leader)

	t.Cleanup(func() { cleanupIMS(ctx, t, h.Leader, profile) })

	for _, step := range imsMatrixSteps(ctx, profile) {
		t.Run(step.name, func(t *testing.T) {
			if err := step.apply(nodes[step.writer]); err != nil {
				t.Fatalf("%s on node %d: %v", step.name, step.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				if got := readIMSState(ctx, t, c, profile); !reflect.DeepEqual(got, step.want) {
					t.Fatalf("node %d ims: got %+v, want %+v", i+1, got, step.want)
				}
			}
		})
	}
}
