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
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

func UeCmRegistration(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType, initialRegistrationInd bool) error {
	switch accessType {
	case models.AccessType3GPPAccess:
		logger.AmfLog.Warn("UE CM Registration for 3GPP Access is not implemented yet")
	case models.AccessTypeNon3GPPAccess:
		return fmt.Errorf("Non-3GPP access is not supported")
	}
	return nil
}
