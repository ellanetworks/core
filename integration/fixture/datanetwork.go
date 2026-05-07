package fixture

import (
	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// DataNetworkSpec describes a PDU session anchor (DNN + IP pool).
type DataNetworkSpec struct {
	Name     string
	IPPool   string
	IPv6Pool string
	DNS      string
	MTU      int32
}

func DefaultDataNetworkSpec() DataNetworkSpec {
	return DataNetworkSpec{
		Name:     scenarios.DefaultDNN,
		IPPool:   scenarios.DefaultUEIPPool,
		IPv6Pool: scenarios.DefaultUEIPv6Pool,
		DNS:      scenarios.DefaultDNS,
		MTU:      scenarios.DefaultMTU,
	}
}

// DataNetwork upserts the baseline data network: when it already exists
// (e.g. Core's seeded "internet"), its configuration is overwritten to
// match spec. A pre-existing DN is never left with mismatched parameters;
// scenarios needing specific DN parameters must use a distinct name.
func (f *F) DataNetwork(spec DataNetworkSpec) {
	f.t.Helper()

	if _, err := f.c.GetDataNetwork(f.ctx, &client.GetDataNetworkOptions{Name: spec.Name}); err == nil {
		if err := f.c.UpdateDataNetwork(f.ctx, &client.UpdateDataNetworkOptions{
			Name:     spec.Name,
			IPPool:   spec.IPPool,
			IPv6Pool: spec.IPv6Pool,
			DNS:      spec.DNS,
			Mtu:      spec.MTU,
		}); err != nil {
			f.fatalf("update data network %q: %v", spec.Name, err)
		}

		return
	}

	if err := f.c.CreateDataNetwork(f.ctx, &client.CreateDataNetworkOptions{
		Name:     spec.Name,
		IPPool:   spec.IPPool,
		IPv6Pool: spec.IPv6Pool,
		DNS:      spec.DNS,
		Mtu:      spec.MTU,
	}); err != nil {
		f.fatalf("create data network %q: %v", spec.Name, err)
	}
}
