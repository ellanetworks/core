package fixture

import (
	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// ProfileSpec describes a subscriber profile (UE-AMBR bucket).
type ProfileSpec struct {
	Name           string
	UeAmbrUplink   string
	UeAmbrDownlink string
}

// DefaultProfileSpec returns the scenarios-package default profile.
func DefaultProfileSpec() ProfileSpec {
	return ProfileSpec{
		Name:           scenarios.DefaultProfileName,
		UeAmbrUplink:   scenarios.DefaultProfileUeAmbrUplink,
		UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink,
	}
}

// Profile creates a profile. Idempotent: no-op if a profile with the same
// name already exists.
func (f *F) Profile(spec ProfileSpec) {
	f.t.Helper()

	err := f.c.CreateProfile(f.ctx, &client.CreateProfileOptions{
		Name:           spec.Name,
		UeAmbrUplink:   spec.UeAmbrUplink,
		UeAmbrDownlink: spec.UeAmbrDownlink,
	})
	if err != nil && !isAlreadyExists(err) {
		f.fatalf("create profile %q: %v", spec.Name, err)
	}
}
