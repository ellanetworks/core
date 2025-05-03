// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"context"

	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel/attribute"
)

// TS 29.503 5.3.2.2.2
func EditRegistrationAmf3gppAccess(registerRequest models.Amf3GppAccessRegistration, ueID string, ctx context.Context) error {
	_, span := tracer.Start(ctx, "EditRegistrationAmf3gppAccess")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.ID", ueID),
	)
	udmContext.CreateAmf3gppRegContext(ueID, registerRequest)
	return nil
}
