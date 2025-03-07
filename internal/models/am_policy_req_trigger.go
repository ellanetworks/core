package models

type AmPolicyReqTrigger string

// List of AMPolicyReqTrigger
const (
	AmPolicyReqTriggerLocationChange  AmPolicyReqTrigger = "LOCATION_CHANGE"
	AmPolicyReqTriggerPraChange       AmPolicyReqTrigger = "PRA_CHANGE"
	AmPolicyReqTriggerSariChange      AmPolicyReqTrigger = "SARI_CHANGE"
	AmPolicyReqTriggerRfspIndexChange AmPolicyReqTrigger = "RFSP_INDEX_CHANGE"
)
