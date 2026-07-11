// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

// FixtureSpec describes everything a scenario needs provisioned on Ella Core
// before it runs. Plain data only — no client-SDK types — so the scenarios
// package stays free of the Ella Core REST client dependency.
type FixtureSpec struct {
	// Operator, when non-nil, overrides the baseline operator config that the
	// integration test otherwise applies from scenarios.Default* values.
	Operator *OperatorSpec

	HomeNetworkKeys []HomeNetworkKeySpec

	// Profiles/Slices/DataNetworks/Policies add to the default set.
	// Idempotent: names already provisioned are left alone.
	Profiles     []ProfileSpec
	Slices       []SliceSpec
	DataNetworks []DataNetworkSpec
	Policies     []PolicySpec

	Subscribers []SubscriberSpec

	// StaticIPs are pinned after subscribers and data networks exist.
	StaticIPs []StaticIPSpec

	// ExtraArgs are passed verbatim to `core-tester run <scenario>`, for the
	// scenario-specific flags it declares via BindFlags.
	ExtraArgs []string

	// AssertUsageForIMSIs lists IMSIs whose uplink+downlink byte counters the
	// integration test polls and asserts > 0 post-scenario. Set only when the
	// scenario exercised the data plane.
	AssertUsageForIMSIs []string
}

type OperatorSpec struct {
	MCC           string
	MNC           string
	SupportedTACs []string
}

type HomeNetworkKeySpec struct {
	KeyIdentifier int
	Scheme        string
	// PrivateKey is hex-encoded X25519; Core derives and serves the public key.
	PrivateKey string
}

type ProfileSpec struct {
	Name           string
	UeAmbrUplink   string
	UeAmbrDownlink string
}

type SliceSpec struct {
	Name string
	SST  int
	SD   string
}

type DataNetworkSpec struct {
	Name     string
	IPv4Pool string
	IPv6Pool string
	DNS      string
	MTU      int32
}

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

type SubscriberSpec struct {
	IMSI           string
	Key            string
	OPc            string
	SequenceNumber string
	ProfileName    string
}

// StaticIPSpec pins an address to a subscriber for a data network. The IP
// version is inferred from the address family.
type StaticIPSpec struct {
	IMSI        string
	DataNetwork string
	Address     string
}

func DefaultSubscriber() SubscriberSpec {
	return SubscriberSpec{
		IMSI:           DefaultIMSI,
		Key:            DefaultKey,
		OPc:            DefaultOPC,
		SequenceNumber: DefaultSequenceNumber,
		ProfileName:    DefaultProfileName,
	}
}

func DefaultSubscriberWith(imsi string, profile string) SubscriberSpec {
	s := DefaultSubscriber()
	s.IMSI = imsi

	if profile != "" {
		s.ProfileName = profile
	}

	return s
}
