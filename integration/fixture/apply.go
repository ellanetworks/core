package fixture

import (
	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// Apply provisions every scenario-owned resource in spec and registers a
// t.Cleanup that deletes it when the subtest ends. Cleanups run in LIFO
// order, which matches the reverse-dependency teardown order: subscribers
// first, then policies, data networks, slices, and finally profiles.
//
// Scoped resources are created with strict semantics — if one already
// exists under the same name, the test fails loudly. That's intentional:
// it surfaces teardown bugs instead of silently mutating shared state.
//
// Non-scoped resources (operator singleton, home network keys) are applied
// idempotently and are not cleaned up; they are expected to be established
// once by the baseline and shared across all subtests.
//
// Rule for scenario authors: a scenario may reference baseline names
// (default profile/slice/DN/policy) only when its assertions match
// baseline values. Anything else must be declared under a scoped name in
// the scenario's fixture — never mutate the baseline.
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
		f.scopedProfile(p)
	}

	for _, s := range spec.Slices {
		f.scopedSlice(s)
	}

	for _, dn := range spec.DataNetworks {
		f.scopedDataNetwork(dn)
	}

	for _, p := range spec.Policies {
		f.scopedPolicy(p)
	}

	for _, s := range spec.Subscribers {
		f.scopedSubscriber(s)
	}
}

func (f *F) scopedProfile(spec scenarios.ProfileSpec) {
	f.t.Helper()

	if err := f.c.CreateProfile(f.ctx, &client.CreateProfileOptions{
		Name:           spec.Name,
		UeAmbrUplink:   spec.UeAmbrUplink,
		UeAmbrDownlink: spec.UeAmbrDownlink,
	}); err != nil {
		f.fatalf("create scoped profile %q: %v", spec.Name, err)
	}

	name := spec.Name

	f.t.Cleanup(func() {
		if err := f.c.DeleteProfile(f.ctx, &client.DeleteProfileOptions{Name: name}); err != nil {
			f.t.Logf("cleanup: delete profile %q: %v", name, err)
		}
	})
}

func (f *F) scopedSlice(spec scenarios.SliceSpec) {
	f.t.Helper()

	if err := f.c.CreateSlice(f.ctx, &client.CreateSliceOptions{
		Name: spec.Name,
		Sst:  spec.SST,
		Sd:   spec.SD,
	}); err != nil {
		f.fatalf("create scoped slice %q: %v", spec.Name, err)
	}

	name := spec.Name

	f.t.Cleanup(func() {
		if err := f.c.DeleteSlice(f.ctx, &client.DeleteSliceOptions{Name: name}); err != nil {
			f.t.Logf("cleanup: delete slice %q: %v", name, err)
		}
	})
}

func (f *F) scopedDataNetwork(spec scenarios.DataNetworkSpec) {
	f.t.Helper()

	if err := f.c.CreateDataNetwork(f.ctx, &client.CreateDataNetworkOptions{
		Name:   spec.Name,
		IPPool: spec.IPPool,
		DNS:    spec.DNS,
		Mtu:    spec.MTU,
	}); err != nil {
		f.fatalf("create scoped data network %q: %v", spec.Name, err)
	}

	name := spec.Name

	f.t.Cleanup(func() {
		if err := f.c.DeleteDataNetwork(f.ctx, &client.DeleteDataNetworkOptions{Name: name}); err != nil {
			f.t.Logf("cleanup: delete data network %q: %v", name, err)
		}
	})
}

func (f *F) scopedPolicy(spec scenarios.PolicySpec) {
	f.t.Helper()

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
		f.fatalf("create scoped policy %q: %v", spec.Name, err)
	}

	name := spec.Name

	f.t.Cleanup(func() {
		if err := f.c.DeletePolicy(f.ctx, &client.DeletePolicyOptions{Name: name}); err != nil {
			f.t.Logf("cleanup: delete policy %q: %v", name, err)
		}
	})
}

func (f *F) scopedSubscriber(spec scenarios.SubscriberSpec) {
	f.t.Helper()

	if err := f.c.CreateSubscriber(f.ctx, &client.CreateSubscriberOptions{
		Imsi:           spec.IMSI,
		Key:            spec.Key,
		SequenceNumber: spec.SequenceNumber,
		ProfileName:    spec.ProfileName,
		OPc:            spec.OPc,
	}); err != nil {
		f.fatalf("create scoped subscriber %q: %v", spec.IMSI, err)
	}

	imsi := spec.IMSI

	f.t.Cleanup(func() {
		if err := f.c.DeleteSubscriber(f.ctx, &client.DeleteSubscriberOptions{ID: imsi}); err != nil {
			f.t.Logf("cleanup: delete subscriber %q: %v", imsi, err)
		}
	})
}
