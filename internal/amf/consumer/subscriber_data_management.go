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
)

func GetAndSetSubscriberData(ctx ctxt.Context, ue *context.AmfUe) error {
	bitRate, dnn, err := context.GetSubscriberData(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get subscriber data: %v", err)
	}

	ue.Dnn = dnn
	ue.Ambr = bitRate

	return nil
}

func GetAndSetSubscribedNSSAI(ctx ctxt.Context, ue *context.AmfUe) error {
	plmn, err := context.GetSupportedPlmn(ctx)
	if err != nil {
		return fmt.Errorf("failed to get plmn: %s", err.Error())
	}

	ue.SubscribedNssai = &plmn.SNssai

	return nil
}
