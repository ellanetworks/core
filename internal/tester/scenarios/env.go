package scenarios

// Env carries the common flag values to scenario runners.
//
// Populated by cmd/core-tester from the common flag families
// (--ella-core-n2-address, --gnb, --gnb-core-target).
type Env struct {
	// CoreN2Addresses are every value supplied via --ella-core-n2-address,
	// in argument order. Single-core scenarios consume the first entry.
	CoreN2Addresses []string

	// GNBs lists every gNB declared via --gnb, in argument order.
	GNBs []GNB

	// GNBCoreTargets maps gNB name → core N2 address for scenarios that
	// need explicit pairing. When empty, scenarios default to pairing gNB
	// i with CoreN2Addresses[i], or all cores for multihomed scenarios.
	GNBCoreTargets map[string]string
}

// GNB is one simulated gNB's address set.
type GNB struct {
	Name        string
	N2Address   string
	N3Address   string
	N3Secondary string
}

// FirstCore returns CoreN2Addresses[0], or "" when empty.
func (e Env) FirstCore() string {
	if len(e.CoreN2Addresses) == 0 {
		return ""
	}

	return e.CoreN2Addresses[0]
}

// FirstGNB returns GNBs[0], or a zero GNB when empty.
func (e Env) FirstGNB() GNB {
	if len(e.GNBs) == 0 {
		return GNB{}
	}

	return e.GNBs[0]
}
