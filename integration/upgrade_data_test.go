// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

const (
	upgradeSeedName       = "upgrade-seed"
	upgradeSeedIMSI       = "001019756180000"
	upgradeSeedStaticIP   = "10.88.0.10"
	upgradeSeedViewer     = "upgrade-viewer@ellanetworks.com"
	upgradeSeedTokenName  = "upgrade-seed-token"
	upgradeSeedPolicyDeny = "0.0.0.0/0"
)

type upgradeSeedSubscriber struct {
	Imsi        string
	ProfileName string
	Description string
	Msisdn      string
}

type upgradeSeedSnapshot struct {
	DataNetwork client.DataNetwork
	Slice       client.Slice
	Profile     client.Profile
	Policy      client.Policy
	Subscriber  upgradeSeedSubscriber
	StaticIPs   []string
	Viewer      client.User
	TokenNames  []string
}

type upgradeSeed struct {
	apiToken string
	want     upgradeSeedSnapshot
}

func seedUpgradeData(t *testing.T, ctx context.Context, c *client.Client) *upgradeSeed {
	t.Helper()

	deny := upgradeSeedPolicyDeny

	steps := []struct {
		name string
		run  func() error
	}{
		{"create data network", func() error {
			return c.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
				Name: upgradeSeedName, IPv4Pool: "10.88.0.0/24", DNS: "1.1.1.1", Mtu: 1380,
			})
		}},
		{"create slice", func() error {
			return c.CreateSlice(ctx, &client.CreateSliceOptions{Name: upgradeSeedName, Sst: 1, Sd: "0000aa"})
		}},
		{"serve the data network on the slice", func() error {
			return serveDataNetworkOnSlice(ctx, c, upgradeSeedName, upgradeSeedName)
		}},
		{"create profile", func() error {
			return c.CreateProfile(ctx, &client.CreateProfileOptions{
				Name: upgradeSeedName, UeAmbrUplink: "300 Mbps", UeAmbrDownlink: "500 Mbps",
			})
		}},
		{"create policy", func() error {
			return c.CreatePolicy(ctx, &client.CreatePolicyOptions{
				Name: upgradeSeedName, ProfileName: upgradeSeedName, SliceName: upgradeSeedName, DataNetworkName: upgradeSeedName,
				SessionAmbrUplink: "50 Mbps", SessionAmbrDownlink: "80 Mbps", Var5qi: 8, Arp: 3,
				Rules: &client.PolicyRules{Uplink: []client.PolicyRule{
					{Description: "allow https", Protocol: 6, PortLow: 443, PortHigh: 443, Action: "allow"},
					{Description: "deny all", RemotePrefix: &deny, Action: "deny"},
				}},
			})
		}},
		{"create subscriber", func() error {
			return c.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
				Imsi: upgradeSeedIMSI, Key: "0eefb0893e6f1c2855a3a244c6db1277", OPc: "98da19bbc55e2a5b53857d10557b1d26",
				SequenceNumber: "000000000022", ProfileName: upgradeSeedName, Description: "upgrade seed", Msisdn: "+15551230001",
			})
		}},
		{"create static IP", func() error {
			return c.CreateDataNetworkStaticIp(ctx, upgradeSeedName, &client.CreateStaticIPOptions{
				IMSI: upgradeSeedIMSI, Address: upgradeSeedStaticIP,
			})
		}},
		{"create viewer user", func() error {
			return c.CreateUser(ctx, &client.CreateUserOptions{
				Email: upgradeSeedViewer, RoleID: client.RoleReadOnly, Password: "Viewer-Pass-123!",
			})
		}},
	}

	for _, s := range steps {
		if err := s.run(); err != nil {
			t.Fatalf("seed upgrade data: %s: %v", s.name, err)
		}
	}

	tok, err := c.CreateMyAPIToken(ctx, &client.CreateAPITokenOptions{Name: upgradeSeedTokenName})
	if err != nil {
		t.Fatalf("seed upgrade data: create API token: %v", err)
	}

	want, err := readUpgradeSeed(ctx, c)
	if err != nil {
		t.Fatalf("read seeded upgrade data: %v", err)
	}

	return &upgradeSeed{apiToken: tok.Token, want: *want}
}

func serveDataNetworkOnSlice(ctx context.Context, c *client.Client, slice, dataNetwork string) error {
	err := c.CreateSliceDataNetwork(ctx, slice, &client.CreateSliceDataNetworkOptions{DataNetwork: dataNetwork})
	if err == nil {
		return nil
	}

	if _, listErr := c.ListSliceDataNetworks(ctx, slice); listErr != nil {
		return nil
	}

	return err
}

func readUpgradeSeed(ctx context.Context, c *client.Client) (*upgradeSeedSnapshot, error) {
	dn, err := c.GetDataNetwork(ctx, &client.GetDataNetworkOptions{Name: upgradeSeedName})
	if err != nil {
		return nil, err
	}

	slice, err := c.GetSlice(ctx, &client.GetSliceOptions{Name: upgradeSeedName})
	if err != nil {
		return nil, err
	}

	profile, err := c.GetProfile(ctx, &client.GetProfileOptions{Name: upgradeSeedName})
	if err != nil {
		return nil, err
	}

	policy, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: upgradeSeedName})
	if err != nil {
		return nil, err
	}

	sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: upgradeSeedIMSI})
	if err != nil {
		return nil, err
	}

	staticIPs, err := c.ListDataNetworkStaticIps(ctx, upgradeSeedName)
	if err != nil {
		return nil, err
	}

	viewer, err := c.GetUser(ctx, &client.GetUserOptions{Email: upgradeSeedViewer})
	if err != nil {
		return nil, err
	}

	tokens, err := c.ListMyAPITokens(ctx, &client.ListParams{Page: 1, PerPage: 100})
	if err != nil {
		return nil, err
	}

	snap := &upgradeSeedSnapshot{
		DataNetwork: client.DataNetwork{Name: dn.Name, IPv4Pool: dn.IPv4Pool, IPv6Pool: dn.IPv6Pool, DNS: dn.DNS, Mtu: dn.Mtu, IMS: dn.IMS},
		Slice:       *slice,
		Profile:     *profile,
		Policy:      *policy,
		Subscriber:  upgradeSeedSubscriber{Imsi: sub.Imsi, ProfileName: sub.ProfileName, Description: sub.Description, Msisdn: sub.Msisdn},
		Viewer:      *viewer,
	}

	for _, ip := range staticIPs.Items {
		snap.StaticIPs = append(snap.StaticIPs, ip.IMSI+"/"+ip.IPVersion+"/"+ip.Address)
	}

	for _, tk := range tokens.Items {
		snap.TokenNames = append(snap.TokenNames, tk.Name)
	}

	return snap, nil
}

func assertUpgradeSeed(t *testing.T, ctx context.Context, c *client.Client, baseURL string, seed *upgradeSeed, when string) {
	t.Helper()

	got, err := readUpgradeSeed(ctx, c)
	if err != nil {
		t.Fatalf("read seeded data %s: %v", when, err)
	}

	if !reflect.DeepEqual(*got, seed.want) {
		t.Fatalf("seeded data changed %s:\n got: %+v\nwant: %+v", when, *got, seed.want)
	}

	tokenClient, err := client.New(&client.Config{BaseURL: baseURL, APIToken: seed.apiToken})
	if err != nil {
		t.Fatalf("API token client %s: %v", when, err)
	}

	if _, err := tokenClient.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: upgradeSeedIMSI}); err != nil {
		t.Fatalf("API token minted before the upgrade rejected %s: %v", when, err)
	}
}

func assertUpgradeSeedOnAllNodes(t *testing.T, ctx context.Context, clients []*client.Client, seed *upgradeSeed, when string) {
	t.Helper()

	for i, c := range clients {
		assertUpgradeSeed(t, ctx, c, APIAddressForCluster(i+1), seed, fmt.Sprintf("on node %d %s", i+1, when))
	}
}

func assertUpgradedClusterSurvivesRestartAndFailover(t *testing.T, ctx context.Context, dc *DockerClient, composeDir string, clients []*client.Client, seed *upgradeSeed) {
	t.Helper()

	services := haNodeServices[:len(clients)]

	HALog(t, "=== post-roll: full cluster restart ===")

	for _, svc := range services {
		if err := dc.ComposeStopWithFile(ctx, composeDir, ComposeFile(), svc); err != nil {
			t.Fatalf("stop %s: %v", svc, err)
		}
	}

	for _, svc := range services {
		if err := dc.ComposeStartWithFile(ctx, composeDir, ComposeFile(), svc); err != nil {
			t.Fatalf("start %s: %v", svc, err)
		}
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		t.Fatalf("cluster not ready after a full restart: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready after a full restart: %v", err)
	}

	assertUpgradeSeedOnAllNodes(t, ctx, clients, seed, "after a full cluster restart")

	HALog(t, "=== post-roll: leader failover ===")

	leaderIdx, _, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader before failover: %v", err)
	}

	if err := dc.ComposeStopWithFile(ctx, composeDir, ComposeFile(), services[leaderIdx]); err != nil {
		t.Fatalf("stop leader %s: %v", services[leaderIdx], err)
	}

	survivors := make([]*client.Client, 0, len(clients)-1)

	for i, c := range clients {
		if i != leaderIdx {
			survivors = append(survivors, c)
		}
	}

	_, newLeader, err := findLeader(ctx, survivors)
	if err != nil {
		t.Fatalf("no new leader after stopping node %d: %v", leaderIdx+1, err)
	}

	const failoverIMSI = "001019756181000"

	createSubscriberWithRetry(t, ctx, newLeader, failoverIMSI, time.Minute)

	if err := dc.ComposeStartWithFile(ctx, composeDir, ComposeFile(), services[leaderIdx]); err != nil {
		t.Fatalf("restart former leader %s: %v", services[leaderIdx], err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready after the former leader rejoined: %v", err)
	}

	idx, err := leaderAppliedIndex(ctx, newLeader)
	if err != nil {
		t.Fatalf("leader applied index after failover: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, clients, idx); err != nil {
		t.Fatalf("nodes did not converge after failover: %v", err)
	}

	assertUpgradeSeedOnAllNodes(t, ctx, clients, seed, "after a leader failover")

	for i, c := range clients {
		assertSubscribersPresent(t, ctx, c, []string{failoverIMSI}, fmt.Sprintf("on node %d after a leader failover", i+1))
	}
}

func createSubscriberWithRetry(t *testing.T, ctx context.Context, c *client.Client, imsi string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	var lastErr error

	for time.Now().Before(deadline) {
		lastErr = c.CreateSubscriber(ctx, standaloneSubscriber(imsi))
		if lastErr == nil {
			return
		}

		if !isTransientWriteError(lastErr) {
			t.Fatalf("create subscriber %s: %v", imsi, lastErr)
		}

		time.Sleep(time.Second)
	}

	t.Fatalf("create subscriber %s still failing after %s: %v", imsi, timeout, lastErr)
}
