package models

type Cause string

// List of Cause
const (
	Cause_REL_DUE_TO_DUPLICATE_SESSION_ID Cause = "REL_DUE_TO_DUPLICATE_SESSION_ID"
	Cause_PDU_SESSION_STATUS_MISMATCH     Cause = "PDU_SESSION_STATUS_MISMATCH"
)
