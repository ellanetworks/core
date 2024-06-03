// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package producer

import (
	"net/http"
	"strconv"
	"time"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/canard/internal/amf/context"
	"github.com/yeastengine/canard/internal/amf/logger"
)

func HandleCreateAMFEventSubscription(request *httpwrapper.Request) *httpwrapper.Response {
	createEventSubscription := request.Body.(models.AmfCreateEventSubscription)

	createdEventSubscription, problemDetails := CreateAMFEventSubscriptionProcedure(createEventSubscription)
	if createdEventSubscription != nil {
		return httpwrapper.NewResponse(http.StatusCreated, nil, createdEventSubscription)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "UNSPECIFIED_NF_FAILURE",
		}
		return httpwrapper.NewResponse(http.StatusInternalServerError, nil, problemDetails)
	}
}

// TODO: handle event filter
func CreateAMFEventSubscriptionProcedure(createEventSubscription models.AmfCreateEventSubscription) (
	*models.AmfCreatedEventSubscription, *models.ProblemDetails,
) {
	amfSelf := context.AMF_Self()

	createdEventSubscription := &models.AmfCreatedEventSubscription{}
	subscription := createEventSubscription.Subscription
	contextEventSubscription := &context.AMFContextEventSubscription{}
	contextEventSubscription.EventSubscription = *subscription
	var isImmediate bool
	var immediateFlags []bool
	var reportlist []models.AmfEventReport

	id, err := amfSelf.EventSubscriptionIDGenerator.Allocate()
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "UNSPECIFIED_NF_FAILURE",
		}
		return nil, problemDetails
	}
	newSubscriptionID := strconv.Itoa(int(id))

	// store subscription in context
	ueEventSubscription := context.AmfUeEventSubscription{}
	ueEventSubscription.EventSubscription = &contextEventSubscription.EventSubscription
	ueEventSubscription.Timestamp = time.Now().UTC()

	if subscription.Options != nil && subscription.Options.Trigger == models.AmfEventTrigger_CONTINUOUS {
		ueEventSubscription.RemainReports = new(int32)
		*ueEventSubscription.RemainReports = subscription.Options.MaxReports
	}
	for _, events := range *subscription.EventList {
		immediateFlags = append(immediateFlags, events.ImmediateFlag)
		if events.ImmediateFlag {
			isImmediate = true
		}
	}

	if subscription.AnyUE {
		contextEventSubscription.IsAnyUe = true
		ueEventSubscription.AnyUe = true
		amfSelf.UePool.Range(func(key, value interface{}) bool {
			ue := value.(*context.AmfUe)
			ue.EventSubscriptionsInfo[newSubscriptionID] = new(context.AmfUeEventSubscription)
			*ue.EventSubscriptionsInfo[newSubscriptionID] = ueEventSubscription
			contextEventSubscription.UeSupiList = append(contextEventSubscription.UeSupiList, ue.Supi)
			return true
		})
	} else if subscription.GroupId != "" {
		contextEventSubscription.IsGroupUe = true
		ueEventSubscription.AnyUe = true
		amfSelf.UePool.Range(func(key, value interface{}) bool {
			ue := value.(*context.AmfUe)
			if ue.GroupID == subscription.GroupId {
				ue.EventSubscriptionsInfo[newSubscriptionID] = new(context.AmfUeEventSubscription)
				*ue.EventSubscriptionsInfo[newSubscriptionID] = ueEventSubscription
				contextEventSubscription.UeSupiList = append(contextEventSubscription.UeSupiList, ue.Supi)
			}
			return true
		})
	} else {
		if ue, ok := amfSelf.AmfUeFindBySupi(subscription.Supi); !ok {
			problemDetails := &models.ProblemDetails{
				Status: http.StatusForbidden,
				Cause:  "UE_NOT_SERVED_BY_AMF",
			}
			return nil, problemDetails
		} else {
			ue.EventSubscriptionsInfo[newSubscriptionID] = new(context.AmfUeEventSubscription)
			*ue.EventSubscriptionsInfo[newSubscriptionID] = ueEventSubscription
			contextEventSubscription.UeSupiList = append(contextEventSubscription.UeSupiList, ue.Supi)
		}
	}

	// delete subscription
	if subscription.Options != nil {
		contextEventSubscription.Expiry = subscription.Options.Expiry
	}
	amfSelf.NewEventSubscription(newSubscriptionID, contextEventSubscription)

	// build response

	createdEventSubscription.Subscription = subscription
	createdEventSubscription.SubscriptionId = newSubscriptionID

	// for immediate use
	if subscription.AnyUE {
		amfSelf.UePool.Range(func(key, value interface{}) bool {
			ue := value.(*context.AmfUe)
			if isImmediate {
				subReports(ue, newSubscriptionID)
			}
			for i, flag := range immediateFlags {
				if flag {
					report, ok := NewAmfEventReport(ue, (*subscription.EventList)[i].Type, newSubscriptionID)
					if ok {
						reportlist = append(reportlist, report)
					}
				}
			}
			// delete subscription
			if reportlistLen := len(reportlist); reportlistLen > 0 && (!reportlist[reportlistLen-1].State.Active) {
				delete(ue.EventSubscriptionsInfo, newSubscriptionID)
			}
			return true
		})
	} else if subscription.GroupId != "" {
		amfSelf.UePool.Range(func(key, value interface{}) bool {
			ue := value.(*context.AmfUe)
			if isImmediate {
				subReports(ue, newSubscriptionID)
			}
			if ue.GroupID == subscription.GroupId {
				for i, flag := range immediateFlags {
					if flag {
						report, ok := NewAmfEventReport(ue, (*subscription.EventList)[i].Type, newSubscriptionID)
						if ok {
							reportlist = append(reportlist, report)
						}
					}
				}
				// delete subscription
				if reportlistLen := len(reportlist); reportlistLen > 0 && (!reportlist[reportlistLen-1].State.Active) {
					delete(ue.EventSubscriptionsInfo, newSubscriptionID)
				}
			}
			return true
		})
	} else {
		ue, _ := amfSelf.AmfUeFindBySupi(subscription.Supi)
		if isImmediate {
			subReports(ue, newSubscriptionID)
		}
		for i, flag := range immediateFlags {
			if flag {
				report, ok := NewAmfEventReport(ue, (*subscription.EventList)[i].Type, newSubscriptionID)
				if ok {
					reportlist = append(reportlist, report)
				}
			}
		}
		// delete subscription
		if reportlistLen := len(reportlist); reportlistLen > 0 && (!reportlist[reportlistLen-1].State.Active) {
			delete(ue.EventSubscriptionsInfo, newSubscriptionID)
		}
	}
	if len(reportlist) > 0 {
		createdEventSubscription.ReportList = reportlist
		// delete subscription
		if !reportlist[0].State.Active {
			amfSelf.DeleteEventSubscription(newSubscriptionID)
		}
	}

	return createdEventSubscription, nil
}

func HandleDeleteAMFEventSubscription(request *httpwrapper.Request) *httpwrapper.Response {
	logger.EeLog.Infoln("Handle Delete AMF Event Subscription")

	subscriptionID := request.Params["subscriptionId"]

	problemDetails := DeleteAMFEventSubscriptionProcedure(subscriptionID)
	if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	} else {
		return httpwrapper.NewResponse(http.StatusOK, nil, nil)
	}
}

func DeleteAMFEventSubscriptionProcedure(subscriptionID string) *models.ProblemDetails {
	amfSelf := context.AMF_Self()

	subscription, ok := amfSelf.FindEventSubscription(subscriptionID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "SUBSCRIPTION_NOT_FOUND",
		}
		return problemDetails
	}

	for _, supi := range subscription.UeSupiList {
		if ue, ok := amfSelf.AmfUeFindBySupi(supi); ok {
			delete(ue.EventSubscriptionsInfo, subscriptionID)
		}
	}
	amfSelf.DeleteEventSubscription(subscriptionID)
	return nil
}

func HandleModifyAMFEventSubscription(request *httpwrapper.Request) *httpwrapper.Response {
	logger.EeLog.Infoln("Handle Modify AMF Event Subscription")

	subscriptionID := request.Params["subscriptionId"]
	modifySubscriptionRequest := request.Body.(models.ModifySubscriptionRequest)

	updatedEventSubscription, problemDetails := ModifyAMFEventSubscriptionProcedure(subscriptionID,
		modifySubscriptionRequest)
	if updatedEventSubscription != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, updatedEventSubscription)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "UNSPECIFIED_NF_FAILURE",
		}
		return httpwrapper.NewResponse(http.StatusInternalServerError, nil, problemDetails)
	}
}

func ModifyAMFEventSubscriptionProcedure(
	subscriptionID string,
	modifySubscriptionRequest models.ModifySubscriptionRequest) (
	*models.AmfUpdatedEventSubscription, *models.ProblemDetails,
) {
	amfSelf := context.AMF_Self()

	contextSubscription, ok := amfSelf.FindEventSubscription(subscriptionID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "SUBSCRIPTION_NOT_FOUND",
		}
		return nil, problemDetails
	}

	if modifySubscriptionRequest.OptionItem != nil {
		contextSubscription.Expiry = modifySubscriptionRequest.OptionItem.Value
	} else if modifySubscriptionRequest.SubscriptionItemInner != nil {
		subscription := &contextSubscription.EventSubscription
		if !contextSubscription.IsAnyUe && !contextSubscription.IsGroupUe {
			if _, ok := amfSelf.AmfUeFindBySupi(subscription.Supi); !ok {
				problemDetails := &models.ProblemDetails{
					Status: http.StatusForbidden,
					Cause:  "UE_NOT_SERVED_BY_AMF",
				}
				return nil, problemDetails
			}
		}
		op := modifySubscriptionRequest.SubscriptionItemInner.Op
		index, err := strconv.Atoi(modifySubscriptionRequest.SubscriptionItemInner.Path[11:])
		if err != nil {
			problemDetails := &models.ProblemDetails{
				Status: http.StatusInternalServerError,
				Cause:  "UNSPECIFIED_NF_FAILURE",
			}
			return nil, problemDetails
		}
		lists := (*subscription.EventList)
		eventlistLen := len(*subscription.EventList)
		switch op {
		case "replace":
			event := *modifySubscriptionRequest.SubscriptionItemInner.Value
			if index < eventlistLen {
				(*subscription.EventList)[index] = event
			}
		case "remove":
			if index < eventlistLen {
				*subscription.EventList = append(lists[:index], lists[index+1:]...)
			}
		case "add":
			event := *modifySubscriptionRequest.SubscriptionItemInner.Value
			*subscription.EventList = append(lists, event)
		}
	}

	updatedEventSubscription := &models.AmfUpdatedEventSubscription{
		Subscription: &contextSubscription.EventSubscription,
	}
	return updatedEventSubscription, nil
}

func subReports(ue *context.AmfUe, subscriptionId string) {
	remainReport := ue.EventSubscriptionsInfo[subscriptionId].RemainReports
	if remainReport == nil {
		return
	}
	*remainReport--
}

// DO NOT handle AmfEventType_PRESENCE_IN_AOI_REPORT and AmfEventType_UES_IN_AREA_REPORT(about area)
func NewAmfEventReport(ue *context.AmfUe, Type models.AmfEventType, subscriptionId string) (
	report models.AmfEventReport, ok bool,
) {
	ueSubscription, ok := ue.EventSubscriptionsInfo[subscriptionId]
	if !ok {
		return report, ok
	}

	report.AnyUe = ueSubscription.AnyUe
	report.Supi = ue.Supi
	report.Type = Type
	report.TimeStamp = &ueSubscription.Timestamp
	report.State = new(models.AmfEventState)
	mode := ueSubscription.EventSubscription.Options
	if mode == nil {
		report.State.Active = true
	} else if mode.Trigger == models.AmfEventTrigger_ONE_TIME {
		report.State.Active = false
	} else if *ueSubscription.RemainReports <= 0 {
		report.State.Active = false
	} else {
		report.State.Active = getDuration(mode.Expiry, &report.State.RemainDuration)
		if report.State.Active {
			report.State.RemainReports = *ueSubscription.RemainReports
		}
	}

	switch Type {
	case models.AmfEventType_LOCATION_REPORT:
		report.Location = &ue.Location
	// case models.AmfEventType_PRESENCE_IN_AOI_REPORT:
	// report.AreaList = (*subscription.EventList)[eventIndex].AreaList
	case models.AmfEventType_TIMEZONE_REPORT:
		report.Timezone = ue.TimeZone
	case models.AmfEventType_ACCESS_TYPE_REPORT:
		for accessType, state := range ue.State {
			if state.Is(context.Registered) {
				report.AccessTypeList = append(report.AccessTypeList, accessType)
			}
		}
	case models.AmfEventType_REGISTRATION_STATE_REPORT:
		var rmInfos []models.RmInfo
		for accessType, state := range ue.State {
			rmInfo := models.RmInfo{
				RmState:    models.RmState_DEREGISTERED,
				AccessType: accessType,
			}
			if state.Is(context.Registered) {
				rmInfo.RmState = models.RmState_REGISTERED
			}
			rmInfos = append(rmInfos, rmInfo)
		}
		report.RmInfoList = rmInfos
	case models.AmfEventType_CONNECTIVITY_STATE_REPORT:
		report.CmInfoList = ue.GetCmInfo()
	case models.AmfEventType_REACHABILITY_REPORT:
		report.Reachability = ue.Reachability
	case models.AmfEventType_SUBSCRIBED_DATA_REPORT:
		report.SubscribedData = &ue.SubscribedData
	case models.AmfEventType_COMMUNICATION_FAILURE_REPORT:
		// TODO : report.CommFailure
	case models.AmfEventType_SUBSCRIPTION_ID_CHANGE:
		report.SubscriptionId = subscriptionId
	case models.AmfEventType_SUBSCRIPTION_ID_ADDITION:
		report.SubscriptionId = subscriptionId
	}
	return report, ok
}

func getDuration(expiry *time.Time, remainDuration *int32) bool {
	if expiry != nil {
		if time.Now().After(*expiry) {
			return false
		} else {
			duration := time.Until(*expiry)
			*remainDuration = int32(duration.Seconds())
		}
	}
	return true
}
