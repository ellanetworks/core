// Package all blank-imports every scenario subpackage so their init()
// registrations execute when the top-level binary imports this package.
//
// cmd/core-tester imports this package for side effects only.
package all

import (
	// Individual scenario packages:
	_ "github.com/ellanetworks/core/internal/tester/scenarios/enb"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/gnb"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/ha"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/multi"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/ue"
)
