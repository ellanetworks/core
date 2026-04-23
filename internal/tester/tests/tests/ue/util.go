package ue

import "github.com/ellanetworks/core/internal/tester/tests/tests/utils/core"

const (
	DefaultMCC                       = "001"
	DefaultMNC                       = "01"
	DefaultSST                       = 1
	DefaultSD                        = "102030"
	DefaultTAC                       = "000001"
	DefaultDNN                       = "internet"
	DefaultProfileName               = "default"
	DefaultSliceName                 = "default"
	DefaultPolicyName                = "default"
	DefaultPolicySessionAmbrUplink   = "100 Mbps"
	DefaultPolicySessionAmbrDownlink = "100 Mbps"
	DefaultProfileUeAmbrUplink       = "100 Mbps"
	DefaultProfileUeAmbrDownlink     = "100 Mbps"
	DefaultIMSI                      = "001017271246546"
	DefaultKey                       = "640f441067cd56f1474cbcacd7a0588f"
	DefaultOPC                       = "cb698a2341629c3241ae01de9d89de4f"
	DefaultSequenceNumber            = "000000000022"
)

func getDefaultEllaCoreConfig() core.EllaCoreConfig {
	return core.EllaCoreConfig{
		Operator: core.OperatorConfig{
			ID: core.OperatorID{
				MCC: DefaultMCC,
				MNC: DefaultMNC,
			},
			Tracking: core.OperatorTracking{
				SupportedTACs: []string{DefaultTAC},
			},
		},
		Profiles: []core.ProfileConfig{
			{
				Name:           DefaultProfileName,
				UeAmbrUplink:   DefaultProfileUeAmbrUplink,
				UeAmbrDownlink: DefaultProfileUeAmbrDownlink,
			},
		},
		Slices: []core.SliceConfig{
			{
				Name: DefaultSliceName,
				SST:  DefaultSST,
				SD:   DefaultSD,
			},
		},
		DataNetworks: []core.DataNetworkConfig{
			{
				Name:   DefaultDNN,
				IPPool: "10.45.0.0/16",
				DNS:    "8.8.8.8",
				Mtu:    1500,
			},
		},
		Policies: []core.PolicyConfig{
			{
				Name:                DefaultPolicyName,
				ProfileName:         DefaultProfileName,
				SliceName:           DefaultSliceName,
				SessionAmbrUplink:   DefaultPolicySessionAmbrUplink,
				SessionAmbrDownlink: DefaultPolicySessionAmbrDownlink,
				Var5qi:              9,
				Arp:                 15,
				DataNetworkName:     DefaultDNN,
			},
		},
		Subscribers: []core.SubscriberConfig{
			{
				Imsi:           DefaultIMSI,
				Key:            DefaultKey,
				SequenceNumber: DefaultSequenceNumber,
				OPc:            DefaultOPC,
				ProfileName:    DefaultProfileName,
			},
		},
	}
}
