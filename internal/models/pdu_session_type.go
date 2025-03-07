package models

type PduSessionType string

const (
	PduSessionTypeIPv4         PduSessionType = "IPV4"
	PduSessionTypeIPv6         PduSessionType = "IPV6"
	PduSessionTypeIPv4v6       PduSessionType = "IPV4V6"
	PduSessionTypeUnstructured PduSessionType = "UNSTRUCTURED"
	PduSessionTypeEthernet     PduSessionType = "ETHERNET"
)
