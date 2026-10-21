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

type imsServicesState struct {
	ApplicationServers []client.IMSApplicationServer
	Services           []client.IMSService
	ProfileServices    []string
}

func readIMSServicesState(ctx context.Context, t *testing.T, c *client.Client, profile string) imsServicesState {
	t.Helper()

	servers, err := c.ListIMSApplicationServers(ctx)
	if err != nil {
		t.Fatalf("list IMS application servers: %v", err)
	}

	services, err := c.ListIMSServices(ctx)
	if err != nil {
		t.Fatalf("list IMS services: %v", err)
	}

	assigned, err := c.GetProfileIMSServices(ctx, profile)
	if err != nil {
		t.Fatalf("get IMS services of profile %s: %v", profile, err)
	}

	state := imsServicesState{ApplicationServers: []client.IMSApplicationServer{}, Services: []client.IMSService{}, ProfileServices: assigned.Services}

	for _, as := range servers.Items {
		if strings.HasPrefix(as.Name, apiMatrixName("")) {
			state.ApplicationServers = append(state.ApplicationServers, as)
		}
	}

	for _, svc := range services.Items {
		if strings.HasPrefix(svc.Name, apiMatrixName("")) {
			state.Services = append(state.Services, svc)
		}
	}

	return state
}

func cleanupIMSServices(ctx context.Context, t *testing.T, c *client.Client, profile string) {
	if err := c.SetProfileIMSServices(ctx, profile, nil); err != nil {
		t.Logf("cleanup: clear IMS services of profile %s: %v", profile, err)
	}

	if err := c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: profile}); err != nil {
		t.Logf("cleanup: delete profile %s: %v", profile, err)
	}

	if services, err := c.ListIMSServices(ctx); err == nil {
		for _, svc := range services.Items {
			if strings.HasPrefix(svc.Name, apiMatrixName("")) {
				if err := c.DeleteIMSService(ctx, svc.Name); err != nil {
					t.Logf("cleanup: delete IMS service %s: %v", svc.Name, err)
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

func imsMatrixMCPTTCriteria() []client.IMSInitialFilterCriterion {
	register, originating, header, icsi := "REGISTER", "ORIGINATING_REGISTERED", "^P-Asserted-Service$", `^urn:urn-7:3gpp-service\.ims\.icsi\.mcptt$`

	return []client.IMSInitialFilterCriterion{
		{
			Priority:                0,
			TriggerPoint:            client.IMSTriggerPoint{SPT: []client.IMSServicePointTrigger{{Group: []int32{0}, Method: &register}}},
			DefaultHandling:         "SESSION_CONTINUED",
			IncludeRegisterRequest:  true,
			IncludeRegisterResponse: true,
		},
		{
			Priority: 1,
			TriggerPoint: client.IMSTriggerPoint{ConditionTypeCNF: true, SPT: []client.IMSServicePointTrigger{
				{Group: []int32{0}, SessionCase: &originating},
				{Group: []int32{1}, SIPHeader: &client.IMSHeaderMatch{Header: header, Content: &icsi}},
			}},
			DefaultHandling: "SESSION_TERMINATED",
		},
	}
}

func imsMatrixCustomCriteria(uri string) []client.IMSInitialFilterCriterion {
	return []client.IMSInitialFilterCriterion{{
		TriggerPoint:    client.IMSTriggerPoint{SPT: []client.IMSServicePointTrigger{{Group: []int32{0}, RequestURI: &uri}}},
		DefaultHandling: "SESSION_TERMINATED",
	}}
}

type imsMatrixStep struct {
	name   string
	writer int
	apply  func(c *client.Client) error
	want   imsServicesState
}

func imsMatrixSteps(ctx context.Context, profile string) []imsMatrixStep {
	mcpttAS := apiMatrixName("ims-as-mcptt")
	confAS := apiMatrixName("ims-as-conf")
	mcptt := apiMatrixName("ims-svc-mcptt")
	conf := apiMatrixName("ims-svc-conf")

	asMCPTT := client.IMSApplicationServer{Name: mcpttAS, ServerName: "sip:mcptt.ims.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"}}
	asConf := client.IMSApplicationServer{Name: confAS, ServerName: "sip:conf.ims.example.org", PublicServiceIdentities: []string{}}
	asConfUpdated := client.IMSApplicationServer{Name: confAS, ServerName: "sip:conf2.ims.example.org", PublicServiceIdentities: []string{"sip:conference@ims.example.org"}}

	svcMCPTT := client.IMSService{Name: mcptt, Kind: "mcptt", ApplicationServerName: mcpttAS, InitialFilterCriteria: imsMatrixMCPTTCriteria()}
	svcConf := client.IMSService{Name: conf, Kind: "custom", ApplicationServerName: confAS, InitialFilterCriteria: imsMatrixCustomCriteria("^sip:conference@.*$")}
	svcConfUpdated := client.IMSService{Name: conf, Kind: "custom", ApplicationServerName: confAS, InitialFilterCriteria: imsMatrixCustomCriteria("^sip:conf@.*$")}

	servers := []client.IMSApplicationServer{asConf, asMCPTT}
	serversUpdated := []client.IMSApplicationServer{asConfUpdated, asMCPTT}

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
			want: imsServicesState{ApplicationServers: servers, Services: []client.IMSService{}, ProfileServices: []string{}},
		},
		{
			name:   "update_application_server",
			writer: 1,
			apply: func(c *client.Client) error {
				return c.UpdateIMSApplicationServer(ctx, confAS, &client.UpdateIMSApplicationServerOptions{ServerName: asConfUpdated.ServerName, PublicServiceIdentities: asConfUpdated.PublicServiceIdentities})
			},
			want: imsServicesState{ApplicationServers: serversUpdated, Services: []client.IMSService{}, ProfileServices: []string{}},
		},
		{
			name:   "create_services",
			writer: 2,
			apply: func(c *client.Client) error {
				if _, err := c.CreateIMSService(ctx, &client.CreateIMSServiceOptions{Name: mcptt, Kind: "mcptt", ApplicationServerName: mcpttAS}); err != nil {
					return err
				}

				_, err := c.CreateIMSService(ctx, &client.CreateIMSServiceOptions{Name: conf, Kind: "custom", ApplicationServerName: confAS, InitialFilterCriteria: svcConf.InitialFilterCriteria})

				return err
			},
			want: imsServicesState{ApplicationServers: serversUpdated, Services: []client.IMSService{svcConf, svcMCPTT}, ProfileServices: []string{}},
		},
		{
			name:   "update_service",
			writer: 0,
			apply: func(c *client.Client) error {
				return c.UpdateIMSService(ctx, conf, &client.UpdateIMSServiceOptions{Kind: "custom", ApplicationServerName: confAS, InitialFilterCriteria: svcConfUpdated.InitialFilterCriteria})
			},
			want: imsServicesState{ApplicationServers: serversUpdated, Services: []client.IMSService{svcConfUpdated, svcMCPTT}, ProfileServices: []string{}},
		},
		{
			name:   "assign_services",
			writer: 1,
			apply: func(c *client.Client) error {
				return c.SetProfileIMSServices(ctx, profile, []string{mcptt, conf})
			},
			want: imsServicesState{ApplicationServers: serversUpdated, Services: []client.IMSService{svcConfUpdated, svcMCPTT}, ProfileServices: []string{mcptt, conf}},
		},
		{
			name:   "reorder_services",
			writer: 2,
			apply: func(c *client.Client) error {
				return c.SetProfileIMSServices(ctx, profile, []string{conf, mcptt})
			},
			want: imsServicesState{ApplicationServers: serversUpdated, Services: []client.IMSService{svcConfUpdated, svcMCPTT}, ProfileServices: []string{conf, mcptt}},
		},
		{
			name:   "unassign_and_delete_service",
			writer: 0,
			apply: func(c *client.Client) error {
				if err := c.SetProfileIMSServices(ctx, profile, []string{mcptt}); err != nil {
					return err
				}

				return c.DeleteIMSService(ctx, conf)
			},
			want: imsServicesState{ApplicationServers: serversUpdated, Services: []client.IMSService{svcMCPTT}, ProfileServices: []string{mcptt}},
		},
		{
			name:   "delete_application_server",
			writer: 1,
			apply: func(c *client.Client) error {
				return c.DeleteIMSApplicationServer(ctx, confAS)
			},
			want: imsServicesState{ApplicationServers: []client.IMSApplicationServer{asMCPTT}, Services: []client.IMSService{svcMCPTT}, ProfileServices: []string{mcptt}},
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

func runIMSServicesMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	profile := createIMSMatrixProfile(ctx, t, c)

	t.Cleanup(func() { cleanupIMSServices(ctx, t, c, profile) })

	for _, step := range imsMatrixSteps(ctx, profile) {
		t.Run(step.name, func(t *testing.T) {
			if err := step.apply(c); err != nil {
				t.Fatalf("%s: %v", step.name, err)
			}

			if got := readIMSServicesState(ctx, t, c, profile); !reflect.DeepEqual(got, step.want) {
				t.Fatalf("ims services: got %+v, want %+v", got, step.want)
			}
		})
	}

	mcpttAS := apiMatrixName("ims-as-mcptt")
	mcptt := apiMatrixName("ims-svc-mcptt")

	t.Run("rejections", func(t *testing.T) {
		if _, err := c.CreateIMSApplicationServer(ctx, &client.CreateIMSApplicationServerOptions{Name: apiMatrixName("ims-as-clash"), ServerName: "sip:clash.ims.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"}}); err == nil {
			t.Fatal("an application server with a PSI hosted elsewhere was created")
		}

		if err := c.DeleteIMSApplicationServer(ctx, mcpttAS); err == nil {
			t.Fatal("an application server used by an IMS service was deleted")
		}

		if err := c.DeleteIMSService(ctx, mcptt); err == nil {
			t.Fatal("an IMS service assigned to a profile was deleted")
		}

		if _, err := c.CreateIMSService(ctx, &client.CreateIMSServiceOptions{Name: apiMatrixName("ims-svc-bad"), Kind: "mcptt", ApplicationServerName: mcpttAS, InitialFilterCriteria: imsMatrixCustomCriteria("^sip:x@.*$")}); err == nil {
			t.Fatal("an mcptt IMS service with initial filter criteria was created")
		}

		if err := c.SetProfileIMSServices(ctx, profile, []string{mcptt, apiMatrixName("ims-svc-missing")}); err == nil {
			t.Fatal("a profile was given an unknown IMS service")
		}

		_, err := c.GetIMSService(ctx, apiMatrixName("ims-svc-missing"))
		assertNotFound(t, err, "IMS service")

		_, err = c.GetIMSApplicationServer(ctx, apiMatrixName("ims-as-missing"))
		assertNotFound(t, err, "IMS application server")
	})
}

func runIMSServicesHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients
	profile := createIMSMatrixProfile(ctx, t, h.Leader)

	t.Cleanup(func() { cleanupIMSServices(ctx, t, h.Leader, profile) })

	for _, step := range imsMatrixSteps(ctx, profile) {
		t.Run(step.name, func(t *testing.T) {
			if err := step.apply(nodes[step.writer]); err != nil {
				t.Fatalf("%s on node %d: %v", step.name, step.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				if got := readIMSServicesState(ctx, t, c, profile); !reflect.DeepEqual(got, step.want) {
					t.Fatalf("node %d ims services: got %+v, want %+v", i+1, got, step.want)
				}
			}
		})
	}
}
