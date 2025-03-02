package models

type QosData struct {
	// Univocally identifies the QoS control policy data within a PDU session.
	QosId   string
	Var5qi  int32
	MaxbrUl string
	MaxbrDl string
	GbrUl   string
	GbrDl   string
	Arp     *Arp
	// Indicates whether notifications are requested from 3GPP NG-RAN when the GFBR can no longer (or again) be guaranteed for a QoS Flow during the lifetime of the QoS Flow.
	Qnc             bool
	PriorityLevel   int32
	AverWindow      int32
	MaxDataBurstVol int32
	// Indicates whether the QoS information is reflective for the corresponding service data flow.
	ReflectiveQos bool
	// Indicates, by containing the same value, what PCC rules may share resource in downlink direction.
	SharingKeyDl string
	// Indicates, by containing the same value, what PCC rules may share resource in uplink direction.
	SharingKeyUl        string
	MaxPacketLossRateDl int32
	MaxPacketLossRateUl int32
	// Indicates that the dynamic PCC rule shall always have its binding with the QoS Flow associated with the default QoS rule
	DefQosFlowIndication bool
}
