package models

type N3gaLocation struct {
	N3gppTai   *Tai
	UeIpv4Addr string
	UeIpv6Addr string
	PortNumber int32
}
