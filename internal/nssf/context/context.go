package context

import (
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/nssf/factory"
)

var nssfContext = NSSFContext{}

func init() {
	nssfContext.NfId = uuid.New().String()
	nssfContext.Name = "NSSF"
	nssfContext.UriScheme = models.UriScheme_HTTP
	serviceName := []models.ServiceName{
		models.ServiceName_NNSSF_NSSELECTION,
		models.ServiceName_NNSSF_NSSAIAVAILABILITY,
	}
	nssfContext.NfService = initNfService(serviceName)
}

type NSSFContext struct {
	NfId        string
	Name        string
	UriScheme   models.UriScheme
	BindingIPv4 string
	NfService   map[models.ServiceName]models.NfService
	SBIPort     int
}

// Initialize NSSF context with configuration factory
func InitNssfContext() {
	nssfConfig := factory.NssfConfig
	nssfContext.Name = nssfConfig.NssfName
	nssfContext.UriScheme = models.UriScheme_HTTP
	nssfContext.SBIPort = nssfConfig.Sbi.Port
	nssfContext.BindingIPv4 = nssfConfig.Sbi.BindingIPv4
	nssfContext.NfService = initNfService(nssfConfig.ServiceNameList)
}

func initNfService(serviceName []models.ServiceName) (
	nfService map[models.ServiceName]models.NfService,
) {
	nfService = make(map[models.ServiceName]models.NfService)
	for idx, name := range serviceName {
		nfService[name] = models.NfService{
			ServiceInstanceId: strconv.Itoa(idx),
			ServiceName:       name,
			Scheme:            nssfContext.UriScheme,
			NfServiceStatus:   models.NfServiceStatus_REGISTERED,
			ApiPrefix:         GetIpv4Uri(),
			IpEndPoints: &[]models.IpEndPoint{
				{
					Ipv4Address: nssfContext.BindingIPv4,
					Transport:   models.TransportProtocol_TCP,
					Port:        int32(nssfContext.SBIPort),
				},
			},
		}
	}

	return
}

func GetIpv4Uri() string {
	return fmt.Sprintf("%s://%s:%d", nssfContext.UriScheme, nssfContext.BindingIPv4, nssfContext.SBIPort)
}

func NSSF_Self() *NSSFContext {
	return &nssfContext
}
