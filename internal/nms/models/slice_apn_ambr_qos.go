package models

type ApnAmbrQosInfo struct {
	Uplink       int32  `json:"uplink-mbr,omitempty"`
	Downlink     int32  `json:"downlink-mbr,omitempty"`
	BitRateUnit  string `json:"bitrate-unit,omitempty"`
	TrafficClass string `json:"traffic-class,omitempty"`
}
