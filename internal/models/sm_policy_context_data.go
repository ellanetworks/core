package models

import (
	"time"
)

type SmPolicyContextData struct {
	// AccNetChId              *AccNetChId            `json:"accNetChId,omitempty" yaml:"accNetChId" bson:"accNetChId" mapstructure:"AccNetChId"`
	// ChargEntityAddr         *AccNetChargingAddress `json:"chargEntityAddr,omitempty" yaml:"chargEntityAddr" bson:"chargEntityAddr" mapstructure:"ChargEntityAddr"`
	Gpsi                    string         `json:"gpsi,omitempty" yaml:"gpsi" bson:"gpsi" mapstructure:"Gpsi"`
	Supi                    string         `json:"supi" yaml:"supi" bson:"supi" mapstructure:"Supi"`
	InterGrpIds             []string       `json:"interGrpIds,omitempty" yaml:"interGrpIds" bson:"interGrpIds" mapstructure:"InterGrpIds"`
	PduSessionId            int32          `json:"pduSessionId" yaml:"pduSessionId" bson:"pduSessionId" mapstructure:"PduSessionId"`
	PduSessionType          PduSessionType `json:"pduSessionType" yaml:"pduSessionType" bson:"pduSessionType" mapstructure:"PduSessionType"`
	Chargingcharacteristics string         `json:"chargingcharacteristics,omitempty" yaml:"chargingcharacteristics" bson:"chargingcharacteristics" mapstructure:"Chargingcharacteristics"`
	Dnn                     string         `json:"dnn" yaml:"dnn" bson:"dnn" mapstructure:"Dnn"`
	NotificationUri         string         `json:"notificationUri" yaml:"notificationUri" bson:"notificationUri" mapstructure:"NotificationUri"`
	AccessType              AccessType     `json:"accessType,omitempty" yaml:"accessType" bson:"accessType" mapstructure:"AccessType"`
	RatType                 RatType        `json:"ratType,omitempty" yaml:"ratType" bson:"ratType" mapstructure:"RatType"`
	ServingNetwork          *NetworkId     `json:"servingNetwork,omitempty" yaml:"servingNetwork" bson:"servingNetwork" mapstructure:"ServingNetwork"`
	// UserLocationInfo        *UserLocation          `json:"userLocationInfo,omitempty" yaml:"userLocationInfo" bson:"userLocationInfo" mapstructure:"UserLocationInfo"`
	UeTimeZone        string `json:"ueTimeZone,omitempty" yaml:"ueTimeZone" bson:"ueTimeZone" mapstructure:"UeTimeZone"`
	Pei               string `json:"pei,omitempty" yaml:"pei" bson:"pei" mapstructure:"Pei"`
	Ipv4Address       string `json:"ipv4Address,omitempty" yaml:"ipv4Address" bson:"ipv4Address" mapstructure:"Ipv4Address"`
	Ipv6AddressPrefix string `json:"ipv6AddressPrefix,omitempty" yaml:"ipv6AddressPrefix" bson:"ipv6AddressPrefix" mapstructure:"Ipv6AddressPrefix"`
	// Indicates the IPv4 address domain
	IpDomain     string                `json:"ipDomain,omitempty" yaml:"ipDomain" bson:"ipDomain" mapstructure:"IpDomain"`
	SubsSessAmbr *Ambr                 `json:"subsSessAmbr,omitempty" yaml:"subsSessAmbr" bson:"subsSessAmbr" mapstructure:"SubsSessAmbr"`
	SubsDefQos   *SubscribedDefaultQos `json:"subsDefQos,omitempty" yaml:"subsDefQos" bson:"subsDefQos" mapstructure:"SubsDefQos"`
	// Contains the number of supported packet filter for signalled QoS rules.
	NumOfPackFilter int32 `json:"numOfPackFilter,omitempty" yaml:"numOfPackFilter" bson:"numOfPackFilter" mapstructure:"NumOfPackFilter"`
	// If it is included and set to true, the online charging is applied to the PDU session.
	Online bool `json:"online,omitempty" yaml:"online" bson:"online" mapstructure:"Online"`
	// If it is included and set to true, the offline charging is applied to the PDU session.
	Offline bool `json:"offline,omitempty" yaml:"offline" bson:"offline" mapstructure:"Offline"`
	// If it is included and set to true, the 3GPP PS Data Off is activated by the UE.
	Var3gppPsDataOffStatus bool `json:"3gppPsDataOffStatus,omitempty" yaml:"3gppPsDataOffStatus" bson:"3gppPsDataOffStatus" mapstructure:"Var3gppPsDataOffStatus"`
	// If it is included and set to true, the reflective QoS is supported by the UE.
	RefQosIndication bool `json:"refQosIndication,omitempty" yaml:"refQosIndication" bson:"refQosIndication" mapstructure:"RefQosIndication"`
	// TraceReq         *TraceData         `json:"traceReq,omitempty" yaml:"traceReq" bson:"traceReq" mapstructure:"TraceReq"`
	SliceInfo    *Snssai      `json:"sliceInfo" yaml:"sliceInfo" bson:"sliceInfo" mapstructure:"SliceInfo"`
	QosFlowUsage QosFlowUsage `json:"qosFlowUsage,omitempty" yaml:"qosFlowUsage" bson:"qosFlowUsage" mapstructure:"QosFlowUsage"`
	// ServNfId         *ServingNfIdentity `json:"servNfId,omitempty" yaml:"servNfId" bson:"servNfId" mapstructure:"ServNfId"`
	SuppFeat     string     `json:"suppFeat,omitempty" yaml:"suppFeat" bson:"suppFeat" mapstructure:"SuppFeat"`
	SmfId        string     `json:"smfId,omitempty" yaml:"smfId" bson:"smfId" mapstructure:"SmfId"`
	RecoveryTime *time.Time `json:"recoveryTime,omitempty" yaml:"recoveryTime" bson:"recoveryTime" mapstructure:"RecoveryTime"`
}
