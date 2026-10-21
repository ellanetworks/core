// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestDataNetworksAndPoliciesBeforeV25(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 24)

	if err := d.InitializeLocalSettings(ctx); err != nil {
		t.Fatalf("initialize local settings: %v", err)
	}

	if err := d.Initialize(ctx); err != nil {
		t.Fatalf("Initialize at schema 24: %v", err)
	}

	ims := &DataNetwork{Name: models.IMSDataNetworkName, IPv4Pool: "10.60.0.0/16", DNS: "8.8.8.8", MTU: 1400, IMS: true}
	if err := d.CreateDataNetwork(ctx, ims); err != nil {
		t.Fatalf("create ims data network: %v", err)
	}

	ims.MTU = 1300
	if err := d.UpdateDataNetwork(ctx, ims); err != nil {
		t.Fatalf("update ims data network: %v", err)
	}

	got, err := d.GetDataNetwork(ctx, models.IMSDataNetworkName)
	if err != nil {
		t.Fatalf("get ims data network: %v", err)
	}

	if !got.IMS || got.MTU != 1300 {
		t.Fatalf("ims data network = %+v, want IMS and MTU 1300", got)
	}

	internet, err := d.GetDataNetwork(ctx, InitialDataNetworkName)
	if err != nil {
		t.Fatalf("get default data network: %v", err)
	}

	if internet.IMS {
		t.Fatalf("default data network is IMS before v25")
	}

	policy, err := d.GetPolicy(ctx, InitialPolicyName)
	if err != nil {
		t.Fatalf("get default policy: %v", err)
	}

	if policy.PreemptionCapability != models.PreemptionCapabilityNotPreempt || policy.PreemptionVulnerability != models.PreemptionVulnerabilityNotPreemptable {
		t.Fatalf("default policy pre-emption %s/%s, want NOT_PREEMPT/NOT_PREEMPTABLE", policy.PreemptionCapability, policy.PreemptionVulnerability)
	}

	policy.Arp = 3
	if err := d.UpdatePolicy(ctx, policy); err != nil {
		t.Fatalf("update default policy: %v", err)
	}

	mcx := &DataNetwork{Name: "mcx", IPv4Pool: "10.61.0.0/16", DNS: "8.8.8.8", MTU: 1400, IMS: true}
	if err := d.CreateDataNetwork(ctx, mcx); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("create an IMS data network not named ims: err = %v, want ErrMigrationPending", err)
	}

	preempting := *policy
	preempting.PreemptionCapability = models.PreemptionCapabilityMayPreempt

	if err := d.UpdatePolicy(ctx, &preempting); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("update with pre-emption: err = %v, want ErrMigrationPending", err)
	}

	if servers, err := d.ListIMSApplicationServers(ctx); err != nil || len(servers) != 0 {
		t.Fatalf("list application servers = %d, err %v; want empty", len(servers), err)
	}

	if services, err := d.ListIMSServices(ctx); err != nil || len(services) != 0 {
		t.Fatalf("list IMS services = %d, err %v; want empty", len(services), err)
	}

	if _, err := d.GetIMSService(ctx, "mcptt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get IMS service: err = %v, want ErrNotFound", err)
	}

	as := &IMSApplicationServer{Name: "mcptt", ServerName: "sip:mcptt.example.org"}
	if err := d.CreateIMSApplicationServer(ctx, as); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("create application server: err = %v, want ErrMigrationPending", err)
	}

	if err := d.SetProfileIMSServices(ctx, policy.ProfileID, nil); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("set profile IMS services: err = %v, want ErrMigrationPending", err)
	}
}
