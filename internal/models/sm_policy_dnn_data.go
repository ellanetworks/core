package models

// Contains the SM policy data for a given DNN (and S-NSSAI).
type SmPolicyDnnData struct {
	Dnn                 string
	AllowedServices     []string
	SubscCats           []string
	GbrUl               string
	GbrDl               string
	AdcSupport          bool
	SubscSpendingLimits bool
	Ipv4Index           int32
	Ipv6Index           int32
	Offline             bool
	Online              bool
	MpsPriority         bool
	ImsSignallingPrio   bool
	MpsPriorityLevel    int32
}
