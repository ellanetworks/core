package models

type Arp struct {
	// nullable true shall not be used for this attribute
	PriorityLevel int32
	PreemptCap    PreemptionCapability
	PreemptVuln   PreemptionVulnerability
}
