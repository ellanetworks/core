package consumer

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/Nnrf_NFManagement"
	"github.com/omec-project/openapi/models"
	pcf_context "github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/logger"
)

func BuildNFInstance(context *pcf_context.PCFContext) (profile models.NfProfile, err error) {
	profile.NfInstanceId = context.NfId
	profile.NfType = models.NfType_PCF
	profile.NfStatus = models.NfStatus_REGISTERED
	profile.Ipv4Addresses = append(profile.Ipv4Addresses, context.RegisterIPv4)
	service := []models.NfService{}
	for _, nfService := range context.NfService {
		service = append(service, nfService)
	}
	profile.NfServices = &service

	var plmns []models.PlmnId
	for _, plmnItem := range context.PlmnList {
		plmns = append(plmns, plmnItem.PlmnId)
	}
	if len(plmns) > 0 {
		profile.PlmnList = &plmns
	}

	profile.PcfInfo = &models.PcfInfo{
		DnnList: context.DnnList,
	}
	return profile, err
}

var SendRegisterNFInstance = func(nrfUri, nfInstanceId string, profile models.NfProfile) (
	nfProfile models.NfProfile, resouceNrfUri string, retrieveNfInstanceID string, err error,
) {
	// Set client and set url
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(nrfUri)
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response
	for {
		nfProfile, res, err = client.NFInstanceIDDocumentApi.RegisterNFInstance(context.TODO(), nfInstanceId, profile)
		if err != nil || res == nil {
			// TODO : add log
			logger.Consumerlog.Infof("PCF register to NRF Error[%v]", err.Error())
			time.Sleep(2 * time.Second)
			continue
		}
		defer func() {
			if resCloseErr := res.Body.Close(); resCloseErr != nil {
				logger.Consumerlog.Errorf("RegisterNFInstance response body cannot close: %+v", resCloseErr)
			}
		}()
		status := res.StatusCode
		if status == http.StatusOK {
			// NFUpdate
			logger.Consumerlog.Infoln("PCF register to NRF - updated Success")
			break
		} else if status == http.StatusCreated {
			// NFRegister
			logger.Consumerlog.Infoln("PCF register to NRF - created Success")
			resourceUri := res.Header.Get("Location")
			resouceNrfUri = resourceUri[:strings.Index(resourceUri, "/nnrf-nfm/")]
			retrieveNfInstanceID = resourceUri[strings.LastIndex(resourceUri, "/")+1:]
			break
		} else {
			logger.Consumerlog.Errorf("NRF return wrong status code: %+v", status)
		}
	}
	return nfProfile, resouceNrfUri, retrieveNfInstanceID, err
}

func SendDeregisterNFInstance() (problemDetails *models.ProblemDetails, err error) {
	logger.Consumerlog.Debugf("Send Deregister NFInstance")

	pcfSelf := pcf_context.PCF_Self()
	// Set client and set url
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(pcfSelf.NrfUri)
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response

	res, err = client.NFInstanceIDDocumentApi.DeregisterNFInstance(context.Background(), pcfSelf.NfId)
	if err == nil {
		return
	} else if res != nil {
		defer func() {
			if resCloseErr := res.Body.Close(); resCloseErr != nil {
				logger.Consumerlog.Errorf("DeregisterNFInstance response cannot close: %+v", resCloseErr)
			}
		}()
		if res.Status != err.Error() {
			return
		}
		problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("server no response")
	}
	return
}

var SendUpdateNFInstance = func(patchItem []models.PatchItem) (nfProfile models.NfProfile, problemDetails *models.ProblemDetails, err error) {
	logger.Consumerlog.Debugf("Send Update NFInstance")

	pcfSelf := pcf_context.PCF_Self()
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(pcfSelf.NrfUri)
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response
	nfProfile, res, err = client.NFInstanceIDDocumentApi.UpdateNFInstance(context.Background(), pcfSelf.NfId, patchItem)
	if err == nil {
		return
	} else if res != nil {
		defer func() {
			if resCloseErr := res.Body.Close(); resCloseErr != nil {
				logger.Consumerlog.Errorf("UpdateNFInstance response cannot close: %+v", resCloseErr)
			}
		}()
		if res.Status != err.Error() {
			logger.Consumerlog.Errorf("UpdateNFInstance received error response: %v", res.Status)
			return
		}
		problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("server no response")
	}
	return
}
