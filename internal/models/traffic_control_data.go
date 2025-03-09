package models

type TrafficControlData struct {
	// Univocally identifies the traffic control policy data within a PDU session.
	TcID       string
	FlowStatus FlowStatus
}
