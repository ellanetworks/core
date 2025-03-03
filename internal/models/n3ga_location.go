package models

type N3gaLocation struct {
	N3gppTai   *Tai
	N3IwfId    string
	UeIpv4Addr string
	UeIpv6Addr string
	PortNumber int32
}
