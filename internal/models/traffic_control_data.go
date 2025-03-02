package models

type TrafficControlData struct {
	// Univocally identifies the traffic control policy data within a PDU session.
	TcId       string
	FlowStatus FlowStatus
	// RedirectInfo *RedirectInformation `json:"redirectInfo,omitempty" yaml:"redirectInfo" bson:"redirectInfo" mapstructure:"RedirectInfo"`
	// Indicates whether applicat'on's start or stop notification is to be muted.
	MuteNotif bool
	// Reference to a pre-configured traffic steering policy for downlink traffic at the SMF.
	TrafficSteeringPolIdDl string
	// Reference to a pre-configured traffic steering policy for uplink traffic at the SMF.
	TrafficSteeringPolIdUl string
}
