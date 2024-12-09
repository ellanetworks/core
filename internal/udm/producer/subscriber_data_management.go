package producer

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/udm/context"
	"github.com/yeastengine/ella/internal/udr/producer"
)

func GetAmData(supi string) (
	*models.AccessAndMobilitySubscriptionData, error,
) {
	amData, err := producer.GetAmData(supi)
	if err != nil {
		logger.UdmLog.Errorf("GetAmData error: %+v", err)
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	udmUe := context.UDM_Self().NewUdmUe(supi)
	udmUe.SetAMSubsriptionData(amData)
	return amData, nil
}

func GetSmData(supi string, Dnn string, Snssai string) ([]models.SessionManagementSubscriptionData, error) {
	sessionManagementSubscriptionDataResp, err := producer.GetSmData(supi)
	if err != nil {
		return nil, fmt.Errorf("GetSmData error: %+v", err)
	}

	udmUe := context.UDM_Self().NewUdmUe(supi)
	smData, _, _, _ := context.UDM_Self().ManageSmData(
		sessionManagementSubscriptionDataResp, Snssai, Dnn)
	udmUe.SetSMSubsData(smData)

	rspSMSubDataList := make([]models.SessionManagementSubscriptionData, 0, 4)

	udmUe.SmSubsDataLock.RLock()
	for _, eachSMSubData := range udmUe.SessionManagementSubsData {
		rspSMSubDataList = append(rspSMSubDataList, eachSMSubData)
	}
	udmUe.SmSubsDataLock.RUnlock()

	return rspSMSubDataList, nil
}

func GetNssai(supi string) (*models.Nssai, error) {
	accessAndMobilitySubscriptionDataResp, err := producer.GetAmData(supi)
	if err != nil {
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	nssaiResp := *accessAndMobilitySubscriptionDataResp.Nssai
	udmUe := context.UDM_Self().NewUdmUe(supi)
	udmUe.Nssai = &nssaiResp
	return udmUe.Nssai, nil
}

func GetSmfSelectData(supi string) (
	*models.SmfSelectionSubscriptionData, error,
) {
	var body models.SmfSelectionSubscriptionData
	context.UDM_Self().CreateSmfSelectionSubsDataforUe(supi, body)
	smfSelectionSubscriptionDataResp, err := producer.GetSmfSelectData(supi)
	if err != nil {
		logger.UdmLog.Errorf("GetSmfSelectData error: %+v", err)
		return nil, fmt.Errorf("GetSmfSelectData error: %+v", err)
	}
	udmUe := context.UDM_Self().NewUdmUe(supi)
	udmUe.SetSmfSelectionSubsData(smfSelectionSubscriptionDataResp)
	return udmUe.SmfSelSubsData, nil
}

func CreateSubscription(sdmSubscription *models.SdmSubscription, supi string) error {
	sdmSubscriptionResp := producer.CreateSdmSubscriptions(*sdmSubscription, supi)
	header := make(http.Header)
	udmUe, _ := context.UDM_Self().UdmUeFindBySupi(supi)
	if udmUe == nil {
		udmUe = context.UDM_Self().NewUdmUe(supi)
	}
	udmUe.CreateSubscriptiontoNotifChange(sdmSubscriptionResp.SubscriptionId, &sdmSubscriptionResp)
	header.Set("Location", udmUe.GetLocationURI2(context.LocationUriSdmSubscription, supi))
	return nil
}

func GetUeContextInSmfData(supi string) (*models.UeContextInSmfData, error) {
	var body models.UeContextInSmfData
	context.UDM_Self().CreateUeContextInSmfDataforUe(supi, body)
	pdusess, err := producer.GetSMFRegistrations(supi)
	if err != nil {
		return nil, fmt.Errorf("GetSMFRegistrations error: %+v", err)
	}
	pduSessionMap := make(map[string]models.PduSession)
	for _, element := range pdusess {
		var pduSession models.PduSession
		pduSession.Dnn = element.Dnn
		pduSession.SmfInstanceId = element.SmfInstanceId
		pduSession.PlmnId = element.PlmnId
		pduSessionMap[strconv.Itoa(int(element.PduSessionId))] = pduSession
	}
	var ueContextInSmfData models.UeContextInSmfData
	ueContextInSmfData.PduSessions = pduSessionMap
	var pgwInfoArray []models.PgwInfo
	for _, element := range pdusess {
		var pgwInfo models.PgwInfo
		pgwInfo.Dnn = element.Dnn
		pgwInfo.PgwFqdn = element.PgwFqdn
		pgwInfo.PlmnId = element.PlmnId
		pgwInfoArray = append(pgwInfoArray, pgwInfo)
	}

	ueContextInSmfData.PgwInfo = pgwInfoArray

	udmUe := context.UDM_Self().NewUdmUe(supi)
	udmUe.UeCtxtInSmfData = &ueContextInSmfData
	return udmUe.UeCtxtInSmfData, nil
}
