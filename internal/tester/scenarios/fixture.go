package scenarios

// FixtureSpec describes everything a scenario needs provisioned on Ella
// Core before it runs. Scenarios own their fixture spec so the
// integration test has a single source of truth per scenario.
//
// Plain data only — no client-SDK types — so the scenarios package stays
// free of the Ella Core REST client dependency.
type FixtureSpec struct {
	// Operator, if non-nil, overrides the baseline operator config.
	// Usually nil; the integration test applies scenarios.Default* values
	// once at test startup.
	Operator *OperatorSpec

	// HomeNetworkKeys to provision. Integration test creates each key
	// if it does not already exist.
	HomeNetworkKeys []HomeNetworkKeySpec

	// Profiles/Slices/DataNetworks/Policies to create in addition to the
	// default set. Idempotent: names already provisioned are left alone.
	Profiles     []ProfileSpec
	Slices       []SliceSpec
	DataNetworks []DataNetworkSpec
	Policies     []PolicySpec

	// Subscribers to provision. Each is created with its named profile.
	Subscribers []SubscriberSpec

	// ExtraArgs are passed verbatim as additional arguments to `core-tester
	// run <scenario>`, for scenario-specific flags the scenario declares
	// via BindFlags.
	ExtraArgs []string

	// AssertUsageForIMSIs lists IMSIs the integration test must poll via
	// the usage API post-scenario. For each IMSI, assert uplink + downlink
	// byte counters are > 0. Only set when the scenario exercised the
	// data plane.
	AssertUsageForIMSIs []string
}

// OperatorSpec, if set, overrides the default operator ID + TACs.
type OperatorSpec struct {
	MCC           string
	MNC           string
	SupportedTACs []string
}

// HomeNetworkKeySpec describes one home-network key to provision.
type HomeNetworkKeySpec struct {
	KeyIdentifier int
	Scheme        string
	// PrivateKey is a hex-encoded X25519 private key. The public key is
	// derived by Core and served over the API.
	PrivateKey string
}

// ProfileSpec describes a subscriber profile.
type ProfileSpec struct {
	Name           string
	UeAmbrUplink   string
	UeAmbrDownlink string
}

// SliceSpec describes a network slice.
type SliceSpec struct {
	Name string
	SST  int
	SD   string
}

// DataNetworkSpec describes a PDU session anchor.
type DataNetworkSpec struct {
	Name   string
	IPPool string
	DNS    string
	MTU    int32
}

// PolicySpec describes a QoS policy.
type PolicySpec struct {
	Name                string
	ProfileName         string
	SliceName           string
	DataNetworkName     string
	SessionAmbrUplink   string
	SessionAmbrDownlink string
	Var5qi              int32
	Arp                 int32
}

// SubscriberSpec describes one subscriber.
type SubscriberSpec struct {
	IMSI           string
	Key            string
	OPc            string
	SequenceNumber string
	ProfileName    string
}

// DefaultSubscriber is the stock subscriber every scenario that registers a
// single UE uses. IMSI matches scenarios.DefaultIMSI.
func DefaultSubscriber() SubscriberSpec {
	return SubscriberSpec{
		IMSI:           DefaultIMSI,
		Key:            DefaultKey,
		OPc:            DefaultOPC,
		SequenceNumber: DefaultSequenceNumber,
		ProfileName:    DefaultProfileName,
	}
}

// DefaultSubscriberWith returns DefaultSubscriber with a different IMSI and
// optionally a different profile.
func DefaultSubscriberWith(imsi string, profile string) SubscriberSpec {
	s := DefaultSubscriber()
	s.IMSI = imsi

	if profile != "" {
		s.ProfileName = profile
	}

	return s
}
