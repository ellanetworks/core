package models

type AmPolicyReqTrigger string

// List of AMPolicyReqTrigger
const (
	AmPolicyReqTrigger_LOCATION_CHANGE   AmPolicyReqTrigger = "LOCATION_CHANGE"
	AmPolicyReqTrigger_PRA_CHANGE        AmPolicyReqTrigger = "PRA_CHANGE"
	AmPolicyReqTrigger_SARI_CHANGE       AmPolicyReqTrigger = "SARI_CHANGE"
	AmPolicyReqTrigger_RFSP_INDEX_CHANGE AmPolicyReqTrigger = "RFSP_INDEX_CHANGE"
)
