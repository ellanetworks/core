package consumer

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/Nnrf_NFManagement"
	"github.com/omec-project/openapi/models"
	udm_context "github.com/yeastengine/canard/internal/udm/context"
	"github.com/yeastengine/canard/internal/udm/logger"
)

func BuildNFInstance(udmContext *udm_context.UDMContext) (profile models.NfProfile, err error) {
	profile.NfInstanceId = udmContext.NfId
	profile.NfStatus = models.NfStatus_REGISTERED
	profile.NfType = models.NfType_UDM
	services := []models.NfService{}
	for _, nfservice := range udmContext.NfService {
		services = append(services, nfservice)
	}
	if len(services) > 0 {
		profile.NfServices = &services
	}

	var plmns []models.PlmnId
	for _, plmnItem := range udmContext.PlmnList {
		plmns = append(plmns, plmnItem.PlmnId)
	}
	if len(plmns) > 0 {
		profile.PlmnList = &plmns
	}

	var udmInfo models.UdmInfo
	profile.UdmInfo = &udmInfo
	profile.UdmInfo.GroupId = udmContext.GroupId
	if udmContext.RegisterIPv4 == "" {
		err = fmt.Errorf("UDM Address is empty")
		return
	}
	profile.Ipv4Addresses = append(profile.Ipv4Addresses, udmContext.RegisterIPv4)

	return
}

var SendRegisterNFInstance = func(nrfUri, nfInstanceId string, profile models.NfProfile) (prof models.NfProfile, resouceNrfUri string,
	retrieveNfInstanceId string, err error,
) {
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(nrfUri)
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response
	for {
		prof, res, err = client.NFInstanceIDDocumentApi.RegisterNFInstance(context.TODO(), nfInstanceId, profile)
		if err != nil || res == nil {
			// TODO : add log
			fmt.Println(fmt.Errorf("UDM register to NRF Error[%v]", err.Error()))
			time.Sleep(2 * time.Second)
			continue
		}
		defer func() {
			if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
				logger.ConsumerLog.Errorf("GetIdentityData response body cannot close: %+v", rspCloseErr)
			}
		}()

		status := res.StatusCode
		if status == http.StatusOK {
			// NFUpdate
			break
		} else if status == http.StatusCreated {
			// NFRegister
			resourceUri := res.Header.Get("Location")
			resouceNrfUri = resourceUri[:strings.Index(resourceUri, "/nnrf-nfm/")]
			retrieveNfInstanceId = resourceUri[strings.LastIndex(resourceUri, "/")+1:]
			break
		} else {
			fmt.Println("handler returned wrong status code", status)
			fmt.Println("NRF return wrong status code", status)
		}
	}
	return prof, resouceNrfUri, retrieveNfInstanceId, err
}

func SendDeregisterNFInstance() (problemDetails *models.ProblemDetails, err error) {
	logger.ConsumerLog.Infof("Send Deregister NFInstance")

	udmSelf := udm_context.UDM_Self()
	// Set client and set url
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(udmSelf.NrfUri)
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response

	res, err = client.NFInstanceIDDocumentApi.DeregisterNFInstance(context.Background(), udmSelf.NfId)
	if err == nil {
		return
	} else if res != nil {
		defer func() {
			if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
				logger.ConsumerLog.Errorf("DeregisterNFInstance response body cannot close: %+v", rspCloseErr)
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
	logger.ConsumerLog.Debugf("Send Update NFInstance")

	udmSelf := udm_context.UDM_Self()
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(udmSelf.NrfUri)
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response
	nfProfile, res, err = client.NFInstanceIDDocumentApi.UpdateNFInstance(context.Background(), udmSelf.NfId, patchItem)
	if err == nil {
		return
	} else if res != nil {
		defer func() {
			if resCloseErr := res.Body.Close(); resCloseErr != nil {
				logger.ConsumerLog.Errorf("UpdateNFInstance response cannot close: %+v", resCloseErr)
			}
		}()
		if res.Status != err.Error() {
			logger.ConsumerLog.Errorf("UpdateNFInstance received error response: %v", res.Status)
			return
		}
		problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("server no response")
	}
	return
}
