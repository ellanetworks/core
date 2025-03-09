package models

type PreemptionCapability string

const (
	PreemptionCapabilityNotPreempt PreemptionCapability = "NOT_PREEMPT"
	PreemptionCapabilityMayPreempt PreemptionCapability = "MAY_PREEMPT"
)
