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

// Policy creates a policy, or verifies an existing one with the same
// name matches the full spec. Fails the test on mismatch.
func (f *F) Policy(spec PolicySpec) {
	f.t.Helper()

	existing, err := f.c.GetPolicy(f.ctx, &client.GetPolicyOptions{Name: spec.Name})
	if err == nil {
		if existing.ProfileName != spec.ProfileName ||
			existing.SliceName != spec.SliceName ||
			existing.DataNetworkName != spec.DataNetworkName ||
			existing.SessionAmbrUplink != spec.SessionAmbrUplink ||
			existing.SessionAmbrDownlink != spec.SessionAmbrDownlink ||
			existing.Var5qi != spec.Var5qi ||
			existing.Arp != spec.Arp {
			f.fatalf("policy %q exists with different config (profile=%q slice=%q dn=%q up=%q down=%q 5qi=%d arp=%d), want (profile=%q slice=%q dn=%q up=%q down=%q 5qi=%d arp=%d)",
				spec.Name,
				existing.ProfileName, existing.SliceName, existing.DataNetworkName,
				existing.SessionAmbrUplink, existing.SessionAmbrDownlink,
				existing.Var5qi, existing.Arp,
				spec.ProfileName, spec.SliceName, spec.DataNetworkName,
				spec.SessionAmbrUplink, spec.SessionAmbrDownlink,
				spec.Var5qi, spec.Arp)
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
