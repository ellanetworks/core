package models

type SmPolicyContextData struct {
	Gpsi           string
	Supi           string
	PduSessionId   int32
	PduSessionType PduSessionType
	Dnn            string
	AccessType     AccessType
	RatType        RatType
	ServingNetwork *PlmnId
	Pei            string
	Ipv4Address    string
	SubsSessAmbr   *Ambr
	SubsDefQos     *SubscribedDefaultQos
	SliceInfo      *Snssai
}
