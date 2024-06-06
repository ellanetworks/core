/*
 * NSSF NSSAI Availability
 *
 * NSSF NSSAI Availability Service
 */

package producer

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/nssf/factory"
	"github.com/yeastengine/ella/internal/nssf/logger"
	"github.com/yeastengine/ella/internal/nssf/util"
)

// Get available subscription ID from configuration
// In this implementation, string converted from 32-bit integer is used as subscription ID
func getUnusedSubscriptionID() (string, error) {
	var idx uint32 = 1
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, subscription := range factory.NssfConfig.Subscriptions {
		tempID, err := strconv.Atoi(subscription.SubscriptionId)
		if err != nil {
			return "", err
		}
		if uint32(tempID) == idx {
			if idx == math.MaxUint32 {
				return "", fmt.Errorf("No available subscription ID")
			}
			idx = idx + 1
		} else {
			break
		}
	}
	return strconv.Itoa(int(idx)), nil
}

// NSSAIAvailability subscription POST method
func NSSAIAvailabilityPostProcedure(createData models.NssfEventSubscriptionCreateData) (
	*models.NssfEventSubscriptionCreatedData, *models.ProblemDetails,
) {
	var (
		response       *models.NssfEventSubscriptionCreatedData = &models.NssfEventSubscriptionCreatedData{}
		problemDetails *models.ProblemDetails
	)

	var subscription factory.Subscription
	tempID, err := getUnusedSubscriptionID()
	if err != nil {
		logger.Nssaiavailability.Warnf(err.Error())

		problemDetails = &models.ProblemDetails{
			Title:  util.UNSUPPORTED_RESOURCE,
			Status: http.StatusNotFound,
			Detail: err.Error(),
		}
		return nil, problemDetails
	}

	subscription.SubscriptionId = tempID
	subscription.SubscriptionData = new(models.NssfEventSubscriptionCreateData)
	*subscription.SubscriptionData = createData

	factory.NssfConfig.Subscriptions = append(factory.NssfConfig.Subscriptions, subscription)

	response.SubscriptionId = subscription.SubscriptionId
	if !subscription.SubscriptionData.Expiry.IsZero() {
		response.Expiry = new(time.Time)
		*response.Expiry = *subscription.SubscriptionData.Expiry
	}
	response.AuthorizedNssaiAvailabilityData = util.AuthorizeOfTaListFromConfig(subscription.SubscriptionData.TaiList)

	return response, nil
}

func NSSAIAvailabilityUnsubscribeProcedure(subscriptionId string) *models.ProblemDetails {
	var problemDetails *models.ProblemDetails

	factory.ConfigLock.Lock()
	defer factory.ConfigLock.Unlock()
	for i, subscription := range factory.NssfConfig.Subscriptions {
		if subscription.SubscriptionId == subscriptionId {
			factory.NssfConfig.Subscriptions = append(factory.NssfConfig.Subscriptions[:i],
				factory.NssfConfig.Subscriptions[i+1:]...)

			return nil
		}
	}

	// No specific subscription ID exists
	problemDetails = &models.ProblemDetails{
		Title:  util.UNSUPPORTED_RESOURCE,
		Status: http.StatusNotFound,
		Detail: fmt.Sprintf("Subscription ID '%s' is not available", subscriptionId),
	}
	return problemDetails
}
