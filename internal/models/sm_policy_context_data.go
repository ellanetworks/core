package models

type SmPolicyContextData struct {
	Supi           string
	Dnn            string
	AccessType     AccessType
	ServingNetwork *PlmnID
	SliceInfo      *Snssai
}
