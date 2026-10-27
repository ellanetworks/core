// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"net/url"
	"strconv"
	"strings"
)

type ServiceName string

const (
	ServiceNnrfDisc  ServiceName = "nnrf-disc"
	ServiceNausfAuth ServiceName = "nausf-auth"
	ServiceNudmSDM   ServiceName = "nudm-sdm"
	ServiceNudmUECM  ServiceName = "nudm-uecm"
)

func (s ServiceName) Version() string {
	switch s {
	case ServiceNudmSDM:
		return "v2"
	case ServiceNnrfDisc, ServiceNausfAuth, ServiceNudmUECM:
		return "v1"
	default:
		return ""
	}
}

func (s ServiceName) path(segments ...string) string {
	var b strings.Builder

	b.WriteString("/" + string(s) + "/" + s.Version())

	for _, seg := range segments {
		b.WriteString("/" + url.PathEscape(seg))
	}

	return b.String()
}

func NFInstancesPath() string {
	return ServiceNnrfDisc.path("nf-instances")
}

func UEAuthenticationsPath() string {
	return ServiceNausfAuth.path("ue-authentications")
}

func SDMDataSetsPath(supi string) string {
	return ServiceNudmSDM.path(supi)
}

func SDMAmDataPath(supi string) string {
	return ServiceNudmSDM.path(supi, "am-data")
}

func SDMSorAckPath(supi string) string {
	return ServiceNudmSDM.path(supi, "am-data", "sor-ack")
}

func SDMSmfSelectDataPath(supi string) string {
	return ServiceNudmSDM.path(supi, "smf-select-data")
}

func SDMSmDataPath(supi string) string {
	return ServiceNudmSDM.path(supi, "sm-data")
}

func SDMSubscriptionsPath(supi string) string {
	return ServiceNudmSDM.path(supi, "sdm-subscriptions")
}

func SDMSubscriptionPath(supi, subscriptionID string) string {
	return ServiceNudmSDM.path(supi, "sdm-subscriptions", subscriptionID)
}

func UECMAmf3GppAccessPath(supi string) string {
	return ServiceNudmUECM.path(supi, "registrations", "amf-3gpp-access")
}

func UECMSmfRegistrationPath(supi string, pduSessionID int) string {
	return ServiceNudmUECM.path(supi, "registrations", "smf-registrations", strconv.Itoa(pduSessionID))
}
