package models

type Subscriber struct {
	IMSI           string `json:"imsi"`
	PlmnID         string `json:"plmnID"`
	OPc            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
}
