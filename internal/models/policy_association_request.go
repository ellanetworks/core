package models

type PolicyAssociationRequest struct {
	NotificationUri string
	// Alternate or backup IPv4 Address(es) where to send Notifications.
	AltNotifIpv4Addrs []string
	// Alternate or backup IPv6 Address(es) where to send Notifications.
	AltNotifIpv6Addrs []string
	Supi              string
	Gpsi              string
	AccessType        AccessType
	Pei               string
	UserLoc           *UserLocation
	TimeZone          string
	ServingPlmn       *PlmnId
	GroupIds          []string
	ServAreaRes       *ServiceAreaRestriction
	Rfsp              int32
	Guami             *Guami
}
