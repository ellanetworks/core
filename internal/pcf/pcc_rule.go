package pcf

import (
	"fmt"
	"time"

	"github.com/omec-project/openapi/models"
)

var MediaTypeTo5qiMap = map[models.MediaType]int32{
	models.MediaType_AUDIO:       1,
	models.MediaType_VIDEO:       2,
	models.MediaType_APPLICATION: 2,
	models.MediaType_DATA:        9,
	models.MediaType_CONTROL:     9,
	models.MediaType_TEXT:        9,
	models.MediaType_MESSAGE:     9,
	models.MediaType_OTHER:       9,
}

// Get pcc rule Identity(PccRuleId-%d)
func GetPccRuleId(id int32) string {
	return fmt.Sprintf("PccRuleId-%d", id)
}

// Get qos Identity(QosId-%d)
func GetQosId(id int32) string {
	return fmt.Sprintf("QosId-%d", id)
}

// Get Cond Identity(CondId-%d)
func GetCondId(id int32) string {
	return fmt.Sprintf("CondId-%d", id)
}

// Get Traffic Control Identity(TcId-%d)
func GetTcId(id int32) string {
	return fmt.Sprintf("TcId-%d", id)
}

// Get Charging Identity(ChgId-%d)
func GetChgId(id int32) string {
	return fmt.Sprintf("ChgId-%d", id)
}

// Get Charging Identity(ChgId-%d)
func GetUmId(sponId, aspId string) string {
	return fmt.Sprintf("umId-%s-%s", sponId, aspId)
}

// Get Packet Filter Identity(PackFiltId-%d)
func GetPackFiltId(id int32) string {
	return fmt.Sprintf("PackFiltId-%d", id)
}

// Create Pcc Rule with param id, precedence, flow information, appID
func CreatePccRule(id, precedence int32, flowInfo []models.FlowInformation, appID string) *models.PccRule {
	rule := models.PccRule{
		AppId:      appID,
		FlowInfos:  flowInfo,
		PccRuleId:  GetPccRuleId(id),
		Precedence: precedence,
	}
	return &rule
}

func CreateCondData(id int32) models.ConditionData {
	activationTime := time.Now()
	return models.ConditionData{
		CondId:         GetCondId(id),
		ActivationTime: &activationTime,
	}
}

func CreateQosData(id, var5qi, arp int32) models.QosData {
	return models.QosData{
		QosId:  GetQosId(id),
		Var5qi: var5qi,
		Arp: &models.Arp{
			PriorityLevel: arp,
		},
	}
}

func CreateTcData(id int32, fullID string, flowStatus models.FlowStatus) *models.TrafficControlData {
	if flowStatus == "" {
		flowStatus = models.FlowStatus_ENABLED
	}
	if fullID == "" {
		fullID = GetTcId(id)
	}
	return &models.TrafficControlData{
		TcId:       fullID,
		FlowStatus: flowStatus,
	}
}

func CreateUmData(umId string, thresh models.UsageThreshold) models.UsageMonitoringData {
	return models.UsageMonitoringData{
		UmId:                    umId,
		VolumeThreshold:         thresh.TotalVolume,
		VolumeThresholdUplink:   thresh.UplinkVolume,
		VolumeThresholdDownlink: thresh.DownlinkVolume,
		TimeThreshold:           thresh.Duration,
	}
}

// Convert Packet Filter information list to Flow Information List(Packet Filter Usage always true),
// EthDescription is Not Supported
func ConvertPacketInfoToFlowInformation(infos []models.PacketFilterInfo) (flowInfos []models.FlowInformation) {
	for _, info := range infos {
		flowInfo := models.FlowInformation{
			FlowDescription:   info.PackFiltCont,
			PackFiltId:        info.PackFiltId,
			PacketFilterUsage: true,
			TosTrafficClass:   info.TosTrafficClass,
			Spi:               info.Spi,
			FlowLabel:         info.FlowLabel,
			FlowDirection:     models.FlowDirectionRm(info.FlowDirection),
		}
		flowInfos = append(flowInfos, flowInfo)
	}
	return
}

func GetPccRuleByAfAppId(pccRules map[string]*models.PccRule, afAppId string) *models.PccRule {
	for _, pccRule := range pccRules {
		if pccRule.AppId == afAppId {
			return pccRule
		}
	}
	return nil
}

func GetPccRuleByFlowInfos(pccRules map[string]*models.PccRule, flowInfos []models.FlowInformation) *models.PccRule {
	found := false
	set := make(map[string]models.FlowInformation)

	for _, flowInfo := range flowInfos {
		set[flowInfo.FlowDescription] = flowInfo
	}

	for _, pccRule := range pccRules {
		found = true
		for _, flowInfo := range pccRule.FlowInfos {
			if _, exists := set[flowInfo.FlowDescription]; !exists {
				found = false
				break
			}
		}
		if found {
			return pccRule
		}
	}
	return nil
}

func SetPccRuleRelatedData(decicion *models.SmPolicyDecision, pccRule *models.PccRule,
	tcData *models.TrafficControlData, qosData *models.QosData, chgData *models.ChargingData,
	umData *models.UsageMonitoringData,
) {
	if tcData != nil {
		if decicion.TraffContDecs == nil {
			decicion.TraffContDecs = make(map[string]*models.TrafficControlData)
		}
		decicion.TraffContDecs[tcData.TcId] = tcData
		pccRule.RefTcData = []string{tcData.TcId}
	}
	if qosData != nil {
		if decicion.QosDecs == nil {
			decicion.QosDecs = make(map[string]*models.QosData)
		}
		decicion.QosDecs[qosData.QosId] = qosData
		pccRule.RefQosData = []string{qosData.QosId}
	}
	if chgData != nil {
		if decicion.ChgDecs == nil {
			decicion.ChgDecs = make(map[string]*models.ChargingData)
		}
		decicion.ChgDecs[chgData.ChgId] = chgData
		pccRule.RefChgData = []string{chgData.ChgId}
	}
	if umData != nil {
		if decicion.UmDecs == nil {
			decicion.UmDecs = make(map[string]*models.UsageMonitoringData)
		}
		decicion.UmDecs[umData.UmId] = umData
		pccRule.RefUmData = []string{umData.UmId}
	}
	if pccRule != nil {
		if decicion.PccRules == nil {
			decicion.PccRules = make(map[string]*models.PccRule)
		}
		decicion.PccRules[pccRule.PccRuleId] = pccRule
	}
}
