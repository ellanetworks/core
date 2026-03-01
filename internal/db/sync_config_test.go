// Copyright 2026 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/fleet/client"
	"github.com/ellanetworks/core/internal/db"
)

// newSyncTestDB creates a fresh database with default initialization for each test.
func newSyncTestDB(t *testing.T) *db.Database {
	t.Helper()

	database, err := db.NewDatabase(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabase: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	return database
}

// baseSyncConfig returns a SyncConfig that matches the default database state
// (operator mcc=001/mnc=01, data network "internet" 10.45.0.0/22, policy "default",
// NAT enabled, empty N3 external address, no subscribers, no routes).
// Fleet-side IDs are arbitrary positive integers.
func baseSyncConfig() client.SyncConfig {
	return client.SyncConfig{
		Operator: client.Operator{
			ID:           client.OperatorID{Mcc: "001", Mnc: "01"},
			Slice:        client.OperatorSlice{Sst: 1, Sd: []byte{0x10, 0x20, 0x30}},
			OperatorCode: "",
			Tracking:     client.OperatorTracking{SupportedTacs: []string{}},
			HomeNetwork:  client.OperatorHomeNetwork{PrivateKey: ""},
		},
		Networking: client.SyncNetworking{
			DataNetworks: []client.DataNetwork{
				{ID: 100, Name: "internet", IPPool: "10.45.0.0/22", DNS: "", MTU: 0},
			},
			Routes: []client.Route{},
			NAT:    true,
			NetworkInterfaces: client.SyncNetworkInterfaces{
				N3ExternalAddress: "",
			},
		},
		Policies: []client.Policy{
			{ID: 200, Name: "default", BitrateUplink: "", BitrateDownlink: "", Var5qi: 0, Arp: 0, DataNetworkID: 100},
		},
		Subscribers: []client.Subscriber{},
	}
}

func TestUpdateConfig_OperatorSync(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	cfg := baseSyncConfig()
	cfg.Operator = client.Operator{
		ID:           client.OperatorID{Mcc: "310", Mnc: "410"},
		Slice:        client.OperatorSlice{Sst: 2, Sd: []byte{0xAA, 0xBB, 0xCC}},
		OperatorCode: "ABCDEF",
		Tracking:     client.OperatorTracking{SupportedTacs: []string{"0001", "0002"}},
		HomeNetwork:  client.OperatorHomeNetwork{PrivateKey: "myprivkey"},
	}

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig: %s", err)
	}

	op, err := database.GetOperator(ctx)
	if err != nil {
		t.Fatalf("GetOperator: %s", err)
	}

	if op.Mcc != "310" {
		t.Errorf("expected mcc 310, got %s", op.Mcc)
	}

	if op.Mnc != "410" {
		t.Errorf("expected mnc 410, got %s", op.Mnc)
	}

	if op.Sst != 2 {
		t.Errorf("expected sst 2, got %d", op.Sst)
	}

	if op.OperatorCode != "ABCDEF" {
		t.Errorf("expected operatorCode ABCDEF, got %s", op.OperatorCode)
	}

	if op.HomeNetworkPrivateKey != "myprivkey" {
		t.Errorf("expected homeNetworkPrivateKey myprivkey, got %s", op.HomeNetworkPrivateKey)
	}

	tacs, err := op.GetSupportedTacs()
	if err != nil {
		t.Fatalf("GetSupportedTacs: %s", err)
	}

	if len(tacs) != 2 || tacs[0] != "0001" || tacs[1] != "0002" {
		t.Errorf("expected supported TACs [0001, 0002], got %v", tacs)
	}
}

func TestUpdateConfig_NATSync(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	// Default NAT is enabled. Disable it.
	cfg := baseSyncConfig()
	cfg.Networking.NAT = false

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (disable NAT): %s", err)
	}

	enabled, err := database.IsNATEnabled(ctx)
	if err != nil {
		t.Fatalf("IsNATEnabled: %s", err)
	}

	if enabled {
		t.Error("expected NAT to be disabled, but it is enabled")
	}

	// Re-enable.
	cfg.Networking.NAT = true

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (enable NAT): %s", err)
	}

	enabled, err = database.IsNATEnabled(ctx)
	if err != nil {
		t.Fatalf("IsNATEnabled: %s", err)
	}

	if !enabled {
		t.Error("expected NAT to be enabled, but it is disabled")
	}
}

func TestUpdateConfig_N3Sync(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	cfg := baseSyncConfig()
	cfg.Networking.NetworkInterfaces.N3ExternalAddress = "192.168.1.100"

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig: %s", err)
	}

	n3, err := database.GetN3Settings(ctx)
	if err != nil {
		t.Fatalf("GetN3Settings: %s", err)
	}

	if n3.ExternalAddress != "192.168.1.100" {
		t.Errorf("expected N3 external address 192.168.1.100, got %s", n3.ExternalAddress)
	}
}

func TestUpdateConfig_DataNetworksCRUD(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	// Create: add a second data network.
	cfg := baseSyncConfig()
	cfg.Networking.DataNetworks = append(cfg.Networking.DataNetworks,
		client.DataNetwork{ID: 101, Name: "enterprise", IPPool: "10.50.0.0/24", DNS: "8.8.8.8", MTU: 1400},
	)

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (create DN): %s", err)
	}

	dns, total, err := database.ListDataNetworksPage(ctx, 1, 100)
	if err != nil {
		t.Fatalf("ListDataNetworksPage: %s", err)
	}

	if total != 2 {
		t.Fatalf("expected 2 data networks, got %d", total)
	}

	var enterprise *db.DataNetwork

	for i := range dns {
		if dns[i].Name == "enterprise" {
			enterprise = &dns[i]
		}
	}

	if enterprise == nil {
		t.Fatal("expected 'enterprise' data network to exist")
	}

	if enterprise.IPPool != "10.50.0.0/24" {
		t.Errorf("expected ip_pool '10.50.0.0/24', got %s", enterprise.IPPool)
	}

	if enterprise.DNS != "8.8.8.8" {
		t.Errorf("expected dns '8.8.8.8', got %s", enterprise.DNS)
	}

	if enterprise.MTU != 1400 {
		t.Errorf("expected mtu 1400, got %d", enterprise.MTU)
	}

	// Update: change enterprise's pool.
	cfg.Networking.DataNetworks[1].IPPool = "10.60.0.0/16"

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (update DN): %s", err)
	}

	dn, err := database.GetDataNetwork(ctx, "enterprise")
	if err != nil {
		t.Fatalf("GetDataNetwork: %s", err)
	}

	if dn.IPPool != "10.60.0.0/16" {
		t.Errorf("expected updated ip_pool '10.60.0.0/16', got %s", dn.IPPool)
	}

	// Delete: remove enterprise from desired state.
	cfg.Networking.DataNetworks = cfg.Networking.DataNetworks[:1] // keep only "internet"

	// Policy still references DN ID 100 (internet), so this is safe.
	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (delete DN): %s", err)
	}

	_, total, err = database.ListDataNetworksPage(ctx, 1, 100)
	if err != nil {
		t.Fatalf("ListDataNetworksPage: %s", err)
	}

	if total != 1 {
		t.Errorf("expected 1 data network after delete, got %d", total)
	}
}

func TestUpdateConfig_PoliciesCRUD(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	// Start from base — has "internet" DN (fleet ID 100) and "default" policy (fleet ID 200).
	cfg := baseSyncConfig()

	// Create: add a new policy referencing the "internet" data network (fleet ID 100).
	cfg.Policies = append(cfg.Policies,
		client.Policy{ID: 201, Name: "premium", BitrateUplink: "500 Mbps", BitrateDownlink: "1 Gbps", Var5qi: 5, Arp: 1, DataNetworkID: 100},
	)

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (create policy): %s", err)
	}

	policies, total, err := database.ListPoliciesPage(ctx, 1, 100)
	if err != nil {
		t.Fatalf("ListPoliciesPage: %s", err)
	}

	if total != 2 {
		t.Fatalf("expected 2 policies, got %d", total)
	}

	var premium *db.Policy

	for i := range policies {
		if policies[i].Name == "premium" {
			premium = &policies[i]
		}
	}

	if premium == nil {
		t.Fatal("expected 'premium' policy to exist")
	}

	if premium.BitrateUplink != "500 Mbps" {
		t.Errorf("expected bitrate_uplink '500 Mbps', got %s", premium.BitrateUplink)
	}

	if premium.Var5qi != 5 {
		t.Errorf("expected var5qi 5, got %d", premium.Var5qi)
	}

	// Update: change premium's arp.
	cfg.Policies[1].Arp = 9

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (update policy): %s", err)
	}

	p, err := database.GetPolicy(ctx, "premium")
	if err != nil {
		t.Fatalf("GetPolicy: %s", err)
	}

	if p.Arp != 9 {
		t.Errorf("expected arp 9, got %d", p.Arp)
	}

	// Delete: remove premium policy.
	cfg.Policies = cfg.Policies[:1]

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (delete policy): %s", err)
	}

	_, total, err = database.ListPoliciesPage(ctx, 1, 100)
	if err != nil {
		t.Fatalf("ListPoliciesPage: %s", err)
	}

	if total != 1 {
		t.Errorf("expected 1 policy after delete, got %d", total)
	}
}

func TestUpdateConfig_SubscribersCRUD(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	cfg := baseSyncConfig()

	// Create a subscriber referencing the "default" policy (fleet ID 200).
	cfg.Subscribers = []client.Subscriber{
		{
			ID:             1,
			Imsi:           "001010100007487",
			SequenceNumber: "000000000042",
			PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
			Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
			PolicyID:       200,
		},
	}

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (create subscriber): %s", err)
	}

	sub, err := database.GetSubscriber(ctx, "001010100007487")
	if err != nil {
		t.Fatalf("GetSubscriber: %s", err)
	}

	if sub.PermanentKey != "6f30087629feb0b089783c81d0ae09b5" {
		t.Errorf("expected permanentKey '6f30087629feb0b089783c81d0ae09b5', got %s", sub.PermanentKey)
	}

	if sub.SequenceNumber != "000000000042" {
		t.Errorf("expected initial sequenceNumber '000000000042', got %s", sub.SequenceNumber)
	}

	// Update: change opc. Sequence number should be preserved.
	cfg.Subscribers[0].Opc = "aaaa1111bbbb2222cccc3333dddd4444"

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (update subscriber): %s", err)
	}

	sub, err = database.GetSubscriber(ctx, "001010100007487")
	if err != nil {
		t.Fatalf("GetSubscriber: %s", err)
	}

	if sub.Opc != "aaaa1111bbbb2222cccc3333dddd4444" {
		t.Errorf("expected updated opc, got %s", sub.Opc)
	}

	if sub.SequenceNumber != "000000000042" {
		t.Errorf("expected sequenceNumber to be preserved, got %s", sub.SequenceNumber)
	}

	// Delete: remove subscriber from desired state.
	cfg.Subscribers = nil

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (delete subscriber): %s", err)
	}

	subs, total, err := database.ListSubscribersPage(ctx, 1, 100)
	if err != nil {
		t.Fatalf("ListSubscribersPage: %s", err)
	}

	if total != 0 {
		t.Errorf("expected 0 subscribers after delete, got %d (subs: %v)", total, subs)
	}
}

func TestUpdateConfig_RoutesCRUD(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	cfg := baseSyncConfig()
	cfg.Networking.Routes = []client.Route{
		{ID: 1, Destination: "10.0.0.0/8", Gateway: "192.168.1.1", Interface: "n3", Metric: 100},
		{ID: 2, Destination: "172.16.0.0/12", Gateway: "192.168.1.1", Interface: "n6", Metric: 200},
	}

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (create routes): %s", err)
	}

	routes, total, err := database.ListRoutesPage(ctx, 1, 100)
	if err != nil {
		t.Fatalf("ListRoutesPage: %s", err)
	}

	if total != 2 {
		t.Fatalf("expected 2 routes, got %d", total)
	}

	// Verify interfaces were parsed correctly.
	ifaceMap := make(map[string]db.NetworkInterface)

	for _, r := range routes {
		ifaceMap[r.Destination] = r.Interface
	}

	if ifaceMap["10.0.0.0/8"] != db.N3 {
		t.Errorf("expected N3 interface for 10.0.0.0/8, got %v", ifaceMap["10.0.0.0/8"])
	}

	if ifaceMap["172.16.0.0/12"] != db.N6 {
		t.Errorf("expected N6 interface for 172.16.0.0/12, got %v", ifaceMap["172.16.0.0/12"])
	}

	// Delete one route by removing it from desired state.
	cfg.Networking.Routes = cfg.Networking.Routes[:1]

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig (delete route): %s", err)
	}

	_, total, err = database.ListRoutesPage(ctx, 1, 100)
	if err != nil {
		t.Fatalf("ListRoutesPage: %s", err)
	}

	if total != 1 {
		t.Errorf("expected 1 route after delete, got %d", total)
	}
}

func TestUpdateConfig_Idempotent(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	cfg := baseSyncConfig()
	cfg.Operator.ID.Mcc = "999"
	cfg.Networking.DataNetworks = append(cfg.Networking.DataNetworks,
		client.DataNetwork{ID: 101, Name: "extra", IPPool: "10.99.0.0/16"},
	)
	cfg.Subscribers = []client.Subscriber{
		{ID: 1, Imsi: "999010000000001", SequenceNumber: "000000000000", PermanentKey: "00112233445566778899aabbccddeeff", Opc: "ffeeddccbbaa99887766554433221100", PolicyID: 200},
	}
	cfg.Networking.Routes = []client.Route{
		{ID: 1, Destination: "10.0.0.0/8", Gateway: "1.2.3.4", Interface: "n6", Metric: 50},
	}

	for i := 0; i < 3; i++ {
		if err := database.UpdateConfig(ctx, cfg); err != nil {
			t.Fatalf("UpdateConfig iteration %d: %s", i, err)
		}
	}

	op, err := database.GetOperator(ctx)
	if err != nil {
		t.Fatalf("GetOperator: %s", err)
	}

	if op.Mcc != "999" {
		t.Errorf("expected mcc 999, got %s", op.Mcc)
	}

	dns, dnTotal, err := database.ListDataNetworksPage(ctx, 1, 100)
	if err != nil {
		t.Fatalf("ListDataNetworksPage: %s", err)
	}

	if dnTotal != 2 {
		t.Fatalf("expected 2 data networks, got %d (dns: %v)", dnTotal, dns)
	}

	_, subTotal, err := database.ListSubscribersPage(ctx, 1, 100)
	if err != nil {
		t.Fatalf("ListSubscribersPage: %s", err)
	}

	if subTotal != 1 {
		t.Errorf("expected 1 subscriber, got %d", subTotal)
	}

	_, routeTotal, err := database.ListRoutesPage(ctx, 1, 100)
	if err != nil {
		t.Fatalf("ListRoutesPage: %s", err)
	}

	if routeTotal != 1 {
		t.Errorf("expected 1 route, got %d", routeTotal)
	}
}

func TestUpdateConfig_PolicyWithNewDataNetwork(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	// Add a new data network (fleet ID 101) and a policy that references it.
	cfg := baseSyncConfig()
	cfg.Networking.DataNetworks = append(cfg.Networking.DataNetworks,
		client.DataNetwork{ID: 101, Name: "enterprise", IPPool: "10.70.0.0/16"},
	)
	cfg.Policies = append(cfg.Policies,
		client.Policy{ID: 202, Name: "enterprise-policy", BitrateUplink: "1 Gbps", BitrateDownlink: "1 Gbps", Var5qi: 1, Arp: 5, DataNetworkID: 101},
	)

	if err := database.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateConfig: %s", err)
	}

	p, err := database.GetPolicy(ctx, "enterprise-policy")
	if err != nil {
		t.Fatalf("GetPolicy: %s", err)
	}

	// The local DB ID for "enterprise" should have been properly resolved.
	dn, err := database.GetDataNetwork(ctx, "enterprise")
	if err != nil {
		t.Fatalf("GetDataNetwork: %s", err)
	}

	if p.DataNetworkID != dn.ID {
		t.Errorf("expected policy's DataNetworkID to match local DN ID %d, got %d", dn.ID, p.DataNetworkID)
	}
}

func TestUpdateConfig_SubscriberUnknownPolicyID(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	cfg := baseSyncConfig()
	cfg.Subscribers = []client.Subscriber{
		{ID: 1, Imsi: "001010100000001", SequenceNumber: "000000000000", PermanentKey: "00112233445566778899aabbccddeeff", Opc: "ffeeddccbbaa99887766554433221100", PolicyID: 9999},
	}

	err := database.UpdateConfig(ctx, cfg)
	if err == nil {
		t.Fatal("expected error when subscriber references unknown fleet policy ID, got nil")
	}
}

func TestUpdateConfig_PolicyUnknownDataNetworkID(t *testing.T) {
	database := newSyncTestDB(t)
	ctx := context.Background()

	cfg := baseSyncConfig()
	cfg.Policies = append(cfg.Policies,
		client.Policy{ID: 999, Name: "bad-policy", DataNetworkID: 8888},
	)

	err := database.UpdateConfig(ctx, cfg)
	if err == nil {
		t.Fatal("expected error when policy references unknown fleet data network ID, got nil")
	}
}
