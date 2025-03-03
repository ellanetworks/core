package models

type SmPolicyContextData struct {
	Gpsi            string
	Supi            string
	PduSessionId    int32
	PduSessionType  PduSessionType
	Dnn             string
	NotificationUri string
	AccessType      AccessType
	RatType         RatType
	ServingNetwork  *PlmnId
	Pei             string
	Ipv4Address     string
	SubsSessAmbr    *Ambr
	SubsDefQos      *SubscribedDefaultQos
	// If it is included and set to true, the online charging is applied to the PDU session.
	Online bool
	// If it is included and set to true, the offline charging is applied to the PDU session.
	Offline   bool
	SliceInfo *Snssai
	SuppFeat  string
}
