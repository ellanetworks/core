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
	ApplicationServers []client.IMSApplicationServer
	ServiceProfiles    []client.IMSServiceProfile
	Assigned           client.ProfileIMSServiceProfiles
}

func readIMSState(ctx context.Context, t *testing.T, c *client.Client, profile string) imsState {
	t.Helper()

	servers, err := c.ListIMSApplicationServers(ctx)
	if err != nil {
		t.Fatalf("list IMS application servers: %v", err)
	}

	serviceProfiles, err := c.ListIMSServiceProfiles(ctx)
	if err != nil {
		t.Fatalf("list IMS service profiles: %v", err)
	}

	assigned, err := c.GetProfileIMSServiceProfiles(ctx, profile)
	if err != nil {
		t.Fatalf("get the IMS service profiles of profile %s: %v", profile, err)
	}

	state := imsState{ApplicationServers: []client.IMSApplicationServer{}, ServiceProfiles: []client.IMSServiceProfile{}, Assigned: *assigned}

	for _, as := range servers.Items {
		if strings.HasPrefix(as.Name, apiMatrixName("")) {
			state.ApplicationServers = append(state.ApplicationServers, as)
		}
	}

	for _, sp := range serviceProfiles.Items {
		if strings.HasPrefix(sp.Name, apiMatrixName("")) {
			state.ServiceProfiles = append(state.ServiceProfiles, sp)
		}
	}

	return state
}

func cleanupIMS(ctx context.Context, t *testing.T, c *client.Client, profile string) {
	if err := c.SetProfileIMSServiceProfiles(ctx, profile, &client.ProfileIMSServiceProfiles{}); err != nil {
		t.Logf("cleanup: clear the IMS service profiles of profile %s: %v", profile, err)
	}

	if err := c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: profile}); err != nil {
		t.Logf("cleanup: delete profile %s: %v", profile, err)
	}

	if serviceProfiles, err := c.ListIMSServiceProfiles(ctx); err == nil {
		for _, sp := range serviceProfiles.Items {
			if strings.HasPrefix(sp.Name, apiMatrixName("")) {
				if err := c.DeleteIMSServiceProfile(ctx, sp.Name); err != nil {
					t.Logf("cleanup: delete IMS service profile %s: %v", sp.Name, err)
				}
			}
		}
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
			Trigger:                 &client.IMSTrigger{ConditionsMatch: "all", Groups: []client.IMSConditionGroup{{Conditions: []client.IMSCondition{{Method: &register}}}}},
			DefaultHandling:         new("SESSION_CONTINUED"),
			IncludeRegisterRequest:  true,
			IncludeRegisterResponse: true,
		},
		{
			ApplicationServerName: as,
			Trigger: &client.IMSTrigger{ConditionsMatch: "all", Groups: []client.IMSConditionGroup{{Conditions: []client.IMSCondition{
				{SessionCase: &originating},
				{SIPHeader: &client.IMSHeaderMatch{Header: header, Content: &icsi}},
			}}}},
			DefaultHandling: new("SESSION_TERMINATED"),
		},
	}
}

func imsMatrixConferenceCriterion(as, uri string) client.IMSInitialFilterCriterion {
	return client.IMSInitialFilterCriterion{
		ApplicationServerName: as,
		Trigger:               &client.IMSTrigger{ConditionsMatch: "all", Groups: []client.IMSConditionGroup{{Conditions: []client.IMSCondition{{RequestURI: &uri}}}}},
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
	sp := apiMatrixName("ims-sp-mcptt")

	asMCPTT := client.IMSApplicationServer{Name: mcpttAS, ServerName: "sip:mcptt.ims.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"}}
	asConf := client.IMSApplicationServer{Name: confAS, ServerName: "sip:conf.ims.example.org", PublicServiceIdentities: []string{}}
	asConfUpdated := client.IMSApplicationServer{Name: confAS, ServerName: "sip:conf2.ims.example.org", PublicServiceIdentities: []string{"sip:conference@ims.example.org"}}

	servers := []client.IMSApplicationServer{asConf, asMCPTT}
	serversUpdated := []client.IMSApplicationServer{asConfUpdated, asMCPTT}

	mcptt := imsMatrixMCPTTCriteria(mcpttAS)
	conf := imsMatrixConferenceCriterion(confAS, "^sip:conference@.*$")
	confUpdated := imsMatrixConferenceCriterion(confAS, "^sip:conf@.*$")
	none := []client.IMSServiceProfile{}
	unassigned := client.ProfileIMSServiceProfiles{}
	assigned := client.ProfileIMSServiceProfiles{MCPTT: &sp}

	serviceProfile := func(ifcs ...client.IMSInitialFilterCriterion) []client.IMSServiceProfile {
		return []client.IMSServiceProfile{{Name: sp, InitialFilterCriteria: prioritized(ifcs...)}}
	}

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
			want: imsState{ApplicationServers: servers, ServiceProfiles: none, Assigned: unassigned},
		},
		{
			name:   "update_application_server",
			writer: 1,
			apply: func(c *client.Client) error {
				return c.UpdateIMSApplicationServer(ctx, confAS, &client.UpdateIMSApplicationServerOptions{ServerName: asConfUpdated.ServerName, PublicServiceIdentities: asConfUpdated.PublicServiceIdentities})
			},
			want: imsState{ApplicationServers: serversUpdated, ServiceProfiles: none, Assigned: unassigned},
		},
		{
			name:   "create_service_profile",
			writer: 2,
			apply: func(c *client.Client) error {
				_, err := c.CreateIMSServiceProfile(ctx, &client.CreateIMSServiceProfileOptions{Name: sp, InitialFilterCriteria: append(append([]client.IMSInitialFilterCriterion{}, mcptt...), conf)})

				return err
			},
			want: imsState{ApplicationServers: serversUpdated, ServiceProfiles: serviceProfile(append(append([]client.IMSInitialFilterCriterion{}, mcptt...), conf)...), Assigned: unassigned},
		},
		{
			name:   "assign_service_profile",
			writer: 0,
			apply: func(c *client.Client) error {
				return c.SetProfileIMSServiceProfiles(ctx, profile, &assigned)
			},
			want: imsState{ApplicationServers: serversUpdated, ServiceProfiles: serviceProfile(append(append([]client.IMSInitialFilterCriterion{}, mcptt...), conf)...), Assigned: assigned},
		},
		{
			name:   "reorder_and_change_filter_criteria",
			writer: 1,
			apply: func(c *client.Client) error {
				return c.UpdateIMSServiceProfile(ctx, sp, &client.UpdateIMSServiceProfileOptions{InitialFilterCriteria: append([]client.IMSInitialFilterCriterion{confUpdated}, mcptt...)})
			},
			want: imsState{ApplicationServers: serversUpdated, ServiceProfiles: serviceProfile(append([]client.IMSInitialFilterCriterion{confUpdated}, mcptt...)...), Assigned: assigned},
		},
		{
			name:   "drop_a_filter_criterion",
			writer: 2,
			apply: func(c *client.Client) error {
				return c.UpdateIMSServiceProfile(ctx, sp, &client.UpdateIMSServiceProfileOptions{InitialFilterCriteria: mcptt})
			},
			want: imsState{ApplicationServers: serversUpdated, ServiceProfiles: serviceProfile(mcptt...), Assigned: assigned},
		},
		{
			name:   "delete_application_server",
			writer: 0,
			apply: func(c *client.Client) error {
				return c.DeleteIMSApplicationServer(ctx, confAS)
			},
			want: imsState{ApplicationServers: []client.IMSApplicationServer{asMCPTT}, ServiceProfiles: serviceProfile(mcptt...), Assigned: assigned},
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
			t.Fatal("an application server a service profile uses was deleted")
		}

		if err := c.DeleteIMSServiceProfile(ctx, apiMatrixName("ims-sp-mcptt")); err == nil {
			t.Fatal("a service profile a profile uses was deleted")
		}

		if err := c.UpdateIMSServiceProfile(ctx, apiMatrixName("ims-sp-mcptt"), &client.UpdateIMSServiceProfileOptions{InitialFilterCriteria: []client.IMSInitialFilterCriterion{{ApplicationServerName: apiMatrixName("ims-as-missing")}}}); err == nil {
			t.Fatal("a service profile was given an initial filter criterion towards an unknown application server")
		}

		missing := apiMatrixName("ims-sp-missing")
		if err := c.SetProfileIMSServiceProfiles(ctx, profile, &client.ProfileIMSServiceProfiles{MMTel: &missing}); err == nil {
			t.Fatal("a profile was given an unknown service profile")
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
