// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
)

var pcfCtx *PCFContext

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
	newUeContext := &UeContext{}
	newUeContext.SmPolicyData = make(map[string]*UeSmPolicyData)
	newUeContext.AMPolicyData = make(map[string]*UeAMPolicyData)
	newUeContext.PolAssociationIDGenerator = 1
	newUeContext.Supi = Supi
	c.UePool.Store(Supi, newUeContext)
	return newUeContext, nil
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

func GetSubscriberPolicy(imsi string) *PcfSubscriberPolicyData {
	subscriber, err := pcfCtx.DbInstance.GetSubscriber(imsi)
	if err != nil {
		logger.PcfLog.Warnf("Failed to get subscriber %s: %+v", imsi, err)
		return nil
	}
	profile, err := pcfCtx.DbInstance.GetProfileByID(subscriber.ProfileID)
	if err != nil {
		logger.PcfLog.Warnf("Failed to get profile %d: %+v", subscriber.ProfileID, err)
		return nil
	}
	subscriberPolicies := &PcfSubscriberPolicyData{
		Supi:      imsi,
		PccPolicy: make(map[string]*PccPolicy),
	}
	pccPolicyId := fmt.Sprintf("%d%s", config.Sst, config.Sd)
	if _, exists := subscriberPolicies.PccPolicy[pccPolicyId]; !exists {
		subscriberPolicies.PccPolicy[pccPolicyId] = &PccPolicy{
			SessionPolicy: make(map[string]*SessionPolicy),
			PccRules:      make(map[string]*models.PccRule),
			QosDecs:       make(map[string]*models.QosData),
			TraffContDecs: make(map[string]*models.TrafficControlData),
		}
	}

	if _, exists := subscriberPolicies.PccPolicy[pccPolicyId].SessionPolicy[config.DNN]; !exists {
		subscriberPolicies.PccPolicy[pccPolicyId].SessionPolicy[config.DNN] = &SessionPolicy{
			SessionRules: make(map[string]*models.SessionRule),
		}
	}

	// Generate IDs using ID generators
	qosId, _ := pcfCtx.QoSDataIDGenerator.Allocate()
	sessionRuleId, _ := pcfCtx.SessionRuleIDGenerator.Allocate()

	// Create QoS data
	qosData := &models.QosData{
		QosId:                strconv.FormatInt(qosId, 10),
		Var5qi:               profile.Var5qi,
		MaxbrUl:              profile.BitrateUplink,
		MaxbrDl:              profile.BitrateDownlink,
		Arp:                  &models.Arp{PriorityLevel: profile.PriorityLevel},
		DefQosFlowIndication: true,
	}
	subscriberPolicies.PccPolicy[pccPolicyId].QosDecs[qosData.QosId] = qosData

	// Add session rule
	subscriberPolicies.PccPolicy[pccPolicyId].SessionPolicy[config.DNN].SessionRules[strconv.FormatInt(sessionRuleId, 10)] = &models.SessionRule{
		SessRuleId: strconv.FormatInt(sessionRuleId, 10),
		AuthDefQos: &models.AuthorizedDefaultQos{
			Var5qi: qosData.Var5qi,
			Arp:    qosData.Arp,
		},
		AuthSessAmbr: &models.Ambr{
			Uplink:   profile.BitrateUplink,
			Downlink: profile.BitrateDownlink,
		},
	}
	return subscriberPolicies
}
