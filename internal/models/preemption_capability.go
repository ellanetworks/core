package models

type PreemptionCapability string

// List of PreemptionCapability
const (
	PreemptionCapability_NOT_PREEMPT PreemptionCapability = "NOT_PREEMPT"
	PreemptionCapability_MAY_PREEMPT PreemptionCapability = "MAY_PREEMPT"
)
