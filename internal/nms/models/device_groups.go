package models

type DeviceGroups struct {
	DeviceGroupName string `json:"group-name"`

	Imsis []string `json:"imsis"`

	SiteInfo string `json:"site-info,omitempty"`

	IpDomainName string `json:"ip-domain-name,omitempty"`

	IpDomainExpanded DeviceGroupsIpDomainExpanded `json:"ip-domain-expanded,omitempty"`
}
