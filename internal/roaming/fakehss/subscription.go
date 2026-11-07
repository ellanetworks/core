// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fakehss

import (
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func APN(contextID uint32, name string) s6a.APNConfiguration {
	return s6a.APNConfiguration{
		ContextIdentifier:          contextID,
		PDNType:                    s6a.PDNTypeIPv4,
		ServiceSelection:           name,
		QoS:                        &s6a.EPSSubscribedQoSProfile{QCI: 9, ARP: tgpp.DefaultAllocationRetentionPriority(8)},
		VPLMNDynamicAddressAllowed: true,
		AMBR:                       &s6a.AMBR{UL: 50_000_000, DL: 150_000_000},
	}
}

func Subscription(defaultContext uint32, apns ...s6a.APNConfiguration) s6a.SubscriptionData {
	granted := s6a.SubscriberStatusServiceGranted

	return s6a.SubscriptionData{
		SubscriberStatus: &granted,
		AMBR:             &s6a.AMBR{UL: 80_000_000, DL: 300_000_000},
		APNConfigurationProfile: &s6a.APNConfigurationProfile{
			ContextIdentifier: defaultContext,
			Included:          s6a.AllAPNConfigurationsIncluded,
			Configurations:    apns,
		},
	}
}
