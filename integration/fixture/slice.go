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

func DefaultSliceSpec() SliceSpec {
	return SliceSpec{
		Name: scenarios.DefaultSliceName,
		SST:  scenarios.DefaultSST,
		SD:   scenarios.DefaultSD,
	}
}

// Slice upserts the baseline slice: when it already exists, its SST/SD
// are overwritten to match spec.
func (f *F) Slice(spec SliceSpec) {
	f.t.Helper()

	if _, err := f.c.GetSlice(f.ctx, &client.GetSliceOptions{Name: spec.Name}); err == nil {
		if err := f.c.UpdateSlice(f.ctx, spec.Name, &client.UpdateSliceOptions{
			Sst: spec.SST,
			Sd:  spec.SD,
		}); err != nil {
			f.fatalf("update slice %q: %v", spec.Name, err)
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
