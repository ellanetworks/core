package gnb

import "github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/core"

const (
	DefaultMCC            = "001"
	DefaultMNC            = "01"
	DefaultSST            = 1
	DefaultSD             = "102030"
	DefaultTAC            = "000001"
	DefaultDNN            = "internet"
	DefaultIMSI           = "001017271246546"
	DefaultKey            = "640f441067cd56f1474cbcacd7a0588f"
	DefaultOPC            = "cb698a2341629c3241ae01de9d89de4f"
	DefaultSequenceNumber = "000000000022"
)

func getDefaultEllaCoreConfig() core.EllaCoreConfig {
	return core.EllaCoreConfig{
		Operator: core.OperatorConfig{
			ID: core.OperatorID{
				MCC: DefaultMCC,
				MNC: DefaultMNC,
			},
			Slice: core.OperatorSlice{
				SST: DefaultSST,
				SD:  DefaultSD,
			},
			Tracking: core.OperatorTracking{
				SupportedTACs: []string{DefaultTAC},
			},
		},
	}
}
