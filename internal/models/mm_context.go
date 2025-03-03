package models

type MmContext struct {
	AccessType           AccessType
	NasSecurityMode      *NasSecurityMode
	NasDownlinkCount     int32
	NasUplinkCount       int32
	UeSecurityCapability string
	AllowedNssai         []Snssai
}
