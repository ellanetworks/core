package models

import (
	"net/http"
)

var (
	N1SmError = ProblemDetails{
		Title:  "Invalid N1 Message",
		Status: http.StatusForbidden,
		Detail: "N1 Message Error",
		Cause:  "N1_SM_ERROR",
	}
	DnnDeniedError = ProblemDetails{
		Title:         "DNN Denied",
		Status:        http.StatusForbidden,
		Detail:        "The subscriber does not have the necessary subscription to access the DNN",
		Cause:         "DNN_DENIED",
		InvalidParams: nil,
	}
	DnnNotSupported = ProblemDetails{
		Title:         "DNN Not Supported",
		Status:        http.StatusForbidden,
		Detail:        "The DNN is not supported by the SMF.",
		Cause:         "DNN_NOT_SUPPORTED",
		InvalidParams: nil,
	}
	InsufficientResourceSliceDnn = ProblemDetails{
		Title:         "DNN Resource insufficient",
		Status:        http.StatusInternalServerError,
		Detail:        "The request cannot be provided due to insufficient resources for the specific slice and DNN.",
		Cause:         "INSUFFICIENT_RESOURCES_SLICE_DNN",
		InvalidParams: nil,
	}
	SubscriptionDenied = ProblemDetails{
		Title:         "Subscription Denied",
		Status:        http.StatusForbidden,
		Detail:        "This indicates an error, other than those listed in this table, due to lack of necessary subscription to serve the UE request.",
		Cause:         "SUBSCRIPTION_DENIED",
		InvalidParams: nil,
	}
	NetworkFailure = ProblemDetails{
		Title:         "Network failure",
		Status:        http.StatusGatewayTimeout,
		Detail:        "The request is rejected due to a network problem.",
		Cause:         "NETWORK_FAILURE",
		InvalidParams: nil,
	}
)
