package util

import (
	"fmt"

	"github.com/omec-project/openapi/models"
)

func SearchNFServiceUri(nfProfile models.NfProfile, serviceName models.ServiceName,
	nfServiceStatus models.NfServiceStatus,
) (nfUri string) {
	if nfProfile.NfServices != nil {
		for _, service := range *nfProfile.NfServices {
			if service.ServiceName == serviceName && service.NfServiceStatus == nfServiceStatus {
				if nfProfile.Fqdn != "" {
					nfUri = nfProfile.Fqdn
				} else if service.Fqdn != "" {
					nfUri = service.Fqdn
				} else if service.ApiPrefix != "" {
					nfUri = service.ApiPrefix
				} else if service.IpEndPoints != nil {
					point := (*service.IpEndPoints)[0]
					if point.Ipv4Address != "" {
						nfUri = getSbiUri(service.Scheme, point.Ipv4Address, point.Port)
					} else if len(nfProfile.Ipv4Addresses) != 0 {
						nfUri = getSbiUri(service.Scheme, nfProfile.Ipv4Addresses[0], point.Port)
					}
				}
			}
			if nfUri != "" {
				break
			}
		}
	}

	return
}

func getSbiUri(scheme models.UriScheme, ipv4Address string, port int32) (uri string) {
	return fmt.Sprintf("%s://%s:%d", scheme, ipv4Address, port)
}
