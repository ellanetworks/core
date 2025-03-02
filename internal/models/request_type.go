package models

type RequestType string

// List of RequestType
const (
	RequestType_INITIAL_REQUEST                RequestType = "INITIAL_REQUEST"
	RequestType_EXISTING_PDU_SESSION           RequestType = "EXISTING_PDU_SESSION"
	RequestType_INITIAL_EMERGENCY_REQUEST      RequestType = "INITIAL_EMERGENCY_REQUEST"
	RequestType_EXISTING_EMERGENCY_PDU_SESSION RequestType = "EXISTING_EMERGENCY_PDU_SESSION"
)
