package models

type TrafficControlData struct {
	// Univocally identifies the traffic control policy data within a PDU session.
	TcId       string     `json:"tcId" yaml:"tcId" bson:"tcId" mapstructure:"TcId"`
	FlowStatus FlowStatus `json:"flowStatus,omitempty" yaml:"flowStatus" bson:"flowStatus" mapstructure:"FlowStatus"`
	// RedirectInfo *RedirectInformation `json:"redirectInfo,omitempty" yaml:"redirectInfo" bson:"redirectInfo" mapstructure:"RedirectInfo"`
	// Indicates whether applicat'on's start or stop notification is to be muted.
	MuteNotif bool `json:"muteNotif,omitempty" yaml:"muteNotif" bson:"muteNotif" mapstructure:"MuteNotif"`
	// Reference to a pre-configured traffic steering policy for downlink traffic at the SMF.
	TrafficSteeringPolIdDl string `json:"trafficSteeringPolIdDl,omitempty" yaml:"trafficSteeringPolIdDl" bson:"trafficSteeringPolIdDl" mapstructure:"TrafficSteeringPolIdDl"`
	// Reference to a pre-configured traffic steering policy for uplink traffic at the SMF.
	TrafficSteeringPolIdUl string `json:"trafficSteeringPolIdUl,omitempty" yaml:"trafficSteeringPolIdUl" bson:"trafficSteeringPolIdUl" mapstructure:"TrafficSteeringPolIdUl"`
	// A list of location which the traffic shall be routed to for the AF request
	// RouteToLocs    []RouteToLocation `json:"routeToLocs,omitempty" yaml:"routeToLocs" bson:"routeToLocs" mapstructure:"RouteToLocs"`
	// UpPathChgEvent *UpPathChgEvent   `json:"upPathChgEvent,omitempty" yaml:"upPathChgEvent" bson:"upPathChgEvent" mapstructure:"UpPathChgEvent"`
}
