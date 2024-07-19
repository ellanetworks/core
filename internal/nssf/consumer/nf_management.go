/*
 * NSSF Consumer
 *
 * Network Function Management
 */

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
	nssf_context "github.com/yeastengine/ella/internal/nssf/context"
	"github.com/yeastengine/ella/internal/nssf/logger"
)

func BuildNFProfile(context *nssf_context.NSSFContext) (profile models.NfProfile, err error) {
	profile.NfInstanceId = context.NfId
	profile.NfType = models.NfType_NSSF
	profile.NfStatus = models.NfStatus_REGISTERED
	profile.PlmnList = &context.SupportedPlmnList
	profile.Ipv4Addresses = []string{context.RegisterIPv4}
	var services []models.NfService
	for _, nfService := range context.NfService {
		services = append(services, nfService)
	}
	if len(services) > 0 {
		profile.NfServices = &services
	}
	return profile, err
}

var SendRegisterNFInstance = func(nrfUri, nfInstanceId string, profile models.NfProfile) (
	prof models.NfProfile, resourceNrfUri string, retrieveNfInstanceId string, err error,
) {
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(nrfUri)
	apiClient := Nnrf_NFManagement.NewAPIClient(configuration)
	var waitTime time.Duration = 2

	var res *http.Response
	for {
		prof, res, err = apiClient.NFInstanceIDDocumentApi.RegisterNFInstance(context.TODO(), nfInstanceId, profile)
		if err != nil || res == nil {
			// TODO : add log
			logger.ConsumerLog.Errorf("NSSF register to NRF Error[%s]", err.Error())
			time.Sleep(waitTime * time.Second)
			continue
		}
		defer func() {
			if resCloseErr := res.Body.Close(); resCloseErr != nil {
				logger.ConsumerLog.Errorf("NFInstanceIDDocumentApi response body cannot close: %+v", resCloseErr)
			}
		}()
		status := res.StatusCode
		if status == http.StatusOK {
			// NFUpdate
			break
		} else if status == http.StatusCreated {
			// NFRegister
			resourceUri := res.Header.Get("Location")
			resourceNrfUri = resourceUri[:strings.Index(resourceUri, "/nnrf-nfm/")]
			retrieveNfInstanceId = resourceUri[strings.LastIndex(resourceUri, "/")+1:]
			break
		} else {
			fmt.Println("NRF return wrong status code", status)
		}
	}
	return prof, resourceNrfUri, retrieveNfInstanceId, err
}

var SendUpdateNFInstance = func(patchItem []models.PatchItem) (nfProfile models.NfProfile, problemDetails *models.ProblemDetails, err error) {
	logger.ConsumerLog.Debugf("Send Update NFInstance")

	nssfSelf := nssf_context.NSSF_Self()
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(nssfSelf.NrfUri)
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response
	nfProfile, res, err = client.NFInstanceIDDocumentApi.UpdateNFInstance(context.Background(), nssfSelf.NfId, patchItem)
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

func SendDeregisterNFInstance() (*models.ProblemDetails, error) {
	logger.AppLog.Infof("Send Deregister NFInstance")

	nssfSelf := nssf_context.NSSF_Self()
	// Set client and set url
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(nssfSelf.NrfUri)
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response
	var err error

	res, err = client.NFInstanceIDDocumentApi.DeregisterNFInstance(context.Background(), nssfSelf.NfId)
	if err == nil {
		return nil, err
	} else if res != nil {
		defer func() {
			if resCloseErr := res.Body.Close(); resCloseErr != nil {
				logger.ConsumerLog.Errorf("NFInstanceIDDocumentApi response body cannot close: %+v", resCloseErr)
			}
		}()
		if res.Status != err.Error() {
			return nil, err
		}
		problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		return &problem, err
	} else {
		return nil, openapi.ReportError("server no response")
	}
}
