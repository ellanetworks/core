package models

type PolicyAssociationUpdateRequest struct {
	NotificationUri string
	// Alternate or backup IPv4 Address(es) where to send Notifications.
	AltNotifIpv4Addrs []string
	// Alternate or backup IPv6 Address(es) where to send Notifications.
	AltNotifIpv6Addrs []string
	// Request Triggers that the NF service consumer observes.
	Triggers    []RequestTrigger
	ServAreaRes *ServiceAreaRestriction
	Rfsp        int32
	// Map of PRA status information.
	PraStatuses map[string]PresenceInfo
	UserLoc     *UserLocation
	// TraceReq    *TraceData              `json:"traceReq,omitempty" yaml:"traceReq" bson:"traceReq" mapstructure:"TraceReq"`
}
