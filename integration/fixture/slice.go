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

// Slice creates a slice. Idempotent: no-op if a slice with the same
// name already exists.
func (f *F) Slice(spec SliceSpec) {
	f.t.Helper()

	err := f.c.CreateSlice(f.ctx, &client.CreateSliceOptions{
		Name: spec.Name,
		Sst:  spec.SST,
		Sd:   spec.SD,
	})
	if err != nil && !isAlreadyExists(err) {
		f.fatalf("create slice %q: %v", spec.Name, err)
	}
}
