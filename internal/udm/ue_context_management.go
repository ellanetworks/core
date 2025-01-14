// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"github.com/omec-project/openapi/models"
)

// EditRegistrationAmf3gppAccess
// TS 29.503 5.3.2.2.2
func EditRegistrationAmf3gppAccess(registerRequest models.Amf3GppAccessRegistration, ueID string) error {
	udmContext.CreateAmf3gppRegContext(ueID, registerRequest)
	return nil
}
