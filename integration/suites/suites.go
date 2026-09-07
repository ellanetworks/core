// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package suites

type Cell struct {
	Arch       string `json:"arch"`
	IPFamily   string `json:"ip_version"`
	AttachMode string `json:"attach_mode"`
}

type Profile struct {
	Name  string `json:"name"`
	Cells []Cell `json:"cells"`
}

var (
	ProfileFull = Profile{
		Name: "full",
		Cells: []Cell{
			{"amd64", "ipv4", "xdp-generic"},
			{"amd64", "ipv6", "xdp-generic"},
			{"amd64", "dualstack", "xdp-generic"},
			{"arm64", "ipv4", "xdp-generic"},
			{"arm64", "ipv6", "xdp-generic"},
			{"arm64", "dualstack", "xdp-generic"},
			{"amd64", "ipv4", "tcx"},
			{"amd64", "ipv6", "tcx"},
			{"arm64", "ipv4", "tcx"},
		},
	}

	ProfileClusterFamilies = Profile{
		Name: "cluster-families",
		Cells: []Cell{
			{"amd64", "ipv4", "xdp-generic"},
			{"amd64", "ipv6", "xdp-generic"},
			{"amd64", "dualstack", "xdp-generic"},
		},
	}

	ProfileFamiliesAttach = Profile{
		Name: "families-attach",
		Cells: []Cell{
			{"amd64", "ipv4", "xdp-generic"},
			{"amd64", "ipv4", "tcx"},
			{"amd64", "ipv6", "xdp-generic"},
			{"amd64", "ipv6", "tcx"},
		},
	}

	ProfileAttachPlusV6 = Profile{
		Name: "attach-plus-v6",
		Cells: []Cell{
			{"amd64", "ipv4", "xdp-generic"},
			{"amd64", "ipv4", "tcx"},
			{"amd64", "ipv6", "xdp-generic"},
		},
	}

	ProfileAttach = Profile{
		Name: "attach",
		Cells: []Cell{
			{"amd64", "ipv4", "xdp-generic"},
			{"amd64", "ipv4", "tcx"},
		},
	}

	ProfileFamilies = Profile{
		Name: "families",
		Cells: []Cell{
			{"amd64", "ipv4", "xdp-generic"},
			{"amd64", "ipv6", "xdp-generic"},
		},
	}

	ProfileMinimal = Profile{
		Name:  "minimal",
		Cells: []Cell{{"amd64", "ipv4", "xdp-generic"}},
	}
)

type Name string

const (
	Datapath4G     Name = "datapath-4g"
	Datapath5G     Name = "datapath-5g"
	SRSRAN4G       Name = "srsran-4g"
	UE2UE          Name = "ue2ue"
	Framed         Name = "framed"
	Handover4G     Name = "handover-4g"
	BGP4G          Name = "bgp-4g"
	BGP5G          Name = "bgp-5g"
	HA             Name = "ha"
	RollingUpgrade Name = "rolling-upgrade"
	HA3GPP4G       Name = "ha-3gpp-4g"
	HA3GPP5G       Name = "ha-3gpp-5g"
	APIMatrix      Name = "api-matrix"
	APIMatrixHA    Name = "api-matrix-ha"
)

type Definition struct {
	Profile     Profile
	Timeout     string
	NeedsTester bool
}

var Definitions = map[Name]Definition{
	Datapath4G:     {ProfileFull, "30m", true},
	Datapath5G:     {ProfileFull, "30m", true},
	SRSRAN4G:       {ProfileAttach, "30m", true},
	UE2UE:          {ProfileAttach, "20m", true},
	Framed:         {ProfileFamiliesAttach, "20m", true},
	Handover4G:     {ProfileAttachPlusV6, "15m", true},
	BGP4G:          {ProfileAttachPlusV6, "15m", true},
	BGP5G:          {ProfileFamilies, "15m", true},
	HA:             {ProfileClusterFamilies, "15m", false},
	RollingUpgrade: {ProfileMinimal, "15m", false},
	HA3GPP4G:       {ProfileMinimal, "15m", true},
	HA3GPP5G:       {ProfileMinimal, "15m", true},
	APIMatrix:      {ProfileMinimal, "10m", true},
	APIMatrixHA:    {ProfileMinimal, "15m", false},
}
