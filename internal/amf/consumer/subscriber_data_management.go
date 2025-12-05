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

func SDMGetAmData(ctx ctxt.Context, ue *context.AmfUe) error {
	data, err := udm.GetAmData(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get AM data from UDM: %v", err)
	}

	ue.AccessAndMobilitySubscriptionData = data

	return nil
}

func SDMGetSmfSelectData(ctx ctxt.Context, ue *context.AmfUe) error {
	dnn, err := udm.GetDNN(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get DNN from UDM for subscriber %s: %v", ue.Supi, err)
	}

	ue.Dnn = dnn

	return nil
}

func SDMGetUeContextInSmfData(ctx ctxt.Context, ue *context.AmfUe) (err error) {
	data, err := udm.GetUeContextInSmfData(ctx, ue.Supi)
	if err != nil {
		return err
	}

	ue.UeContextInSmfData = data

	return nil
}

func SDMGetSliceSelectionSubscriptionData(ctx ctxt.Context, ue *context.AmfUe) error {
	snssai, err := udm.GetSnssai(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve snssai from UDM: %s", err.Error())
	}

	ue.SubscribedNssai = snssai

	return nil
}
