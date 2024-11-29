package context

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/ausf/factory"
)

func InitAusfContext(context *AUSFContext) {
	config := factory.AusfConfig

	context.NfId = uuid.New().String()
	context.GroupID = config.GroupId
	context.UdmUri = config.UdmUri
	context.SBIPort = config.Sbi.Port
	context.UriScheme = models.UriScheme_HTTP
	context.BindingIPv4 = config.Sbi.BindingIPv4
	context.Url = fmt.Sprintf("%s://%s:%d", context.UriScheme, context.BindingIPv4, context.SBIPort)
	context.NfService = make(map[models.ServiceName]models.NfService)
	AddNfServices(&context.NfService, &config, context)
}

func AddNfServices(serviceMap *map[models.ServiceName]models.NfService, config *factory.Configuration, context *AUSFContext) {
	var nfService models.NfService
	var ipEndPoints []models.IpEndPoint
	var nfServiceVersions []models.NfServiceVersion
	services := *serviceMap

	// nausf-auth
	nfService.ServiceInstanceId = context.NfId
	nfService.ServiceName = models.ServiceName_NAUSF_AUTH

	var ipEndPoint models.IpEndPoint
	ipEndPoint.Ipv4Address = context.BindingIPv4
	ipEndPoint.Port = int32(context.SBIPort)
	ipEndPoints = append(ipEndPoints, ipEndPoint)

	var nfServiceVersion models.NfServiceVersion
	nfServiceVersions = append(nfServiceVersions, nfServiceVersion)

	nfService.Scheme = context.UriScheme
	nfService.NfServiceStatus = models.NfServiceStatus_REGISTERED

	nfService.IpEndPoints = &ipEndPoints
	nfService.Versions = &nfServiceVersions
	services[models.ServiceName_NAUSF_AUTH] = nfService
}
