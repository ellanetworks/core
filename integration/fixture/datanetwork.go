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

// DataNetwork creates a data network. Idempotent.
func (f *F) DataNetwork(spec DataNetworkSpec) {
	f.t.Helper()

	if _, err := f.c.GetDataNetwork(f.ctx, &client.GetDataNetworkOptions{Name: spec.Name}); err == nil {
		return
	}

	if err := f.c.CreateDataNetwork(f.ctx, &client.CreateDataNetworkOptions{
		Name:   spec.Name,
		IPPool: spec.IPPool,
		DNS:    spec.DNS,
		Mtu:    spec.MTU,
	}); err != nil && !isAlreadyExists(err) {
		f.fatalf("create data network %q: %v", spec.Name, err)
	}
}
