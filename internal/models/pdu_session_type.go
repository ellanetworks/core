package models

type PduSessionType string

// List of PduSessionType
const (
	PduSessionType_IPV4         PduSessionType = "IPV4"
	PduSessionType_IPV6         PduSessionType = "IPV6"
	PduSessionType_IPV4_V6      PduSessionType = "IPV4V6"
	PduSessionType_UNSTRUCTURED PduSessionType = "UNSTRUCTURED"
	PduSessionType_ETHERNET     PduSessionType = "ETHERNET"
)
