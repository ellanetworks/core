package models

type QosData struct {
	QFI           uint8
	Var5qi        int32
	MaxbrUl       string
	MaxbrDl       string
	Arp           *Arp
	PriorityLevel int32
}
