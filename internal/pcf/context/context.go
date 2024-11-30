package context

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/idgenerator"
	"github.com/yeastengine/ella/internal/pcf/factory"
	"github.com/yeastengine/ella/internal/pcf/logger"
	"github.com/yeastengine/ella/internal/webui/configapi"
)

var pcfCtx *PCFContext

func init() {
	pcfCtx = new(PCFContext)
	pcfCtx.Name = "pcf"
	pcfCtx.UriScheme = models.UriScheme_HTTP
	pcfCtx.TimeFormat = "2006-01-02 15:04:05"
	pcfCtx.DefaultBdtRefId = "BdtPolicyId-"
	pcfCtx.NfService = make(map[models.ServiceName]models.NfService)
	pcfCtx.PcfServiceUris = make(map[models.ServiceName]string)
	pcfCtx.PcfSuppFeats = make(map[models.ServiceName]openapi.SupportedFeature)
	pcfCtx.BdtPolicyIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
	pcfCtx.SessionRuleIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
	pcfCtx.QoSDataIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
}

type PlmnSupportItem struct {
	PlmnId models.PlmnId
}

type PCFContext struct {
	NfId            string
	Name            string
	UriScheme       models.UriScheme
	BindingIPv4     string
	TimeFormat      string
	DefaultBdtRefId string
	NfService       map[models.ServiceName]models.NfService
	PcfServiceUris  map[models.ServiceName]string
	PcfSuppFeats    map[models.ServiceName]openapi.SupportedFeature
	AmfUri          string
	UdrUri          string
	// UePool          map[string]*UeContext
	UePool sync.Map
	// Bdt Policy related
	BdtPolicyPool        sync.Map
	BdtPolicyIDGenerator *idgenerator.IDGenerator
	// App Session related
	AppSessionPool sync.Map
	// AMF Status Change Subscription related
	AMFStatusSubsData sync.Map // map[string]AMFStatusSubscriptionData; subscriptionID as key

	DnnList []string
	SBIPort int
	// lock
	DefaultUdrURILock sync.RWMutex

	SessionRuleIDGenerator *idgenerator.IDGenerator
	QoSDataIDGenerator     *idgenerator.IDGenerator
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

func GetUri(name models.ServiceName) string {
	return pcfCtx.PcfServiceUris[name]
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

func (c *PCFContext) GetIPv4Uri() string {
	return fmt.Sprintf("%s://%s:%d", c.UriScheme, c.BindingIPv4, c.SBIPort)
}

// Init NfService with supported service list ,and version of services
func (c *PCFContext) InitNFService(serviceList []factory.Service) {
	for index, service := range serviceList {
		name := models.ServiceName(service.ServiceName)
		c.NfService[name] = models.NfService{
			ServiceInstanceId: strconv.Itoa(index),
			ServiceName:       name,
			Scheme:            c.UriScheme,
			NfServiceStatus:   models.NfServiceStatus_REGISTERED,
			ApiPrefix:         c.GetIPv4Uri(),
			IpEndPoints: &[]models.IpEndPoint{
				{
					Ipv4Address: c.BindingIPv4,
					Transport:   models.TransportProtocol_TCP,
					Port:        int32(c.SBIPort),
				},
			},
			SupportedFeatures: service.SuppFeat,
		}
	}
}

// Allocate PCF Ue with supi and add to pcf Context and returns allocated ue
func (c *PCFContext) NewPCFUe(Supi string) (*UeContext, error) {
	if strings.HasPrefix(Supi, "imsi-") {
		newUeContext := &UeContext{}
		newUeContext.SmPolicyData = make(map[string]*UeSmPolicyData)
		newUeContext.AMPolicyData = make(map[string]*UeAMPolicyData)
		newUeContext.PolAssociationIDGenerator = 1
		newUeContext.AppSessionIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
		newUeContext.Supi = Supi
		c.UePool.Store(Supi, newUeContext)
		return newUeContext, nil
	} else {
		return nil, fmt.Errorf(" add Ue context fail ")
	}
}

// Return Bdt Policy Id with format "BdtPolicyId-%d" which be allocated
func (c *PCFContext) AllocBdtPolicyID() (bdtPolicyID string, err error) {
	var allocID int64
	if allocID, err = c.BdtPolicyIDGenerator.Allocate(); err != nil {
		logger.CtxLog.Warnf("Allocate pathID error: %+v", err)
		return "", err
	}

	bdtPolicyID = fmt.Sprintf("BdtPolicyId-%d", allocID)
	return bdtPolicyID, nil
}

// Find PcfUe which the policyId belongs to
func (c *PCFContext) PCFUeFindByPolicyId(PolicyId string) *UeContext {
	index := strings.LastIndex(PolicyId, "-")
	if index == -1 {
		return nil
	}
	supi := PolicyId[:index]
	if supi != "" {
		if value, ok := c.UePool.Load(supi); ok {
			ueContext := value.(*UeContext)
			return ueContext
		}
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
		// TODO: find by MAC address
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
	plmnSupportList := make([]PlmnSupportItem, 0)
	networkSliceNames := configapi.ListNetworkSlices()
	for _, networkSliceName := range networkSliceNames {
		networkSlice := configapi.GetNetworkSliceByName2(networkSliceName)
		plmnID := models.PlmnId{
			Mcc: networkSlice.SiteInfo.Plmn.Mcc,
			Mnc: networkSlice.SiteInfo.Plmn.Mnc,
		}
		plmnSupportItem := PlmnSupportItem{
			PlmnId: plmnID,
		}
		plmnSupportList = append(plmnSupportList, plmnSupportItem)
	}
	return plmnSupportList
}

func GetSubscriberPolicies() map[string]*PcfSubscriberPolicyData {
	subscriberPolicies := make(map[string]*PcfSubscriberPolicyData)

	networkSliceNames := configapi.ListNetworkSlices()
	for _, networkSliceName := range networkSliceNames {
		networkSlice := configapi.GetNetworkSliceByName2(networkSliceName)
		pccPolicyId := networkSlice.SliceId.Sst + networkSlice.SliceId.Sd
		deviceGroupNames := networkSlice.SiteDeviceGroup
		for _, devGroupName := range deviceGroupNames {
			deviceGroup := configapi.GetDeviceGroupByName2(devGroupName)
			for _, imsi := range deviceGroup.Imsis {
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

				dnn := deviceGroup.IpDomainExpanded.Dnn
				if _, exists := subscriberPolicies[imsi].PccPolicy[pccPolicyId].SessionPolicy[dnn]; !exists {
					subscriberPolicies[imsi].PccPolicy[pccPolicyId].SessionPolicy[dnn] = &SessionPolicy{
						SessionRules: make(map[string]*models.SessionRule),
					}
				}

				// Generate IDs using ID generators
				qosId, _ := pcfCtx.QoSDataIDGenerator.Allocate()
				sessionRuleId, _ := pcfCtx.SessionRuleIDGenerator.Allocate()

				ul, uunit := GetBitRateUnit(deviceGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink)
				dl, dunit := GetBitRateUnit(deviceGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink)

				// Create QoS data
				qosData := &models.QosData{
					QosId:                strconv.FormatInt(qosId, 10),
					Var5qi:               deviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Qci,
					MaxbrUl:              strconv.FormatInt(ul, 10) + uunit,
					MaxbrDl:              strconv.FormatInt(dl, 10) + dunit,
					Arp:                  &models.Arp{PriorityLevel: deviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Arp},
					DefQosFlowIndication: true,
				}
				subscriberPolicies[imsi].PccPolicy[pccPolicyId].QosDecs[qosData.QosId] = qosData

				// Add session rule
				subscriberPolicies[imsi].PccPolicy[pccPolicyId].SessionPolicy[dnn].SessionRules[strconv.FormatInt(sessionRuleId, 10)] = &models.SessionRule{
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
	}

	return subscriberPolicies
}

func GetBitRateUnit(val int64) (int64, string) {
	unit := " Kbps"
	if val < 1000 {
		logger.GrpcLog.Warnf("configured value [%v] is lesser than 1000 bps, so setting 1 Kbps", val)
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
