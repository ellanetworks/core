package scenarios

// Default values shared between scenario runners and the integration
// fixture package. The integration suite provisions Core with these values;
// scenarios consume them directly.
const (
	DefaultMCC = "001"
	DefaultMNC = "01"
	DefaultSST = 1
	DefaultSD  = "102030"
	DefaultTAC = "000001"
	DefaultDNN = "internet"

	DefaultProfileName               = "default"
	DefaultSliceName                 = "default"
	DefaultPolicyName                = "default"
	DefaultProfileUeAmbrUplink       = "100 Mbps"
	DefaultProfileUeAmbrDownlink     = "100 Mbps"
	DefaultPolicySessionAmbrUplink   = "100 Mbps"
	DefaultPolicySessionAmbrDownlink = "100 Mbps"

	DefaultIMSI           = "001017271246546"
	DefaultKey            = "640f441067cd56f1474cbcacd7a0588f"
	DefaultOPC            = "cb698a2341629c3241ae01de9d89de4f"
	DefaultSequenceNumber = "000000000022"

	DefaultUEIPPool = "10.45.0.0/16"
	DefaultDNS      = "8.8.8.8"
	DefaultMTU      = 1500

	DefaultGNBID            = "000008"
	DefaultRANUENGAPID      = 1
	DefaultPDUSessionID     = 1
	DefaultAMF              = "80000000000000000000000000000000"
	DefaultRoutingIndicator = "0000"
	DefaultIMEISV           = "3569380356438091"

	DefaultPingDestination = "8.8.8.8"
)
