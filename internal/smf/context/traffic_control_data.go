package context

import (
	"github.com/omec-project/openapi/models"
)

// TrafficControlData - Traffic control data defines how traffic data flows
// associated with a rule are treated (e.g. blocked, redirected).
type TrafficControlData struct {
	// maybe include attribute
	UpPathChgEvent *models.UpPathChgEvent
	FlowStatus     models.FlowStatus

	// referenced dataType
	refedPCCRule map[string]string

	TrafficControlID string                   // mandatory attribute
	RouteToLocs      []models.RouteToLocation // not mandatory attribute
}

// NewTrafficControlDataFromModel - create the traffic control data from OpenAPI model
func NewTrafficControlDataFromModel(model *models.TrafficControlData) *TrafficControlData {
	trafficControlData := new(TrafficControlData)

	trafficControlData.TrafficControlID = model.TcId
	trafficControlData.FlowStatus = model.FlowStatus
	trafficControlData.RouteToLocs = model.RouteToLocs
	trafficControlData.UpPathChgEvent = model.UpPathChgEvent

	trafficControlData.refedPCCRule = make(map[string]string)

	return trafficControlData
}

// RefedPCCRules - returns the PCCRules that reference this tcData
func (tc *TrafficControlData) RefedPCCRules() map[string]string {
	return tc.refedPCCRule
}

func (tc *TrafficControlData) AddRefedPCCRules(PCCref string) {
	tc.refedPCCRule[PCCref] = PCCref
}

func (tc *TrafficControlData) DeleteRefedPCCRules(PCCref string) {
	delete(tc.refedPCCRule, PCCref)
}
