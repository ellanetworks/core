package models

type PolicyAssociationRequest struct {
	NotificationUri string `json:"notificationUri" yaml:"notificationUri" bson:"notificationUri" mapstructure:"NotificationUri"`
	// Alternate or backup IPv4 Address(es) where to send Notifications.
	AltNotifIpv4Addrs []string `json:"altNotifIpv4Addrs,omitempty" yaml:"altNotifIpv4Addrs" bson:"altNotifIpv4Addrs" mapstructure:"AltNotifIpv4Addrs"`
	// Alternate or backup IPv6 Address(es) where to send Notifications.
	AltNotifIpv6Addrs []string                `json:"altNotifIpv6Addrs,omitempty" yaml:"altNotifIpv6Addrs" bson:"altNotifIpv6Addrs" mapstructure:"AltNotifIpv6Addrs"`
	Supi              string                  `json:"supi" yaml:"supi" bson:"supi" mapstructure:"Supi"`
	Gpsi              string                  `json:"gpsi,omitempty" yaml:"gpsi" bson:"gpsi" mapstructure:"Gpsi"`
	AccessType        AccessType              `json:"accessType,omitempty" yaml:"accessType" bson:"accessType" mapstructure:"AccessType"`
	Pei               string                  `json:"pei,omitempty" yaml:"pei" bson:"pei" mapstructure:"Pei"`
	UserLoc           *UserLocation           `json:"userLoc,omitempty" yaml:"userLoc" bson:"userLoc" mapstructure:"UserLoc"`
	TimeZone          string                  `json:"timeZone,omitempty" yaml:"timeZone" bson:"timeZone" mapstructure:"TimeZone"`
	ServingPlmn       *NetworkId              `json:"servingPlmn,omitempty" yaml:"servingPlmn" bson:"servingPlmn" mapstructure:"ServingPlmn"`
	RatType           RatType                 `json:"ratType,omitempty" yaml:"ratType" bson:"ratType" mapstructure:"RatType"`
	GroupIds          []string                `json:"groupIds,omitempty" yaml:"groupIds" bson:"groupIds" mapstructure:"GroupIds"`
	ServAreaRes       *ServiceAreaRestriction `json:"servAreaRes,omitempty" yaml:"servAreaRes" bson:"servAreaRes" mapstructure:"ServAreaRes"`
	Rfsp              int32                   `json:"rfsp,omitempty" yaml:"rfsp" bson:"rfsp" mapstructure:"Rfsp"`
	Guami             *Guami                  `json:"guami,omitempty" yaml:"guami" bson:"guami" mapstructure:"Guami"`
	// If the NF service consumer is an AMF, it should provide the name of a service produced by the AMF that makes use of information received within the Npcf_AMPolicyControl_UpdateNotify service operation.
	ServiveName string `json:"serviveName,omitempty" yaml:"serviveName" bson:"serviveName" mapstructure:"ServiveName"`
	// TraceReq    *TraceData `json:"traceReq,omitempty" yaml:"traceReq" bson:"traceReq" mapstructure:"TraceReq"`
	SuppFeat string `json:"suppFeat" yaml:"suppFeat" bson:"suppFeat" mapstructure:"SuppFeat"`
}
