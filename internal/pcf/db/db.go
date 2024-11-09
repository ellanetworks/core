package db

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/idgenerator"
	"github.com/yeastengine/ella/internal/db/sql"
	"github.com/yeastengine/ella/internal/nssf/factory"
	pcf_context "github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/logger"
)

func getBitRateUnit(val int64) (int64, string) {
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

func getSessionRule(deviceGroup sql.DeviceGroup) (sessionRule *models.SessionRule) {
	sessionRule = &models.SessionRule{}
	sessionRule.AuthDefQos = &models.AuthorizedDefaultQos{
		Var5qi: int32(deviceGroup.TrafficClassQci),
		Arp:    &models.Arp{PriorityLevel: int32(deviceGroup.TrafficClassArp)},
	}
	ul, uunit := getBitRateUnit(deviceGroup.DnnMbrUplink)
	dl, dunit := getBitRateUnit(deviceGroup.DnnMbrDownlink)
	sessionRule.AuthSessAmbr = &models.Ambr{
		Uplink:   strconv.FormatInt(ul, 10) + uunit,
		Downlink: strconv.FormatInt(dl, 10) + dunit,
	}
	return sessionRule
}

func GetSubscriberPolicy(imsi string) (*pcf_context.PcfSubscriberPolicyData, error) {
	queries := factory.NssfConfig.Configuration.DBQueries
	if queries == nil {
		return nil, fmt.Errorf("db queries not initialized")
	}
	subscriber, err := queries.GetSubscriberByImsi(context.Background(), imsi)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber by imsi: %w", err)
	}
	deviceGroupID := subscriber.DeviceGroupID
	deviceGroup, err := queries.GetDeviceGroup(context.Background(), deviceGroupID.Int64)
	if err != nil {
		return nil, fmt.Errorf("couldn't get device group: %w", err)
	}
	networkSliceID := deviceGroup.NetworkSliceID
	networkSlice, err := queries.GetNetworkSlice(context.Background(), networkSliceID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get network slice: %w", err)
	}
	sliceid := fmt.Sprintf("%c%v", networkSlice.Sst, networkSlice.Sd)
	logger.CtxLog.Warnf("TO DELETE: sliceid: %v", sliceid)

	sessionrule := getSessionRule(deviceGroup)

	policyData := &pcf_context.PcfSubscriberPolicyData{}
	policyData.CtxLog = logger.CtxLog.WithField(logger.FieldSupi, "imsi-"+imsi)
	policyData.PccPolicy = make(map[string]*pcf_context.PccPolicy)
	policyData.PccPolicy[sliceid] = &pcf_context.PccPolicy{
		PccRules: make(map[string]*models.PccRule),
		QosDecs:  make(map[string]*models.QosData), TraffContDecs: make(map[string]*models.TrafficControlData),
		SessionPolicy: make(map[string]*pcf_context.SessionPolicy), IdGenerator: nil,
	}
	policyData.PccPolicy[sliceid].SessionPolicy[deviceGroup.Dnn] = &pcf_context.SessionPolicy{SessionRules: make(map[string]*models.SessionRule), SessionRuleIdGenerator: idgenerator.NewGenerator(1, math.MaxInt16)}
	id, err := policyData.PccPolicy[sliceid].SessionPolicy[deviceGroup.Dnn].SessionRuleIdGenerator.Allocate()
	if err != nil {
		return nil, fmt.Errorf("couldn't allocate session rule id: %w", err)
	}
	sessionrule.SessRuleId = deviceGroup.Dnn + "-" + strconv.Itoa(int(id))
	policyData.PccPolicy[sliceid].SessionPolicy[deviceGroup.Dnn].SessionRules[sessionrule.SessRuleId] = sessionrule
	// Note from Guillaume: Here, we removed the use of application filters, as they did not seem to be used in the codebase.
	policyData.CtxLog.Infof("Subscriber Detals: %v", policyData)
	return policyData, nil
}
