package fixture

import (
	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// PolicySpec describes a QoS policy for a (profile, slice, data network)
// triple.
type PolicySpec struct {
	Name                string
	ProfileName         string
	SliceName           string
	DataNetworkName     string
	SessionAmbrUplink   string
	SessionAmbrDownlink string
	Var5qi              int32
	Arp                 int32
}

// DefaultPolicySpec returns the scenarios-package default policy.
func DefaultPolicySpec() PolicySpec {
	return PolicySpec{
		Name:                scenarios.DefaultPolicyName,
		ProfileName:         scenarios.DefaultProfileName,
		SliceName:           scenarios.DefaultSliceName,
		DataNetworkName:     scenarios.DefaultDNN,
		SessionAmbrUplink:   scenarios.DefaultPolicySessionAmbrUplink,
		SessionAmbrDownlink: scenarios.DefaultPolicySessionAmbrDownlink,
		Var5qi:              9,
		Arp:                 15,
	}
}

// Policy upserts the policy: when a policy with the same name exists
// (e.g. Core's seeded "default"), its full config is overwritten to
// match spec; otherwise the policy is created.
func (f *F) Policy(spec PolicySpec) {
	f.t.Helper()

	if _, err := f.c.GetPolicy(f.ctx, &client.GetPolicyOptions{Name: spec.Name}); err == nil {
		if err := f.c.UpdatePolicy(f.ctx, spec.Name, &client.UpdatePolicyOptions{
			ProfileName:         spec.ProfileName,
			SliceName:           spec.SliceName,
			DataNetworkName:     spec.DataNetworkName,
			SessionAmbrUplink:   spec.SessionAmbrUplink,
			SessionAmbrDownlink: spec.SessionAmbrDownlink,
			Var5qi:              spec.Var5qi,
			Arp:                 spec.Arp,
		}); err != nil {
			f.fatalf("update policy %q: %v", spec.Name, err)
		}

		return
	}

	if err := f.c.CreatePolicy(f.ctx, &client.CreatePolicyOptions{
		Name:                spec.Name,
		ProfileName:         spec.ProfileName,
		SliceName:           spec.SliceName,
		DataNetworkName:     spec.DataNetworkName,
		SessionAmbrUplink:   spec.SessionAmbrUplink,
		SessionAmbrDownlink: spec.SessionAmbrDownlink,
		Var5qi:              spec.Var5qi,
		Arp:                 spec.Arp,
	}); err != nil {
		f.fatalf("create policy %q: %v", spec.Name, err)
	}
}
