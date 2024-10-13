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
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/pcf/factory"
	"github.com/yeastengine/ella/internal/pcf/logger"
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
	pcfCtx.PcfSubscriberPolicyData = make(map[string]*PcfSubscriberPolicyData)
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
	AMFStatusSubsData       sync.Map                            // map[string]AMFStatusSubscriptionData; subscriptionID as key
	PcfSubscriberPolicyData map[string]*PcfSubscriberPolicyData // subscriberId is key

	DnnList  []string
	PlmnList []factory.PlmnSupportItem
	SBIPort  int
	// lock
	DefaultUdrURILock sync.RWMutex
}

type SessionPolicy struct {
	SessionRules           map[string]*models.SessionRule
	SessionRuleIdGenerator *idgenerator.IDGenerator
}

type PccPolicy struct {
	PccRules      map[string]*models.PccRule
	QosDecs       map[string]*models.QosData
	TraffContDecs map[string]*models.TrafficControlData
	SessionPolicy map[string]*SessionPolicy // dnn is key
	IdGenerator   *idgenerator.IDGenerator
}
type PcfSubscriberPolicyData struct {
	PccPolicy map[string]*PccPolicy // sst+sd is key
	CtxLog    *logrus.Entry
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
func (c *PCFContext) InitNFService(serviceList []factory.Service, version string) {
	tmpVersion := strings.Split(version, ".")
	versionUri := "v" + tmpVersion[0]
	for index, service := range serviceList {
		name := models.ServiceName(service.ServiceName)
		c.NfService[name] = models.NfService{
			ServiceInstanceId: strconv.Itoa(index),
			ServiceName:       name,
			Versions: &[]models.NfServiceVersion{
				{
					ApiFullVersion:  version,
					ApiVersionInUri: versionUri,
				},
			},
			Scheme:          c.UriScheme,
			NfServiceStatus: models.NfServiceStatus_REGISTERED,
			ApiPrefix:       c.GetIPv4Uri(),
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

// Find PcfUe which Ipv6 belongs to
func (c *PCFContext) PcfUeFindByIPv6(v6 string) *UeContext {
	var ue *UeContext
	c.UePool.Range(func(key, value interface{}) bool {
		ue = value.(*UeContext)
		if ue.SMPolicyFindByIpv6(v6) != nil {
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
			err = fmt.Errorf("Can't find Ue with Ipv4[%s]", req.UeIpv4)
		}
	} else if req.UeIpv6 != "" {
		policy = ue.SMPolicyFindByIdentifiersIpv6(req.UeIpv6, req.SliceInfo, req.Dnn)
		if policy == nil {
			err = fmt.Errorf("Can't find Ue with Ipv6 prefix[%s]", req.UeIpv6)
		}
	} else {
		// TODO: find by MAC address
		err = fmt.Errorf("Ue finding by MAC address does not support")
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
		err = fmt.Errorf("No SM policy found")
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

func GetIpv6Address(ipindex int32) string {
	return Ipv6_pool[fmt.Sprint(ipindex)]
}

func DeleteIpv6index(Ipv6index int32) {
	delete(Ipv6_pool, fmt.Sprint(Ipv6index))
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

func (c *PCFContext) DisplayPcfSubscriberPolicyData(imsi string) {
	logger.CtxLog.Infof("Pcf Subscriber [%v] Policy Details :", imsi)
	subs, exist := pcfCtx.PcfSubscriberPolicyData[imsi]
	if !exist {
		logger.CtxLog.Infof("Pcf Subscriber [%v] not exist", imsi)
	} else {
		for slice, val := range subs.PccPolicy {
			subs.CtxLog.Infof("   SliceId: %v", slice)
			for name, srule := range val.SessionPolicy {
				subs.CtxLog.Infof("   Session-Name/Dnn: %v", name)
				for _, srules := range srule.SessionRules {
					logger.CtxLog.Infof("   SessionRuleId: %v", srules.SessRuleId)
					if srules.AuthSessAmbr != nil {
						logger.CtxLog.Infof("   AmbrUplink  %v", srules.AuthSessAmbr.Uplink)
						logger.CtxLog.Infof("   AmbrDownlink  %v", srules.AuthSessAmbr.Downlink)
					}
					if srules.AuthDefQos != nil {
						logger.CtxLog.Infof("    DefQos.5qi: %v", srules.AuthDefQos.Var5qi)
						if srules.AuthDefQos.Arp != nil {
							logger.CtxLog.Infof("    DefQos.Arp.PriorityLevel: %v", srules.AuthDefQos.Arp.PriorityLevel)
							logger.CtxLog.Infof("    DefQos.Arp.PreemptCapability: %v", srules.AuthDefQos.Arp.PreemptCap)
							logger.CtxLog.Infof("    DefQos.Arp.PreemptVulnerability: %v", srules.AuthDefQos.Arp.PreemptVuln)
						}
						logger.CtxLog.Infof("    DefQos.prioritylevel: %v", srules.AuthDefQos.PriorityLevel)
					}
				}
			}
			for rulename, rule := range val.PccRules {
				logger.CtxLog.Infof("   PccRule-Name: %v", rulename)
				logger.CtxLog.Infof("   PccRule-Id: %v", rule.PccRuleId)
				logger.CtxLog.Infof("   Precedence: %v", rule.Precedence)

				for _, flow := range rule.FlowInfos {
					logger.CtxLog.Infof("   FlowDescription: %v", flow.FlowDescription)
					logger.CtxLog.Infof("   TosTrafficClass: %v", flow.TosTrafficClass)
					logger.CtxLog.Infof("   FlowDirection: %v", flow.FlowDirection)
				}
			}
			logger.CtxLog.Infof("   Qos Details")
			for _, qos := range val.QosDecs {
				logger.CtxLog.Infof("     QosId: %v", qos.QosId)
				logger.CtxLog.Infof("     5qi: %v", qos.Var5qi)
				logger.CtxLog.Infof("     MaxbrUl: %v", qos.MaxbrUl)
				logger.CtxLog.Infof("     MaxbrDl: %v", qos.MaxbrDl)
				logger.CtxLog.Infof("     GbrDl: %v", qos.GbrDl)
				logger.CtxLog.Infof("     GbrUl: %v", qos.GbrUl)
				logger.CtxLog.Infof("     PriorityLevel: %v", qos.PriorityLevel)
				if qos.Arp != nil {
					logger.CtxLog.Infof("    Arp.PreemptCapability: %v", qos.Arp.PreemptCap)
					logger.CtxLog.Infof("    Arp.PreemptVulnerability: %v", qos.Arp.PreemptVuln)
				}
			}

			logger.CtxLog.Infof("   Traffic Control Details")
			for _, t := range val.TraffContDecs {
				logger.CtxLog.Infof("     TcId: %v", t.TcId)
				logger.CtxLog.Infof("     FlowStatus: %v", t.FlowStatus)
			}
		}
	}
}
