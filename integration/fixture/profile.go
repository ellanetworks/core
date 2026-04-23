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

// Profile upserts the profile: when a profile with the same name exists
// (e.g. Core's seeded "default"), its UE-AMBR is overwritten to match
// spec; otherwise the profile is created.
func (f *F) Profile(spec ProfileSpec) {
	f.t.Helper()

	if _, err := f.c.GetProfile(f.ctx, &client.GetProfileOptions{Name: spec.Name}); err == nil {
		if err := f.c.UpdateProfile(f.ctx, spec.Name, &client.UpdateProfileOptions{
			UeAmbrUplink:   spec.UeAmbrUplink,
			UeAmbrDownlink: spec.UeAmbrDownlink,
		}); err != nil {
			f.fatalf("update profile %q: %v", spec.Name, err)
		}

		return
	}

	if err := f.c.CreateProfile(f.ctx, &client.CreateProfileOptions{
		Name:           spec.Name,
		UeAmbrUplink:   spec.UeAmbrUplink,
		UeAmbrDownlink: spec.UeAmbrDownlink,
	}); err != nil {
		f.fatalf("create profile %q: %v", spec.Name, err)
	}
}
