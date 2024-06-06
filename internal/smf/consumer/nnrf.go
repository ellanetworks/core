package consumer

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/antihax/optional"
	"github.com/mohae/deepcopy"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/Nnrf_NFDiscovery"
	"github.com/omec-project/openapi/Nnrf_NFManagement"
	"github.com/omec-project/openapi/Nudm_SubscriberDataManagement"
	"github.com/omec-project/openapi/models"
	nrf_cache "github.com/yeastengine/ella/internal/nrf/nrfcache"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
)

func SendNFRegistration() (*models.NfProfile, error) {
	var rep models.NfProfile
	sNssais := []models.Snssai{}

	if len(*smf_context.SmfInfo.SNssaiSmfInfoList) == 0 {
		logger.ConsumerLog.Errorf("slice info not available, dropping NRF registration")
		return &rep, fmt.Errorf("slice info nil")
	}

	for _, snssaiSmfInfo := range *smf_context.SmfInfo.SNssaiSmfInfoList {
		sNssais = append(sNssais, *snssaiSmfInfo.SNssai)
	}

	// set nfProfile
	profile := models.NfProfile{
		NfInstanceId:  smf_context.SMF_Self().NfInstanceID,
		NfType:        models.NfType_SMF,
		NfStatus:      models.NfStatus_REGISTERED,
		Ipv4Addresses: []string{smf_context.SMF_Self().RegisterIPv4},
		NfServices:    smf_context.NFServices,
		SmfInfo:       smf_context.SmfInfo,
		SNssais:       &sNssais,
		PlmnList:      smf_context.SmfPlmnConfig(),
		AllowedPlmns:  smf_context.SmfPlmnConfig(),
	}

	var res *http.Response
	var err error

	// Check data (Use RESTful PUT)

	rep, res, err = smf_context.SMF_Self().
		NFManagementClient.
		NFInstanceIDDocumentApi.
		RegisterNFInstance(context.TODO(), smf_context.SMF_Self().NfInstanceID, profile)

	if err != nil || res == nil {
		logger.ConsumerLog.Infof("SMF register to NRF Error[%s]", err.Error())
		return &rep, fmt.Errorf("NRF Registration failure")
	}

	if res != nil {
		defer func() {
			if resCloseErr := res.Body.Close(); resCloseErr != nil {
				logger.ConsumerLog.Errorf("RegisterNFInstance response body cannot close: %+v", resCloseErr)
			}
		}()
	}

	status := res.StatusCode
	if status == http.StatusOK {
		// NFUpdate
		logger.ConsumerLog.Infof("NRF Registration success, status [%v]", http.StatusText(res.StatusCode))
	} else if status == http.StatusCreated {
		// NFRegister
		resourceUri := res.Header.Get("Location")
		// resouceNrfUri := resourceUri[strings.LastIndex(resourceUri, "/"):]
		smf_context.SMF_Self().NfInstanceID = resourceUri[strings.LastIndex(resourceUri, "/")+1:]
		logger.ConsumerLog.Infof("NRF Registration success, status [%v]", http.StatusText(res.StatusCode))
	} else {
		logger.ConsumerLog.Infof("handler returned wrong status code %d", status)
		// fmt.Errorf("NRF return wrong status code %d", status)
		logger.ConsumerLog.Errorf("NRF Registration failure, status [%v]", http.StatusText(res.StatusCode))
		return &rep, fmt.Errorf("NRF Registration failure, [%v]", http.StatusText(res.StatusCode))
	}

	logger.InitLog.Infof("SMF Registration to NRF %v", rep)
	return &rep, nil
}

func ReSendNFRegistration() (profile *models.NfProfile) {
	for {
		var err error
		if profile, err = SendNFRegistration(); err != nil {
			logger.ConsumerLog.Warnf("Send NFRegistration Failed, %v", err)
			time.Sleep(time.Second * 2)
			continue
		}
		return profile
	}
}

var SendUpdateNFInstance = func(patchItem []models.PatchItem) (nfProfile *models.NfProfile, problemDetails *models.ProblemDetails, err error) {
	logger.ConsumerLog.Debugf("Send Update NFInstance")

	smfSelf := smf_context.SMF_Self()
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(smfSelf.NrfUri)
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response
	var nf models.NfProfile
	nf, res, err = client.NFInstanceIDDocumentApi.UpdateNFInstance(context.Background(), smfSelf.NfInstanceID, patchItem)
	if err == nil {
		return &nf, nil, nil
	} else if res != nil {
		defer func() {
			if resCloseErr := res.Body.Close(); resCloseErr != nil {
				logger.ConsumerLog.Errorf("UpdateNFInstance response cannot close: %+v", resCloseErr)
			}
		}()
		if res.Status != err.Error() {
			logger.ConsumerLog.Errorf("UpdateNFInstance received error response: %v", res.Status)
			return &nf, problemDetails, err
		}
		problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("server no response")
	}
	return &nf, problemDetails, err
}

func SendNFDeregistration() error {
	// Check data (Use RESTful DELETE)
	res, localErr := smf_context.SMF_Self().
		NFManagementClient.
		NFInstanceIDDocumentApi.
		DeregisterNFInstance(context.TODO(), smf_context.SMF_Self().NfInstanceID)
	if localErr != nil {
		logger.ConsumerLog.Warnln(localErr)
		return localErr
	}
	defer func() {
		if resCloseErr := res.Body.Close(); resCloseErr != nil {
			logger.ConsumerLog.Errorf("DeregisterNFInstance response body cannot close: %+v", resCloseErr)
		}
	}()
	if res != nil {
		if status := res.StatusCode; status != http.StatusNoContent {
			logger.ConsumerLog.Warnln("handler returned wrong status code ", status)
			return openapi.ReportError("handler returned wrong status code %d", status)
		}
	}
	return nil
}

func SendNrfForNfInstance(nrfUri string, targetNfType, requestNfType models.NfType,
	param *Nnrf_NFDiscovery.SearchNFInstancesParamOpts,
) (models.SearchResult, error) {
	result, httpResp, localErr := smf_context.SMF_Self().
		NFDiscoveryClient.
		NFInstancesStoreApi.
		SearchNFInstances(context.TODO(), targetNfType, requestNfType, param)

	if localErr == nil {
		if result.NfInstances == nil {
			if status := httpResp.StatusCode; status != http.StatusOK {
				logger.ConsumerLog.Warnln("handler returned wrong status code", status)
			}

			logger.ConsumerLog.Warnln("NfInstances is nil")
			return result, openapi.ReportError("NfInstances is nil")
		}
	} else if httpResp != nil {
		defer func() {
			if resCloseErr := httpResp.Body.Close(); resCloseErr != nil {
				logger.ConsumerLog.Errorf("SearchNFInstances response body cannot close: %+v", resCloseErr)
			}
		}()

		logger.ConsumerLog.Warnln("handler returned wrong status code ", httpResp.Status)
		if httpResp.Status != localErr.Error() {
		} else {
		}
	} else {
		localErr = openapi.ReportError("server no response")
	}

	smfSelf := smf_context.SMF_Self()

	for _, nfProfile := range result.NfInstances {
		if _, ok := smfSelf.NfStatusSubscriptions.Load(nfProfile.NfInstanceId); !ok {
			nrfSubscriptionData := models.NrfSubscriptionData{
				NfStatusNotificationUri: fmt.Sprintf("%s://%s:%d/nsmf-callback/nf-status-notify",
					smfSelf.URIScheme,
					smfSelf.RegisterIPv4,
					smfSelf.SBIPort),
				SubscrCond: &models.NfInstanceIdCond{NfInstanceId: nfProfile.NfInstanceId},
				ReqNfType:  requestNfType,
			}
			nrfSubData, problemDetails, err := SendCreateSubscription(nrfUri, nrfSubscriptionData, targetNfType)
			if problemDetails != nil {
				logger.ConsumerLog.Errorf("SendCreateSubscription to NRF, Problem[%+v]", problemDetails)
			} else if err != nil {
				logger.ConsumerLog.Errorf("SendCreateSubscription Error[%+v]", err)
			}
			smfSelf.NfStatusSubscriptions.Store(nfProfile.NfInstanceId, nrfSubData.SubscriptionId)
		}
	}
	return result, localErr
}

func SendNFDiscoveryUDM() (*models.ProblemDetails, error) {
	localVarOptionals := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{}

	var result models.SearchResult
	var localErr error

	if smf_context.SMF_Self().EnableNrfCaching {
		result, localErr = nrf_cache.SearchNFInstances(smf_context.SMF_Self().NrfUri, models.NfType_UDM, models.NfType_SMF, &localVarOptionals)
	} else {
		result, localErr = SendNrfForNfInstance(smf_context.SMF_Self().NrfUri, models.NfType_UDM, models.NfType_SMF, &localVarOptionals)
	}

	if localErr == nil {
		smf_context.SMF_Self().UDMProfile = result.NfInstances[0]

		for _, service := range *smf_context.SMF_Self().UDMProfile.NfServices {
			if service.ServiceName == models.ServiceName_NUDM_SDM {
				SDMConf := Nudm_SubscriberDataManagement.NewConfiguration()
				SDMConf.SetBasePath(service.ApiPrefix)
				smf_context.SMF_Self().SubscriberDataManagementClient = Nudm_SubscriberDataManagement.NewAPIClient(SDMConf)
			}
		}

		if smf_context.SMF_Self().SubscriberDataManagementClient == nil {
			logger.ConsumerLog.Warnln("sdm client failed")
		}
	} else {
		apiError, ok := localErr.(openapi.GenericOpenAPIError)
		if ok {
			problem := apiError.Model().(models.ProblemDetails)
			return &problem, nil
		}

		return nil, localErr
	}
	return nil, nil
}

func SendNFDiscoveryPCF() (problemDetails *models.ProblemDetails, err error) {
	localVarOptionals := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{}

	var result models.SearchResult
	var localErr error

	if smf_context.SMF_Self().EnableNrfCaching {
		result, localErr = nrf_cache.SearchNFInstances(smf_context.SMF_Self().NrfUri, models.NfType_PCF, models.NfType_SMF, &localVarOptionals)
	} else {
		result, localErr = SendNrfForNfInstance(smf_context.SMF_Self().NrfUri, models.NfType_PCF, models.NfType_SMF, &localVarOptionals)
	}

	if localErr == nil {
		logger.ConsumerLog.Traceln(result.NfInstances)
	} else {
		apiError, ok := localErr.(openapi.GenericOpenAPIError)
		if ok {
			problem := apiError.Model().(models.ProblemDetails)
			return &problem, nil
		}

		return nil, localErr
	}

	return problemDetails, err
}

func SendNFDiscoveryServingAMF(smContext *smf_context.SMContext) (*models.ProblemDetails, error) {
	localVarOptionals := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{}

	localVarOptionals.TargetNfInstanceId = optional.NewInterface(smContext.ServingNfId)

	var result models.SearchResult
	var localErr error

	if smf_context.SMF_Self().EnableNrfCaching {
		result, localErr = nrf_cache.SearchNFInstances(smf_context.SMF_Self().NrfUri, models.NfType_AMF, models.NfType_SMF, &localVarOptionals)
	} else {
		result, localErr = SendNrfForNfInstance(smf_context.SMF_Self().NrfUri, models.NfType_AMF, models.NfType_SMF, &localVarOptionals)
	}

	if localErr == nil {
		if result.NfInstances == nil {
			return nil, openapi.ReportError("NfInstances is nil")
		}
		smContext.SubConsumerLog.Info("send NF Discovery Serving AMF Successful")
		smContext.AMFProfile = deepcopy.Copy(result.NfInstances[0]).(models.NfProfile)
	} else {
		apiError, ok := localErr.(openapi.GenericOpenAPIError)
		if ok {
			problem := apiError.Model().(models.ProblemDetails)
			return &problem, nil
		}

		return nil, localErr
	}
	return nil, nil
}

func SendDeregisterNFInstance() (*models.ProblemDetails, error) {
	logger.ConsumerLog.Infof("Send Deregister NFInstance")

	smfSelf := smf_context.SMF_Self()
	// Set client and set url

	res, err := smfSelf.
		NFManagementClient.
		NFInstanceIDDocumentApi.
		DeregisterNFInstance(context.Background(), smfSelf.NfInstanceID)
	if err == nil {
		return nil, err
	} else if res != nil {
		defer func() {
			if resCloseErr := res.Body.Close(); resCloseErr != nil {
				logger.ConsumerLog.Errorf("DeregisterNFInstance response body cannot close: %+v", resCloseErr)
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

func SendCreateSubscription(nrfUri string, nrfSubscriptionData models.NrfSubscriptionData, targetNfType models.NfType) (nrfSubData models.NrfSubscriptionData, problemDetails *models.ProblemDetails, err error) {
	logger.ConsumerLog.Debugf("Send Create Subscription for %v", targetNfType)

	var res *http.Response
	nrfSubData, res, err = smf_context.SMF_Self().NFManagementClient.SubscriptionsCollectionApi.CreateSubscription(context.TODO(), nrfSubscriptionData)
	if err == nil {
		return
	} else if res != nil {
		defer func() {
			if resCloseErr := res.Body.Close(); resCloseErr != nil {
				logger.ConsumerLog.Errorf("SendCreateSubscription response cannot close: %+v", resCloseErr)
			}
		}()
		if res.Status != err.Error() {
			logger.ConsumerLog.Errorf("SendCreateSubscription received error response: %v", res.Status)
			return
		}
		problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("server no response")
	}
	return
}

func SendRemoveSubscriptionProcedure(notificationData models.NotificationData) {
	logger.ConsumerLog.Infof("[SMF] Send Remove Subscription Procedure")
	nfInstanceId := notificationData.NfInstanceUri[strings.LastIndex(notificationData.NfInstanceUri, "/")+1:]

	if subscriptionId, ok := smf_context.SMF_Self().NfStatusSubscriptions.Load(nfInstanceId); ok {
		logger.ConsumerLog.Debugf("SubscriptionId of nfInstance %v is %v", nfInstanceId, subscriptionId.(string))
		problemDetails, err := SendRemoveSubscription(subscriptionId.(string))
		if problemDetails != nil {
			logger.ConsumerLog.Errorf("Remove NF Subscription Failed Problem[%+v]", problemDetails)
		} else if err != nil {
			logger.ConsumerLog.Errorf("Remove NF Subscription Error[%+v]", err)
		} else {
			logger.ConsumerLog.Infoln("[SMF] Remove NF Subscription successful")
			smf_context.SMF_Self().NfStatusSubscriptions.Delete(nfInstanceId)
		}
	} else {
		logger.ConsumerLog.Infof("nfinstance %v not found in map", nfInstanceId)
	}
}

func SendRemoveSubscription(subscriptionId string) (problemDetails *models.ProblemDetails, err error) {
	logger.ConsumerLog.Infof("[SMF] Send Remove Subscription for Subscription Id: %v", subscriptionId)

	var res *http.Response
	res, err = smf_context.SMF_Self().NFManagementClient.SubscriptionIDDocumentApi.RemoveSubscription(context.Background(), subscriptionId)
	if err == nil {
		return
	} else if res != nil {
		defer func() {
			if bodyCloseErr := res.Body.Close(); bodyCloseErr != nil {
				err = fmt.Errorf("RemoveSubscription' response body cannot close: %+w", bodyCloseErr)
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
