// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/udm"
)

func GetAndSetSubscriberData(ctx ctxt.Context, ue *context.AmfUe) error {
	subData, err := udm.GetSubscriberData(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get subscriber data from UDM: %v", err)
	}

	ue.Dnn = subData.Dnn
	ue.AccessAndMobilitySubscriptionData = subData.AccessAndMobilitySubscriptionData

	return nil
}

func GetAndSetSubscribedNSSAI(ctx ctxt.Context, ue *context.AmfUe) error {
	snssai, err := udm.GetSnssai(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve snssai from UDM: %s", err.Error())
	}

	ue.SubscribedNssai = snssai

	return nil
}
