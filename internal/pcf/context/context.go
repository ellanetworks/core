package context

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/config"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/util/idgenerator"
)

var pcfCtx *PCFContext

func init() {
	pcfCtx = &PCFContext{}
}

type PlmnSupportItem struct {
	PlmnId models.PlmnId
}

type PCFContext struct {
	TimeFormat             string
	PcfSuppFeats           map[models.ServiceName]openapi.SupportedFeature
	UePool                 sync.Map
	AppSessionPool         sync.Map
	AMFStatusSubsData      sync.Map // map[string]AMFStatusSubscriptionData; subscriptionID as key
	DefaultUdrURILock      sync.RWMutex
	SessionRuleIDGenerator *idgenerator.IDGenerator
	QoSDataIDGenerator     *idgenerator.IDGenerator
	DbInstance             *db.Database
}

type SessionPolicy struct {
	SessionRules map[string]*models.SessionRule
}

type PccPolicy struct {
	PccRules      map[string]*models.PccRule
	QosDecs       map[string]*models.QosData
	TraffContDecs map[string]*models.TrafficControlData
	SessionPolicy map[string]*SessionPolicy // dnn is key
}

type PcfSubscriberPolicyData struct {
	PccPolicy map[string]*PccPolicy // sst+sd is key
	Supi      string
}

type AMFStatusSubscriptionData struct {
	AmfUri       string
	AmfStatusUri string
	GuamiList    []models.Guami
}

type AppSessionData struct {
	AppSessionContext *models.AppSessionContext
	// (compN/compN-subCompN/appId-%s) map to PccRule
	RelatedPccRuleIds    map[string]string
	PccRuleIdMapToCompId map[string]string
	// related Session
	SmPolicyData *UeSmPolicyData
	// EventSubscription
	Events   map[models.AfEvent]models.AfNotifMethod
	EventUri string

	AppSessionId string
}

// Create new PCF context
func PCF_Self() *PCFContext {
	return pcfCtx
}

func GetTimeformat() string {
	return pcfCtx.TimeFormat
}

var (
	PolicyAuthorizationUri = "/npcf-policyauthorization/v1/app-sessions/"
	SmUri                  = "/npcf-smpolicycontrol/v1"
	IPv4Address            = "192.168."
	IPv6Address            = "ffab::"
	CheckNotifiUri         = "/npcf-callback/v1/nudr-notify/"
	Ipv4_pool              = make(map[string]string)
	Ipv6_pool              = make(map[string]string)
)

// BdtPolicy default value
const DefaultBdtRefId = "BdtPolicyId-"

// Allocate PCF Ue with supi and add to pcf Context and returns allocated ue
func (c *PCFContext) NewPCFUe(Supi string) (*UeContext, error) {
	if strings.HasPrefix(Supi, "imsi-") {
		newUeContext := &UeContext{}
		newUeContext.SmPolicyData = make(map[string]*UeSmPolicyData)
		newUeContext.AMPolicyData = make(map[string]*UeAMPolicyData)
		newUeContext.PolAssociationIDGenerator = 1
		newUeContext.AppSessionIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
		newUeContext.Supi = Supi
		logger.PcfLog.Warnf("Storing new UeContext with Supi[%s]", Supi)
		c.UePool.Store(Supi, newUeContext)
		return newUeContext, nil
	} else {
		return nil, fmt.Errorf(" add Ue context fail ")
	}
}

// Find PcfUe which the policyId belongs to
func (c *PCFContext) PCFUeFindByPolicyId(PolicyId string) *UeContext {
	index := strings.LastIndex(PolicyId, "-")
	if index == -1 {
		logger.PcfLog.Errorf("Invalid PolicyId format: %s", PolicyId)
		return nil
	}
	supi := PolicyId[:index]
	if value, ok := c.UePool.Load(supi); ok {
		return value.(*UeContext)
	}
	return nil
}

// Find PcfUe which the AppSessionId belongs to
func (c *PCFContext) PCFUeFindByAppSessionId(appSessionId string) *UeContext {
	index := strings.LastIndex(appSessionId, "-")
	if index == -1 {
		return nil
	}
	supi := appSessionId[:index]
	if supi != "" {
		if value, ok := c.UePool.Load(supi); ok {
			ueContext := value.(*UeContext)
			return ueContext
		}
	}
	return nil
}

// Find PcfUe which Ipv4 belongs to
func (c *PCFContext) PcfUeFindByIPv4(v4 string) *UeContext {
	var ue *UeContext
	c.UePool.Range(func(key, value interface{}) bool {
		ue = value.(*UeContext)
		if ue.SMPolicyFindByIpv4(v4) != nil {
			return false
		} else {
			return true
		}
	})

	return ue
}

// Find SMPolicy with AppSessionContext
func ueSMPolicyFindByAppSessionContext(ue *UeContext, req *models.AppSessionContextReqData) (*UeSmPolicyData, error) {
	var policy *UeSmPolicyData
	var err error

	if req.UeIpv4 != "" {
		policy = ue.SMPolicyFindByIdentifiersIpv4(req.UeIpv4, req.SliceInfo, req.Dnn, req.IpDomain)
		if policy == nil {
			err = fmt.Errorf("can't find Ue with Ipv4[%s]", req.UeIpv4)
		}
	} else if req.UeIpv6 != "" {
		policy = ue.SMPolicyFindByIdentifiersIpv6(req.UeIpv6, req.SliceInfo, req.Dnn)
		if policy == nil {
			err = fmt.Errorf("can't find Ue with Ipv6 prefix[%s]", req.UeIpv6)
		}
	} else {
		err = fmt.Errorf("UE finding by MAC address does not support")
	}
	return policy, err
}

// SessionBinding from application request to get corresponding Sm policy
func (c *PCFContext) SessionBinding(req *models.AppSessionContextReqData) (*UeSmPolicyData, error) {
	var selectedUE *UeContext
	var policy *UeSmPolicyData
	var err error

	if req.Supi != "" {
		if val, exist := c.UePool.Load(req.Supi); exist {
			selectedUE = val.(*UeContext)
		}
	}

	if req.Gpsi != "" && selectedUE == nil {
		c.UePool.Range(func(key, value interface{}) bool {
			ue := value.(*UeContext)
			if ue.Gpsi == req.Gpsi {
				selectedUE = ue
				return false
			} else {
				return true
			}
		})
	}

	if selectedUE != nil {
		policy, err = ueSMPolicyFindByAppSessionContext(selectedUE, req)
	} else {
		c.UePool.Range(func(key, value interface{}) bool {
			ue := value.(*UeContext)
			policy, err = ueSMPolicyFindByAppSessionContext(ue, req)
			return true
		})
	}
	if policy == nil && err == nil {
		err = fmt.Errorf("no SM policy found")
	}
	return policy, err
}

func Ipv4Pool(ipindex int32) string {
	ipv4address := IPv4Address + fmt.Sprint((int(ipindex)/255)+1) + "." + fmt.Sprint(int(ipindex)%255)
	return ipv4address
}

func Ipv4Index() int32 {
	if len(Ipv4_pool) == 0 {
		Ipv4_pool["1"] = Ipv4Pool(1)
	} else {
		for i := 1; i <= len(Ipv4_pool); i++ {
			if Ipv4_pool[fmt.Sprint(i)] == "" {
				Ipv4_pool[fmt.Sprint(i)] = Ipv4Pool(int32(i))
				return int32(i)
			}
		}

		Ipv4_pool[fmt.Sprint(int32(len(Ipv4_pool)+1))] = Ipv4Pool(int32(len(Ipv4_pool) + 1))
		return int32(len(Ipv4_pool))
	}
	return 1
}

func GetIpv4Address(ipindex int32) string {
	return Ipv4_pool[fmt.Sprint(ipindex)]
}

func DeleteIpv4index(Ipv4index int32) {
	delete(Ipv4_pool, fmt.Sprint(Ipv4index))
}

func Ipv6Pool(ipindex int32) string {
	ipv6address := IPv6Address + fmt.Sprintf("%x\n", ipindex)
	return ipv6address
}

func Ipv6Index() int32 {
	if len(Ipv6_pool) == 0 {
		Ipv6_pool["1"] = Ipv6Pool(1)
	} else {
		for i := 1; i <= len(Ipv6_pool); i++ {
			if Ipv6_pool[fmt.Sprint(i)] == "" {
				Ipv6_pool[fmt.Sprint(i)] = Ipv6Pool(int32(i))
				return int32(i)
			}
		}

		Ipv6_pool[fmt.Sprint(int32(len(Ipv6_pool)+1))] = Ipv6Pool(int32(len(Ipv6_pool) + 1))
		return int32(len(Ipv6_pool))
	}
	return 1
}

func (c *PCFContext) NewAmfStatusSubscription(subscriptionID string, subscriptionData AMFStatusSubscriptionData) {
	c.AMFStatusSubsData.Store(subscriptionID, subscriptionData)
}

func (subs PcfSubscriberPolicyData) String() string {
	var s string
	for slice, val := range subs.PccPolicy {
		s += fmt.Sprintf("PccPolicy[%v]: %v", slice, val)
		for rulename, rule := range val.PccRules {
			s += fmt.Sprintf("\n   PccRules[%v]: ", rulename)
			s += fmt.Sprintf("RuleId: %v, Precedence: %v, ", rule.PccRuleId, rule.Precedence)
			for i, flow := range rule.FlowInfos {
				s += fmt.Sprintf("FlowInfo[%v]: FlowDesc: %v, TrafficClass: %v, FlowDir: %v", i, flow.FlowDescription, flow.TosTrafficClass, flow.FlowDirection)
			}
		}
		for i, qos := range val.QosDecs {
			s += fmt.Sprintf("\n   QosDecs[%v] ", i)
			s += fmt.Sprintf("QosId: %v, 5Qi: %v, MaxbrUl: %v, MaxbrDl: %v, GbrUl: %v, GbrUl: %v,PL: %v ", qos.QosId, qos.Var5qi, qos.MaxbrUl, qos.MaxbrDl, qos.GbrDl, qos.GbrUl, qos.PriorityLevel)
			if qos.Arp != nil {
				s += fmt.Sprintf("PL: %v, PC: %v, PV: %v", qos.Arp.PriorityLevel, qos.Arp.PreemptCap, qos.Arp.PreemptVuln)
			}
		}
		for i, tr := range val.TraffContDecs {
			s += fmt.Sprintf("\n   TrafficDecs[%v]: ", i)
			s += fmt.Sprintf("TcId: %v, FlowStatus: %v", tr.TcId, tr.FlowStatus)
		}
	}
	return s
}

func (pcc PccPolicy) String() string {
	var s string
	for name, srule := range pcc.SessionPolicy {
		s += fmt.Sprintf("\n   SessionPolicy[%v]: %v ", name, srule)
	}
	return s
}

func (sess SessionPolicy) String() string {
	var s string
	for name, srule := range sess.SessionRules {
		s += fmt.Sprintf("\n    SessRule[%v]: SessionRuleId: %v, ", name, srule.SessRuleId)
		if srule.AuthDefQos != nil {
			s += fmt.Sprintf("AuthQos: 5Qi: %v, Arp: ", srule.AuthDefQos.Var5qi)
			if srule.AuthDefQos.Arp != nil {
				s += fmt.Sprintf("PL: %v, PC: %v, PV: %v", srule.AuthDefQos.Arp.PriorityLevel, srule.AuthDefQos.Arp.PreemptCap, srule.AuthDefQos.Arp.PreemptVuln)
			}
		}
		if srule.AuthSessAmbr != nil {
			s += fmt.Sprintf("AuthSessAmbr: Uplink: %v, Downlink: %v", srule.AuthSessAmbr.Uplink, srule.AuthSessAmbr.Downlink)
		}
	}
	return s
}

func GetPLMNList() []PlmnSupportItem {
	pcfSelf := PCF_Self()
	plmnSupportList := make([]PlmnSupportItem, 0)
	dbNetwork, err := pcfSelf.DbInstance.GetNetwork()
	if err != nil {
		logger.PcfLog.Warnf("Failed to get network slice names: %+v", err)
		return plmnSupportList
	}
	plmnID := models.PlmnId{
		Mcc: dbNetwork.Mcc,
		Mnc: dbNetwork.Mnc,
	}
	plmnSupportItem := PlmnSupportItem{
		PlmnId: plmnID,
	}
	plmnSupportList = append(plmnSupportList, plmnSupportItem)
	return plmnSupportList
}

func GetSubscriberPolicies() map[string]*PcfSubscriberPolicyData {
	pcfSelf := PCF_Self()
	subscriberPolicies := make(map[string]*PcfSubscriberPolicyData)
	pccPolicyId := fmt.Sprintf("%d%s", config.Sst, config.Sd)
	profiles, err := pcfSelf.DbInstance.ListProfiles()
	if err != nil {
		logger.PcfLog.Warnf("Failed to get profiles: %+v", err)
		return subscriberPolicies
	}
	for _, profile := range profiles {
		imsis, err := profile.GetImsis()
		if err != nil {
			logger.PcfLog.Warnf("Failed to get imsis from device group: %+v", err)
			continue
		}
		for _, imsi := range imsis {
			if _, exists := subscriberPolicies[imsi]; !exists {
				subscriberPolicies[imsi] = &PcfSubscriberPolicyData{
					Supi:      imsi,
					PccPolicy: make(map[string]*PccPolicy),
				}
			}

			if _, exists := subscriberPolicies[imsi].PccPolicy[pccPolicyId]; !exists {
				subscriberPolicies[imsi].PccPolicy[pccPolicyId] = &PccPolicy{
					SessionPolicy: make(map[string]*SessionPolicy),
					PccRules:      make(map[string]*models.PccRule),
					QosDecs:       make(map[string]*models.QosData),
					TraffContDecs: make(map[string]*models.TrafficControlData),
				}
			}

			if _, exists := subscriberPolicies[imsi].PccPolicy[pccPolicyId].SessionPolicy[config.DNN]; !exists {
				subscriberPolicies[imsi].PccPolicy[pccPolicyId].SessionPolicy[config.DNN] = &SessionPolicy{
					SessionRules: make(map[string]*models.SessionRule),
				}
			}

			// Generate IDs using ID generators
			qosId, _ := pcfCtx.QoSDataIDGenerator.Allocate()
			sessionRuleId, _ := pcfCtx.SessionRuleIDGenerator.Allocate()

			ul, uunit := GetBitRateUnit(profile.BitrateUplink)
			dl, dunit := GetBitRateUnit(profile.BitrateDownlink)

			// Create QoS data
			qosData := &models.QosData{
				QosId:                strconv.FormatInt(qosId, 10),
				Var5qi:               profile.Var5qi,
				MaxbrUl:              strconv.FormatInt(ul, 10) + uunit,
				MaxbrDl:              strconv.FormatInt(dl, 10) + dunit,
				Arp:                  &models.Arp{PriorityLevel: profile.Arp},
				DefQosFlowIndication: true,
			}
			subscriberPolicies[imsi].PccPolicy[pccPolicyId].QosDecs[qosData.QosId] = qosData

			// Add session rule
			subscriberPolicies[imsi].PccPolicy[pccPolicyId].SessionPolicy[config.DNN].SessionRules[strconv.FormatInt(sessionRuleId, 10)] = &models.SessionRule{
				SessRuleId: strconv.FormatInt(sessionRuleId, 10),
				AuthDefQos: &models.AuthorizedDefaultQos{
					Var5qi: qosData.Var5qi,
					Arp:    qosData.Arp,
				},
				AuthSessAmbr: &models.Ambr{
					Uplink:   strconv.FormatInt(ul, 10) + uunit,
					Downlink: strconv.FormatInt(dl, 10) + dunit,
				},
			}
		}
	}

	return subscriberPolicies
}

func GetBitRateUnit(val int64) (int64, string) {
	unit := " Kbps"
	if val < 1000 {
		logger.PcfLog.Warnf("configured value [%v] is lesser than 1000 bps, so setting 1 Kbps", val)
		val = 1
		return val, unit
	}
	if val >= 0xFFFF {
		val = (val / 1000)
		unit = " Kbps"
		if val >= 0xFFFF {
			val = (val / 1000)
			unit = " Mbps"
		}
		if val >= 0xFFFF {
			val = (val / 1000)
			unit = " Gbps"
		}
	} else {
		// minimum supported is kbps by SMF/UE
		val = val / 1000
	}

	return val, unit
}
