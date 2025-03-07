package models

type SmPolicyContextData struct {
	Gpsi           string
	Supi           string
	PduSessionID   int32
	PduSessionType PduSessionType
	Dnn            string
	AccessType     AccessType
	RatType        RatType
	ServingNetwork *PlmnID
	Pei            string
	Ipv4Address    string
	SubsSessAmbr   *Ambr
	SubsDefQos     *SubscribedDefaultQos
	SliceInfo      *Snssai
}
