package fixture

import (
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// Apply provisions everything in spec against the Ella Core client bound
// to f. It is idempotent: resources that already match the spec are left
// in place; mismatches fail the test via f.t.Fatalf.
//
// The order follows dependencies: operator → profiles → slices → data
// networks → policies → subscribers, so each resource's dependencies
// exist before it is referenced.
func (f *F) Apply(spec scenarios.FixtureSpec) {
	f.t.Helper()

	if spec.Operator != nil {
		f.Operator(OperatorSpec{
			MCC:           spec.Operator.MCC,
			MNC:           spec.Operator.MNC,
			SupportedTACs: spec.Operator.SupportedTACs,
		})
	}

	for _, k := range spec.HomeNetworkKeys {
		f.HomeNetworkKey(HomeNetworkKeySpec{
			KeyIdentifier: k.KeyIdentifier,
			Scheme:        k.Scheme,
			PrivateKey:    k.PrivateKey,
		})
	}

	for _, p := range spec.Profiles {
		f.Profile(ProfileSpec{
			Name:           p.Name,
			UeAmbrUplink:   p.UeAmbrUplink,
			UeAmbrDownlink: p.UeAmbrDownlink,
		})
	}

	for _, s := range spec.Slices {
		f.Slice(SliceSpec{
			Name: s.Name,
			SST:  s.SST,
			SD:   s.SD,
		})
	}

	for _, dn := range spec.DataNetworks {
		f.DataNetwork(DataNetworkSpec{
			Name:   dn.Name,
			IPPool: dn.IPPool,
			DNS:    dn.DNS,
			MTU:    dn.MTU,
		})
	}

	for _, p := range spec.Policies {
		f.Policy(PolicySpec{
			Name:                p.Name,
			ProfileName:         p.ProfileName,
			SliceName:           p.SliceName,
			DataNetworkName:     p.DataNetworkName,
			SessionAmbrUplink:   p.SessionAmbrUplink,
			SessionAmbrDownlink: p.SessionAmbrDownlink,
			Var5qi:              p.Var5qi,
			Arp:                 p.Arp,
		})
	}

	for _, s := range spec.Subscribers {
		f.Subscriber(SubscriberSpec{
			IMSI:           s.IMSI,
			Key:            s.Key,
			OPc:            s.OPc,
			SequenceNumber: s.SequenceNumber,
			ProfileName:    s.ProfileName,
		})
	}
}
