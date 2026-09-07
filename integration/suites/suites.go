// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package suites

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Cell struct {
	Arch       string `json:"arch"`
	IPFamily   string `json:"ip_version"`
	AttachMode string `json:"attach_mode"`
}

type Profile struct {
	Name  string
	Cells []Cell
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

var Profiles = []Profile{
	ProfileFull,
	ProfileClusterFamilies,
	ProfileFamiliesAttach,
	ProfileAttachPlusV6,
	ProfileAttach,
	ProfileFamilies,
	ProfileMinimal,
}

var SubtestPartitioned = map[string]string{
	"TestIntegrationTester": "s1enb",
}

type Suite struct {
	Name        string
	Profile     Profile
	Timeout     string
	NeedsTester bool
	Run         string
	Skip        string
	Tests       []string
}

var All = []Suite{
	{
		Name:        "datapath-4g",
		NeedsTester: true,
		Profile:     ProfileFull,
		Timeout:     "30m",
		Run:         "TestIntegrationTester/s1enb|TestIntegration4GNetworkRules",
		Tests:       []string{"TestIntegrationTester", "TestIntegration4GNetworkRules"},
	},
	{
		Name:        "datapath-5g",
		NeedsTester: true,
		Profile:     ProfileFull,
		Timeout:     "30m",
		Run:         "TestIntegrationTester|TestIntegration5G",
		Skip:        "TestIntegrationTester/s1enb|TestIntegration5GHAFailover|TestIntegration5GBGP|TestIntegration5GFramedRouting|TestIntegration5GUE2UE",
		Tests: []string{
			"TestIntegrationTester",
			"TestIntegration5GBufferedDownlink",
			"TestIntegration5GFlowReportsSmoke",
			"TestIntegration5GMultiGNB",
			"TestIntegration5GN2Handover",
			"TestIntegration5GNetworkRulesAndFlowReports",
			"TestIntegration5GUERANSIM",
			"TestIntegration5GUPFNATChecksum",
			"TestIntegration5GXnHandover",
		},
	},
	{
		Name:        "srsran-4g",
		NeedsTester: true,
		Profile:     ProfileAttach,
		Timeout:     "30m",
		Run:         "TestIntegration4G",
		Skip:        "TestIntegration4GHAFailover|TestIntegration4GNetworkRules|TestIntegration4GUE2UE|TestIntegration4GBGP|TestIntegration4GFramedRouting|TestIntegration4GS1Handover|TestIntegration4GX2Handover",
		Tests: []string{
			"TestIntegration4GAttach",
			"TestIntegration4GAuthMACFailure",
			"TestIntegration4GBufferedDownlink",
			"TestIntegration4GDataNetworkChanges",
			"TestIntegration4GDetach",
			"TestIntegration4GIdle",
			"TestIntegration4GMultiPDN",
			"TestIntegration4GNetworkDetach",
			"TestIntegration4GS1Reset",
			"TestIntegration4GS1Setup",
			"TestIntegration4GServiceRequest",
			"TestIntegration4GSessionModification",
			"TestIntegration4GUPFNATChecksum",
			"TestIntegration4GUnknownIMSI",
			"TestIntegration4GUserPlane",
			"TestIntegration4GUserPlaneIPv6",
		},
	},
	{
		Name:        "ue2ue",
		NeedsTester: true,
		Profile:     ProfileAttach,
		Timeout:     "20m",
		Run:         "TestIntegration4GUE2UE|TestIntegration5GUE2UE",
		Tests:       []string{"TestIntegration4GUE2UE", "TestIntegration5GUE2UE"},
	},
	{
		Name:        "framed",
		NeedsTester: true,
		Profile:     ProfileFamiliesAttach,
		Timeout:     "20m",
		Run:         "TestIntegration4GFramedRouting|TestIntegration5GFramedRouting",
		Tests: []string{
			"TestIntegration4GFramedRouting",
			"TestIntegration5GFramedRouting",
			"TestIntegration5GFramedRoutingReconcile",
		},
	},
	{
		Name:        "handover-4g",
		NeedsTester: true,
		Profile:     ProfileAttachPlusV6,
		Timeout:     "15m",
		Run:         "TestIntegration4GS1Handover|TestIntegration4GX2Handover",
		Tests:       []string{"TestIntegration4GS1Handover", "TestIntegration4GX2Handover"},
	},
	{
		Name:        "bgp-4g",
		NeedsTester: true,
		Profile:     ProfileAttachPlusV6,
		Timeout:     "15m",
		Run:         "TestIntegration4GBGP",
		Tests:       []string{"TestIntegration4GBGP"},
	},
	{
		Name:        "bgp-5g",
		NeedsTester: true,
		Profile:     ProfileFamilies,
		Timeout:     "15m",
		Run:         "TestIntegration5GBGP",
		Tests:       []string{"TestIntegration5GBGP"},
	},
	{
		Name:    "ha",
		Profile: ProfileClusterFamilies,
		Timeout: "15m",
		Run:     "TestIntegrationHA",
		Skip:    "TestIntegrationHARollingUpgrade",
		Tests: []string{
			"TestIntegrationHAClusterFormation",
			"TestIntegrationHADisasterRecovery",
			"TestIntegrationHADrainLeadership",
			"TestIntegrationHADrainResumeCycle",
			"TestIntegrationHAFollowerProxy",
			"TestIntegrationHAFollowerReturnsOnNewAddress",
			"TestIntegrationHAForceRemoveUnreachableMember",
			"TestIntegrationHAFreshClusterConcurrentBootstrap",
			"TestIntegrationHAJoinTokenRejection",
			"TestIntegrationHALeaderCrash",
			"TestIntegrationHALeaderFailure",
			"TestIntegrationHANetworkPartition",
			"TestIntegrationHAQuorumRecovery",
			"TestIntegrationHARemoveLeader",
			"TestIntegrationHAScaleUpDown",
			"TestIntegrationHASnapshotInstallOnNewJoiner",
		},
	},
	{
		Name:    "rolling-upgrade",
		Profile: ProfileMinimal,
		Timeout: "15m",
		Run:     "TestIntegrationHARollingUpgrade",
		Tests:   []string{"TestIntegrationHARollingUpgrade"},
	},
	{
		Name:        "ha-3gpp-4g",
		NeedsTester: true,
		Profile:     ProfileMinimal,
		Timeout:     "15m",
		Run:         "TestIntegration4GHAFailover",
		Tests:       []string{"TestIntegration4GHAFailover"},
	},
	{
		Name:        "ha-3gpp-5g",
		NeedsTester: true,
		Profile:     ProfileMinimal,
		Timeout:     "15m",
		Run:         "TestIntegration5GHAFailover",
		Tests:       []string{"TestIntegration5GHAFailover"},
	},
	{
		Name:        "api-matrix",
		NeedsTester: true,
		Profile:     ProfileMinimal,
		Timeout:     "10m",
		Run:         "^TestAPIMatrix$",
		Tests:       []string{"TestAPIMatrix"},
	},
	{
		Name:    "api-matrix-ha",
		Profile: ProfileMinimal,
		Timeout: "15m",
		Run:     "TestAPIMatrixHA",
		Tests:   []string{"TestAPIMatrixHA"},
	},
}

type Leg struct {
	Name           string `json:"name"`
	Suite          string `json:"suite"`
	Timeout        string `json:"timeout"`
	TimeoutMinutes int    `json:"timeout_minutes"`
	NeedsTester    bool   `json:"needs_tester"`
	Run            string `json:"run"`
	Skip           string `json:"skip"`
	Cell
}

const setupHeadroomMinutes = 15

func Legs() []Leg {
	out := make([]Leg, 0, 64)

	for _, s := range All {
		minutes, err := strconv.Atoi(strings.TrimSuffix(s.Timeout, "m"))
		if err != nil {
			panic(fmt.Sprintf("suite %s: timeout %q must be expressed in whole minutes", s.Name, s.Timeout))
		}

		for _, c := range s.Profile.Cells {
			out = append(out, Leg{
				Name:           fmt.Sprintf("%s (%s, %s, %s)", s.Name, c.Arch, c.IPFamily, c.AttachMode),
				Suite:          s.Name,
				Timeout:        s.Timeout,
				TimeoutMinutes: minutes + setupHeadroomMinutes,
				NeedsTester:    s.NeedsTester,
				Run:            s.Run,
				Skip:           s.Skip,
				Cell:           c,
			})
		}
	}

	return out
}

func Lookup(name string) (Suite, bool) {
	for _, s := range All {
		if s.Name == name {
			return s, true
		}
	}

	return Suite{}, false
}

func SplitTests() map[string][]string {
	out := make(map[string][]string)

	for _, s := range All {
		for _, t := range s.Tests {
			out[t] = append(out[t], s.Name)
		}
	}

	for _, v := range out {
		sort.Strings(v)
	}

	return out
}

func (p Profile) String() string {
	parts := make([]string, 0, len(p.Cells))

	for _, c := range p.Cells {
		parts = append(parts, fmt.Sprintf("%s/%s/%s", c.Arch, c.IPFamily, c.AttachMode))
	}

	return p.Name + "[" + strings.Join(parts, " ") + "]"
}
