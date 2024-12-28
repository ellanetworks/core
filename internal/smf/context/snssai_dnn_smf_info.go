package context

import (
	"net"

	"github.com/omec-project/openapi/models"
)

// SnssaiSmfInfo records the SMF S-NSSAI related information
type SnssaiSmfInfo struct {
	DnnInfos map[string]*SnssaiSmfDnnInfo
	PlmnId   models.PlmnId
	Snssai   SNssai
}

// SnssaiSmfDnnInfo records the SMF per S-NSSAI DNN information
type SnssaiSmfDnnInfo struct {
	DNS DNS
	MTU uint16
}

type DNS struct {
	IPv4Addr net.IP
	IPv6Addr net.IP
}
