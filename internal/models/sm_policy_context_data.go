package models

import (
	"time"
)

type SmPolicyContextData struct {
	Gpsi                    string
	Supi                    string
	InterGrpIds             []string
	PduSessionId            int32
	PduSessionType          PduSessionType
	Chargingcharacteristics string
	Dnn                     string
	NotificationUri         string
	AccessType              AccessType
	RatType                 RatType
	ServingNetwork          *NetworkId
	UeTimeZone              string
	Pei                     string
	Ipv4Address             string
	Ipv6AddressPrefix       string
	// Indicates the IPv4 address domain
	IpDomain     string
	SubsSessAmbr *Ambr
	SubsDefQos   *SubscribedDefaultQos
	// Contains the number of supported packet filter for signalled QoS rules.
	NumOfPackFilter int32
	// If it is included and set to true, the online charging is applied to the PDU session.
	Online bool
	// If it is included and set to true, the offline charging is applied to the PDU session.
	Offline bool
	// If it is included and set to true, the 3GPP PS Data Off is activated by the UE.
	Var3gppPsDataOffStatus bool
	// If it is included and set to true, the reflective QoS is supported by the UE.
	RefQosIndication bool
	SliceInfo        *Snssai
	QosFlowUsage     QosFlowUsage
	SuppFeat         string
	SmfId            string
	RecoveryTime     *time.Time
}
