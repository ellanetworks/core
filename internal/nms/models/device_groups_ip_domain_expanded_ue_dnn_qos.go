package models

// DeviceGroupsIpDomainExpandedUeDnnQos struct for DeviceGroupsIpDomainExpandedUeDnnQos
type DeviceGroupsIpDomainExpandedUeDnnQos struct {
	// uplink data rate
	DnnMbrUplink int64 `json:"dnn-mbr-uplink,omitempty"`
	// downlink data rate
	DnnMbrDownlink int64 `json:"dnn-mbr-downlink,omitempty"`
	// data rate unit for uplink and downlink
	BitrateUnit string `json:"bitrate-unit,omitempty"`
	// QCI/QFI for the traffic
	TrafficClass *TrafficClassInfo `json:"traffic-class,omitempty"`
}
