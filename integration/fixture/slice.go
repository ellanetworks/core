package fixture

import (
	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// SliceSpec describes a network slice.
type SliceSpec struct {
	Name string
	SST  int
	SD   string
}

// DefaultSliceSpec returns the scenarios-package default slice.
func DefaultSliceSpec() SliceSpec {
	return SliceSpec{
		Name: scenarios.DefaultSliceName,
		SST:  scenarios.DefaultSST,
		SD:   scenarios.DefaultSD,
	}
}

// Slice creates a slice, or verifies an existing one with the same name
// matches SST/SD. Fails the test on mismatch.
func (f *F) Slice(spec SliceSpec) {
	f.t.Helper()

	existing, err := f.c.GetSlice(f.ctx, &client.GetSliceOptions{Name: spec.Name})
	if err == nil {
		if existing.Sst != spec.SST || existing.Sd != spec.SD {
			f.fatalf("slice %q exists with different (SST, SD): have (%d, %q), want (%d, %q)",
				spec.Name, existing.Sst, existing.Sd, spec.SST, spec.SD)
		}

		return
	}

	if err := f.c.CreateSlice(f.ctx, &client.CreateSliceOptions{
		Name: spec.Name,
		Sst:  spec.SST,
		Sd:   spec.SD,
	}); err != nil {
		f.fatalf("create slice %q: %v", spec.Name, err)
	}
}
