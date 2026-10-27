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
	ContentTypeJSON       = "application/json"
	ContentTypeProblem    = "application/problem+json"
	ContentTypeHAL        = "application/3gppHal+json"
	ContentTypeMergePatch = "application/merge-patch+json"
)

type Cause string

const (
	CauseInvalidAPI                   Cause = "INVALID_API"
	CauseInvalidMsgFormat             Cause = "INVALID_MSG_FORMAT"
	CauseInvalidQueryParam            Cause = "INVALID_QUERY_PARAM"
	CauseMandatoryQueryParamIncorrect Cause = "MANDATORY_QUERY_PARAM_INCORRECT"
	CauseOptionalQueryParamIncorrect  Cause = "OPTIONAL_QUERY_PARAM_INCORRECT"
	CauseMandatoryQueryParamMissing   Cause = "MANDATORY_QUERY_PARAM_MISSING"
	CauseMandatoryIEIncorrect         Cause = "MANDATORY_IE_INCORRECT"
	CauseOptionalIEIncorrect          Cause = "OPTIONAL_IE_INCORRECT"
	CauseMandatoryIEMissing           Cause = "MANDATORY_IE_MISSING"
	CauseUnspecifiedMsgFailure        Cause = "UNSPECIFIED_MSG_FAILURE"
	CauseResourceContextNotFound      Cause = "RESOURCE_CONTEXT_NOT_FOUND"
	CauseMsgLoopDetected              Cause = "MSG_LOOP_DETECTED"
	CauseRequestedPurposeNotAllowed   Cause = "REQUESTED_PURPOSE_NOT_ALLOWED"
	CauseOriginatingNetworkIDMismatch Cause = "ORIGINATING_NETWORK_ID_MISMATCH"
	CauseSubscriptionNotFound         Cause = "SUBSCRIPTION_NOT_FOUND"
	CauseResourceURIStructureNotFound Cause = "RESOURCE_URI_STRUCTURE_NOT_FOUND"
	CauseMaxJSONSizeExceeded          Cause = "MAX_JSON_SIZE_EXCEEDED"
	CauseNFCongestionRisk             Cause = "NF_CONGESTION_RISK"
	CauseNFServiceCongestionRisk      Cause = "NF_SERVICE_CONGESTION_RISK"
	CauseInsufficientResources        Cause = "INSUFFICIENT_RESOURCES"
	CauseUnspecifiedNFFailure         Cause = "UNSPECIFIED_NF_FAILURE"
	CauseSystemFailure                Cause = "SYSTEM_FAILURE"
	CauseNFFailover                   Cause = "NF_FAILOVER"
	CauseNFServiceFailover            Cause = "NF_SERVICE_FAILOVER"
	CauseNFCongestion                 Cause = "NF_CONGESTION"
	CauseNFServiceCongestion          Cause = "NF_SERVICE_CONGESTION"
	CauseTimedOutRequest              Cause = "TIMED_OUT_REQUEST"
	CauseTargetNFNotReachable         Cause = "TARGET_NF_NOT_REACHABLE"
	CauseTargetPLMNNotReachable       Cause = "TARGET_PLMN_NOT_REACHABLE"

	CauseServingNetworkNotAuthorized  Cause = "SERVING_NETWORK_NOT_AUTHORIZED"
	CauseAuthenticationRejected       Cause = "AUTHENTICATION_REJECTED"
	CauseInvalidHNPublicKeyIdentifier Cause = "INVALID_HN_PUBLIC_KEY_IDENTIFIER"
	CauseInvalidSchemeOutput          Cause = "INVALID_SCHEME_OUTPUT"
	CauseContextNotFound              Cause = "CONTEXT_NOT_FOUND"
	CauseUserNotFound                 Cause = "USER_NOT_FOUND"
	CauseUpstreamServerError          Cause = "UPSTREAM_SERVER_ERROR"
	CauseNetworkFailure               Cause = "NETWORK_FAILURE"
	CauseAVGenerationProblem          Cause = "AV_GENERATION_PROBLEM"
	CauseUnsupportedProtectionScheme  Cause = "UNSUPPORTED_PROTECTION_SCHEME"

	CauseDataNotFound                       Cause = "DATA_NOT_FOUND"
	CauseUnknown5GSSubscription             Cause = "UNKNOWN_5GS_SUBSCRIPTION"
	CauseNoPSSubscription                   Cause = "NO_PS_SUBSCRIPTION"
	CauseRoamingNotAllowed                  Cause = "ROAMING_NOT_ALLOWED"
	CauseRATNotAllowed                      Cause = "RAT_NOT_ALLOWED"
	CauseAccessNotAllowed                   Cause = "ACCESS_NOT_ALLOWED"
	CauseDNNNotAllowed                      Cause = "DNN_NOT_ALLOWED"
	CauseReauthenticationRequired           Cause = "REAUTHENTICATION_REQUIRED"
	CauseInvalidGUAMI                       Cause = "INVALID_GUAMI"
	CauseServiceNotAllowed                  Cause = "SERVICE_NOT_ALLOWED"
	CauseTemporaryRejectRegistrationOngoing Cause = "TEMPORARY_REJECT_REGISTRATION_ONGOING"
)

type InvalidParam struct {
	Param  string `json:"param"`
	Reason string `json:"reason,omitempty"`
}

func (ip InvalidParam) Check(Policy) error {
	return present("/param", ip.Param != "")
}

type ProblemDetails struct {
	Type                 string            `json:"type,omitempty"`
	Title                string            `json:"title,omitempty"`
	Status               int               `json:"status,omitempty"`
	Detail               string            `json:"detail,omitempty"`
	Instance             string            `json:"instance,omitempty"`
	Cause                Cause             `json:"cause,omitempty"`
	InvalidParams        []InvalidParam    `json:"invalidParams,omitempty"`
	SupportedFeatures    SupportedFeatures `json:"supportedFeatures,omitempty"`
	NrfID                string            `json:"nrfId,omitempty"`
	SupportedAPIVersions []string          `json:"supportedApiVersions,omitempty"`
}

func NewProblem(status int, cause Cause, detail string) ProblemDetails {
	return ProblemDetails{Title: http.StatusText(status), Status: status, Cause: cause, Detail: detail}
}

func (pd ProblemDetails) Error() string {
	var b strings.Builder

	fmt.Fprintf(&b, "problem %d", pd.Status)

	if pd.Cause != "" {
		b.WriteString(" " + string(pd.Cause))
	}

	if pd.Detail != "" {
		b.WriteString(": " + pd.Detail)
	} else if pd.Title != "" {
		b.WriteString(": " + pd.Title)
	}

	return b.String()
}

func (pd ProblemDetails) Check(p Policy) error {
	if p != Strict {
		return nil
	}

	if pd.Status != 0 {
		if err := checkRange("/status", pd.Status, 100, 599, true); err != nil {
			return err
		}
	}

	return firstError(
		Nest("/supportedFeatures", pd.SupportedFeatures.Check(p)),
		CheckAll("/invalidParams", pd.InvalidParams, p),
	)
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

type ReportItem struct {
	Path   *string `json:"path"`
	Reason string  `json:"reason,omitempty"`
}

func (r ReportItem) Check(Policy) error {
	return present("/path", r.Path != nil)
}

type PatchResult struct {
	Report []ReportItem `json:"report"`
}

func (r PatchResult) Check(p Policy) error {
	if len(r.Report) == 0 {
		return Missing("/report")
	}

	if p != Strict {
		return nil
	}

	return CheckAll("/report", r.Report, p)
}
