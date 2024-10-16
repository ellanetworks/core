package context

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/nssf/factory"
	"github.com/yeastengine/ella/internal/nssf/logger"
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
	nssfContext.NfService = initNfService(serviceName, "1.0.0")
}

type NSSFContext struct {
	NfId        string
	Name        string
	UriScheme   models.UriScheme
	BindingIPv4 string
	NfService   map[models.ServiceName]models.NfService
	// SupportedPlmnList []models.PlmnId
	SBIPort int
}

// Initialize NSSF context with configuration factory
func InitNssfContext() {
	if !factory.Configured {
		logger.ContextLog.Warnf("NSSF is not configured")
		return
	}
	nssfConfig := factory.NssfConfig
	nssfContext.Name = nssfConfig.Configuration.NssfName
	nssfContext.UriScheme = models.UriScheme_HTTP
	nssfContext.SBIPort = nssfConfig.Configuration.Sbi.Port
	nssfContext.BindingIPv4 = nssfConfig.Configuration.Sbi.BindingIPv4
	nssfContext.NfService = initNfService(nssfConfig.Configuration.ServiceNameList, nssfConfig.Info.Version)
}

func initNfService(serviceName []models.ServiceName, version string) (
	nfService map[models.ServiceName]models.NfService,
) {
	versionUri := "v" + strings.Split(version, ".")[0]
	nfService = make(map[models.ServiceName]models.NfService)
	for idx, name := range serviceName {
		nfService[name] = models.NfService{
			ServiceInstanceId: strconv.Itoa(idx),
			ServiceName:       name,
			Versions: &[]models.NfServiceVersion{
				{
					ApiFullVersion:  version,
					ApiVersionInUri: versionUri,
				},
			},
			Scheme:          nssfContext.UriScheme,
			NfServiceStatus: models.NfServiceStatus_REGISTERED,
			ApiPrefix:       GetIpv4Uri(),
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
