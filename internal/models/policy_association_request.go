package models

type PolicyAssociationRequest struct {
	NotificationUri string
	// Alternate or backup IPv4 Address(es) where to send Notifications.
	AltNotifIpv4Addrs []string
	// Alternate or backup IPv6 Address(es) where to send Notifications.
	AltNotifIpv6Addrs []string
	Supi              string
	Gpsi              string
	AccessType        AccessType
	Pei               string
	UserLoc           *UserLocation
	TimeZone          string
	ServingPlmn       *NetworkId
	RatType           RatType
	GroupIds          []string
	ServAreaRes       *ServiceAreaRestriction
	Rfsp              int32
	Guami             *Guami
	// If the NF service consumer is an AMF, it should provide the name of a service produced by the AMF that makes use of information received within the Npcf_AMPolicyControl_UpdateNotify service operation.
	ServiveName string
	// TraceReq    *TraceData `json:"traceReq,omitempty" yaml:"traceReq" bson:"traceReq" mapstructure:"TraceReq"`
	SuppFeat string
}
