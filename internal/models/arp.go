package models

type Arp struct {
	// nullable true shall not be used for this attribute
	PriorityLevel int32                   `json:"priorityLevel" yaml:"priorityLevel" bson:"priorityLevel" mapstructure:"PriorityLevel"`
	PreemptCap    PreemptionCapability    `json:"preemptCap" yaml:"preemptCap" bson:"preemptCap" mapstructure:"PreemptCap"`
	PreemptVuln   PreemptionVulnerability `json:"preemptVuln" yaml:"preemptVuln" bson:"preemptVuln" mapstructure:"PreemptVuln"`
}
