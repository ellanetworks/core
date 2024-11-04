// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package sql

import (
	"database/sql"
)

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
	NetworkSliceID   sql.NullInt64
}

type NetworkSlice struct {
	ID       int64
	Name     string
	Sst      int64
	Sd       string
	SiteName string
	Mcc      string
	Mnc      string
}

type Radio struct {
	ID             int64
	Name           string
	Tac            string
	NetworkSliceID sql.NullInt64
}

type Subscriber struct {
	ID             int64
	Imsi           string
	PlmnID         string
	Opc            string
	Key            string
	SequenceNumber string
	DeviceGroupID  sql.NullInt64
}
