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

// Profile creates a profile, or verifies an existing one with the same
// name matches the spec. Fails the test on mismatch so earlier fixture
// calls cannot silently shadow a later scenario's requirements.
func (f *F) Profile(spec ProfileSpec) {
	f.t.Helper()

	existing, err := f.c.GetProfile(f.ctx, &client.GetProfileOptions{Name: spec.Name})
	if err == nil {
		if existing.UeAmbrUplink != spec.UeAmbrUplink || existing.UeAmbrDownlink != spec.UeAmbrDownlink {
			f.fatalf("profile %q exists with different AMBR: have (up=%q down=%q), want (up=%q down=%q)",
				spec.Name,
				existing.UeAmbrUplink, existing.UeAmbrDownlink,
				spec.UeAmbrUplink, spec.UeAmbrDownlink)
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
