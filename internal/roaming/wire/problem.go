// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	ContentTypeJSON    = "application/json"
	ContentTypeProblem = "application/problem+json"
	ContentTypeHAL     = "application/3gppHal+json"
)

const (
	CauseInvalidAPI                   = "INVALID_API"
	CauseInvalidMsgFormat             = "INVALID_MSG_FORMAT"
	CauseInvalidQueryParam            = "INVALID_QUERY_PARAM"
	CauseMandatoryQueryParamMissing   = "MANDATORY_QUERY_PARAM_MISSING"
	CauseMandatoryIEIncorrect         = "MANDATORY_IE_INCORRECT"
	CauseOptionalIEIncorrect          = "OPTIONAL_IE_INCORRECT"
	CauseMandatoryIEMissing           = "MANDATORY_IE_MISSING"
	CauseUnspecifiedMsgFailure        = "UNSPECIFIED_MSG_FAILURE"
	CauseMsgLoopDetected              = "MSG_LOOP_DETECTED"
	CauseRequestedPurposeNotAllowed   = "REQUESTED_PURPOSE_NOT_ALLOWED"
	CauseOriginatingNetworkIDMismatch = "ORIGINATING_NETWORK_ID_MISMATCH"
	CauseResourceURIStructureNotFound = "RESOURCE_URI_STRUCTURE_NOT_FOUND"
	CauseContextNotFound              = "CONTEXT_NOT_FOUND"
	CauseMaxJSONSizeExceeded          = "MAX_JSON_SIZE_EXCEEDED"
	CauseNFCongestionRisk             = "NF_CONGESTION_RISK"
	CauseInsufficientResources        = "INSUFFICIENT_RESOURCES"
	CauseUnspecifiedNFFailure         = "UNSPECIFIED_NF_FAILURE"
	CauseSystemFailure                = "SYSTEM_FAILURE"
	CauseNFCongestion                 = "NF_CONGESTION"
	CauseTimedOutRequest              = "TIMED_OUT_REQUEST"
	CauseTargetNFNotReachable         = "TARGET_NF_NOT_REACHABLE"
	CauseTargetPLMNNotReachable       = "TARGET_PLMN_NOT_REACHABLE"

	CauseUserNotFound                 = "USER_NOT_FOUND"
	CauseAuthenticationRejected       = "AUTHENTICATION_REJECTED"
	CauseServingNetworkNotAuthorized  = "SERVING_NETWORK_NOT_AUTHORIZED"
	CauseInvalidHNPublicKeyIdentifier = "INVALID_HN_PUBLIC_KEY_IDENTIFIER"
	CauseUnsupportedProtectionScheme  = "UNSUPPORTED_PROTECTION_SCHEME"
	CauseAVGenerationProblem          = "AV_GENERATION_PROBLEM"

	CauseDataNotFound           = "DATA_NOT_FOUND"
	CauseUnknown5GSSubscription = "UNKNOWN_5GS_SUBSCRIPTION"
	CauseNoPSSubscription       = "NO_PS_SUBSCRIPTION"
	CauseRoamingNotAllowed      = "ROAMING_NOT_ALLOWED"
	CauseRATNotAllowed          = "RAT_NOT_ALLOWED"
	CauseAccessNotAllowed       = "ACCESS_NOT_ALLOWED"
	CauseDNNNotAllowed          = "DNN_NOT_ALLOWED"
)

type InvalidParam struct {
	Param  string `json:"param"`
	Reason string `json:"reason,omitempty"`
}

func (ip InvalidParam) Check(Policy) error {
	if ip.Param == "" {
		return missing("/param")
	}

	return nil
}

type ProblemDetails struct {
	Type                 string            `json:"type,omitempty"`
	Title                string            `json:"title,omitempty"`
	Status               int               `json:"status,omitempty"`
	Detail               string            `json:"detail,omitempty"`
	Instance             string            `json:"instance,omitempty"`
	Cause                string            `json:"cause,omitempty"`
	InvalidParams        []InvalidParam    `json:"invalidParams,omitempty"`
	SupportedFeatures    SupportedFeatures `json:"supportedFeatures,omitempty"`
	NrfID                string            `json:"nrfId,omitempty"`
	SupportedAPIVersions []string          `json:"supportedApiVersions,omitempty"`
}

func NewProblem(status int, cause string, detail string) ProblemDetails {
	return ProblemDetails{Title: http.StatusText(status), Status: status, Cause: cause, Detail: detail}
}

func (pd ProblemDetails) Error() string {
	var b strings.Builder

	fmt.Fprintf(&b, "problem %d", pd.Status)

	if pd.Cause != "" {
		b.WriteString(" " + pd.Cause)
	}

	if pd.Detail != "" {
		b.WriteString(": " + pd.Detail)
	} else if pd.Title != "" {
		b.WriteString(": " + pd.Title)
	}

	return b.String()
}

func (pd ProblemDetails) Check(p Policy) error {
	if p == Strict {
		if pd.Status != 0 {
			if err := checkRange("/status", pd.Status, 100, 599, true); err != nil {
				return err
			}
		}

		if err := pd.SupportedFeatures.Check(p); err != nil {
			return Nest("/supportedFeatures", err)
		}
	}

	return checkAll("/invalidParams", pd.InvalidParams, p)
}

func ParseProblem(status int, body []byte) ProblemDetails {
	var pd ProblemDetails
	if len(body) > 0 && json.Unmarshal(body, &pd) != nil {
		pd = ProblemDetails{}
	}

	pd.Status = status

	return pd
}

func ProblemFromError(err error) ProblemDetails {
	var pd ProblemDetails
	if errors.As(err, &pd) {
		return pd
	}

	var ie *IEError
	if errors.As(err, &ie) {
		p := NewProblem(http.StatusBadRequest, ie.Cause(), ie.Error())

		reason := ie.Reason
		if ie.Missing {
			reason = "missing"
		}

		p.InvalidParams = []InvalidParam{{Param: ie.Param, Reason: reason}}

		return p
	}

	if errors.Is(err, ErrMalformed) {
		return NewProblem(http.StatusBadRequest, CauseInvalidMsgFormat, err.Error())
	}

	return NewProblem(http.StatusInternalServerError, CauseSystemFailure, "")
}
