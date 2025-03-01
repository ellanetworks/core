package models

type PolicyAssociationUpdateRequest struct {
	NotificationUri string `json:"notificationUri,omitempty" yaml:"notificationUri" bson:"notificationUri" mapstructure:"NotificationUri"`
	// Alternate or backup IPv4 Address(es) where to send Notifications.
	AltNotifIpv4Addrs []string `json:"altNotifIpv4Addrs,omitempty" yaml:"altNotifIpv4Addrs" bson:"altNotifIpv4Addrs" mapstructure:"AltNotifIpv4Addrs"`
	// Alternate or backup IPv6 Address(es) where to send Notifications.
	AltNotifIpv6Addrs []string `json:"altNotifIpv6Addrs,omitempty" yaml:"altNotifIpv6Addrs" bson:"altNotifIpv6Addrs" mapstructure:"AltNotifIpv6Addrs"`
	// Request Triggers that the NF service consumer observes.
	Triggers    []RequestTrigger        `json:"triggers,omitempty" yaml:"triggers" bson:"triggers" mapstructure:"Triggers"`
	ServAreaRes *ServiceAreaRestriction `json:"servAreaRes,omitempty" yaml:"servAreaRes" bson:"servAreaRes" mapstructure:"ServAreaRes"`
	Rfsp        int32                   `json:"rfsp,omitempty" yaml:"rfsp" bson:"rfsp" mapstructure:"Rfsp"`
	// Map of PRA status information.
	PraStatuses map[string]PresenceInfo `json:"praStatuses,omitempty" yaml:"praStatuses" bson:"praStatuses" mapstructure:"PraStatuses"`
	UserLoc     *UserLocation           `json:"userLoc,omitempty" yaml:"userLoc" bson:"userLoc" mapstructure:"UserLoc"`
	// TraceReq    *TraceData              `json:"traceReq,omitempty" yaml:"traceReq" bson:"traceReq" mapstructure:"TraceReq"`
}
