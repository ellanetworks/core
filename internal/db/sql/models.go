// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package sql

type DeviceGroup struct {
	ID               int64
	Name             string
	SiteInfo         string
	IpDomainName     string
	Dnn              string
	UeIpPool         string
	DnsPrimary       string
	Mtu              int64
	DnnMbrUplink     int64
	DnnMbrDownlink   int64
	TrafficClassName string
	TrafficClassArp  int64
	TrafficClassPdb  int64
	TrafficClassPelr int64
	TrafficClassQci  int64
}

type DeviceGroupSubscriber struct {
	DeviceGroupID int64
	SubscriberID  int64
}

type Subscriber struct {
	ID             int64
	Imsi           string
	PlmnID         string
	Opc            string
	Key            string
	SequenceNumber string
}
