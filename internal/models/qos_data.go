package models

type QosData struct {
	QFI           uint8
	Var5qi        int32
	MaxbrUl       string
	MaxbrDl       string
	Arp           *Arp
	PriorityLevel int32
	// Indicates that the dynamic PCC rule shall always have its binding with the QoS Flow associated with the default QoS rule
	DefQosFlowIndication bool
}
