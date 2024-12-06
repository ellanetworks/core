package consumer

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/omec-project/nas/nasType"
	"github.com/omec-project/openapi/models"
	amf_context "github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/logger"
	"github.com/yeastengine/ella/internal/ausf/producer"
)

func SendUEAuthenticationAuthenticateRequest(ue *amf_context.AmfUe,
	resynchronizationInfo *models.ResynchronizationInfo,
) (*models.UeAuthenticationCtx, *models.ProblemDetails, error) {
	// configuration := Nausf_UEAuthentication.NewConfiguration()
	// configuration.SetBasePath(ue.AusfUri)

	guamiList := amf_context.GetServedGuamiList()
	servedGuami := guamiList[0]
	var plmnId *models.PlmnId
	// take ServingNetwork plmn from UserLocation.Tai if received
	if ue.Tai.PlmnId != nil {
		plmnId = ue.Tai.PlmnId
	} else {
		ue.GmmLog.Warnf("Tai is not received from Serving Network, Serving Plmn [Mcc: %v Mnc: %v] is taken from Guami List", servedGuami.PlmnId.Mcc, servedGuami.PlmnId.Mnc)
		plmnId = servedGuami.PlmnId
	}

	var authInfo models.AuthenticationInfo
	authInfo.SupiOrSuci = ue.Suci
	if mnc, err := strconv.Atoi(plmnId.Mnc); err != nil {
		return nil, nil, err
	} else {
		authInfo.ServingNetworkName = fmt.Sprintf("5G:mnc%03d.mcc%s.3gppnetwork.org", mnc, plmnId.Mcc)
	}
	if resynchronizationInfo != nil {
		authInfo.ResynchronizationInfo = resynchronizationInfo
	}
	// ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	// defer cancel()

	ueAuthenticationCtx, err := producer.UeAuthPostRequestProcedure(authInfo)
	if err != nil {
		logger.ConsumerLog.Errorf("UE Authentication Authenticate Request failed: %+v", err)
		return nil, nil, err
	}
	return ueAuthenticationCtx, nil, nil

	// client := Nausf_UEAuthentication.NewAPIClient(configuration)
	// ueAuthenticationCtx, httpResponse, err := client.DefaultApi.UeAuthenticationsPost(ctx, authInfo)
	// if err == nil {
	// 	return &ueAuthenticationCtx, nil, nil
	// } else if httpResponse != nil {
	// 	if httpResponse.Status != err.Error() {
	// 		return nil, nil, err
	// 	}
	// 	problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
	// 	return nil, &problem, nil
	// } else {
	// 	return nil, nil, openapi.ReportError("server no response")
	// }
}

func SendAuth5gAkaConfirmRequest(ue *amf_context.AmfUe, resStar string) (
	*models.ConfirmationDataResponse, *models.ProblemDetails, error,
) {
	logger.ConsumerLog.Warnf("SendAuth5gAkaConfirmRequest")
	confirmationData := models.ConfirmationData{
		ResStar: resStar,
	}
	confirmResult, err := producer.Auth5gAkaComfirmRequestProcedure(confirmationData, ue.Suci)
	if err != nil {
		logger.ConsumerLog.Errorf("Auth5gAkaComfirmRequestProcedure failed: %+v", err)
		problemDetails := &models.ProblemDetails{
			Status: 500,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		return nil, problemDetails, err
	}
	return confirmResult, nil, nil
}

func SendEapAuthConfirmRequest(ue *amf_context.AmfUe, eapMsg nasType.EAPMessage) (
	*models.EapSession, *models.ProblemDetails, error,
) {
	logger.ConsumerLog.Warnf("SendEapAuthConfirmRequest")

	eapSession := models.EapSession{
		EapPayload: base64.StdEncoding.EncodeToString(eapMsg.GetEAPMessage()),
	}

	response, err := producer.EapAuthComfirmRequestProcedure(eapSession, ue.Suci)
	if err != nil {
		logger.ConsumerLog.Errorf("EapAuthComfirmRequestProcedure failed: %+v", err)
		problemDetails := &models.ProblemDetails{
			Status: 500,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		return nil, problemDetails, err
	}
	return response, nil, nil
}
