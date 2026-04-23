package fixture

import (
	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// DataNetworkSpec describes a PDU session anchor (DNN + IP pool).
type DataNetworkSpec struct {
	Name   string
	IPPool string
	DNS    string
	MTU    int32
}

// DefaultDataNetworkSpec returns the scenarios-package default data
// network.
func DefaultDataNetworkSpec() DataNetworkSpec {
	return DataNetworkSpec{
		Name:   scenarios.DefaultDNN,
		IPPool: scenarios.DefaultUEIPPool,
		DNS:    scenarios.DefaultDNS,
		MTU:    scenarios.DefaultMTU,
	}
}

// DataNetwork creates a data network, or verifies an existing one with
// the same name matches the spec.
func (f *F) DataNetwork(spec DataNetworkSpec) {
	f.t.Helper()

	existing, err := f.c.GetDataNetwork(f.ctx, &client.GetDataNetworkOptions{Name: spec.Name})
	if err == nil {
		if existing.IPPool != spec.IPPool || existing.DNS != spec.DNS || existing.Mtu != spec.MTU {
			f.fatalf("data network %q exists with different config: have (pool=%q dns=%q mtu=%d), want (pool=%q dns=%q mtu=%d)",
				spec.Name,
				existing.IPPool, existing.DNS, existing.Mtu,
				spec.IPPool, spec.DNS, spec.MTU)
		}

		return
	}

	if err := f.c.CreateDataNetwork(f.ctx, &client.CreateDataNetworkOptions{
		Name:   spec.Name,
		IPPool: spec.IPPool,
		DNS:    spec.DNS,
		Mtu:    spec.MTU,
	}); err != nil {
		f.fatalf("create data network %q: %v", spec.Name, err)
	}
}
