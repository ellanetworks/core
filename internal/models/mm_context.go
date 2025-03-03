package models

type MmContext struct {
	AccessType            AccessType       `json:"accessType"`
	NasSecurityMode       *NasSecurityMode `json:"nasSecurityMode,omitempty"`
	NasDownlinkCount      int32            `json:"nasDownlinkCount,omitempty"`
	NasUplinkCount        int32            `json:"nasUplinkCount,omitempty"`
	UeSecurityCapability  string           `json:"ueSecurityCapability,omitempty"`
	S1UeNetworkCapability string           `json:"s1UeNetworkCapability,omitempty"`
	AllowedNssai          []Snssai         `json:"allowedNssai,omitempty"`
	// NssaiMappingList      []NssaiMapping      `json:"nssaiMappingList,omitempty"`
	// NsInstanceList        []string            `json:"nsInstanceList,omitempty"`
	// ExpectedUEbehavior    *ExpectedUeBehavior `json:"expectedUEbehavior,omitempty"`
}
