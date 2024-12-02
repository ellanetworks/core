package models

// DeviceGroupsIpDomainExpanded - This is APN for device
type DeviceGroupsIpDomainExpanded struct {
	Dnn string `json:"dnn,omitempty"`

	UeIpPool string `json:"ue-ip-pool,omitempty"`

	DnsPrimary string `json:"dns-primary,omitempty"`

	DnsSecondary string `json:"dns-secondary,omitempty"`

	Mtu int32 `json:"mtu,omitempty"`

	UeDnnQos *DeviceGroupsIpDomainExpandedUeDnnQos `json:"ue-dnn-qos,omitempty"`
}
