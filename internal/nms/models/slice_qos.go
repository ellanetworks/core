package models

type SliceQos struct {
	// uplink data rate in bps
	Uplink int32 `json:"uplink,omitempty"`

	// downlink data rate in bps
	Downlink int32 `json:"downlink,omitempty"`

	// data rate unit for uplink and downlink
	BitrateUnit string `json:"bitrate-unit,omitempty"`

	// QCI/QFI for the traffic
	TrafficClass string `json:"traffic-class,omitempty"`
}
