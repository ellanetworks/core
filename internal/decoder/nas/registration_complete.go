// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "github.com/ellanetworks/core/nas/fgs"

type RegistrationComplete struct {
	GetSORContent []uint8 `json:"sor_transparent_container,omitempty"`
}

func buildRegistrationComplete(msg *fgs.RegistrationComplete) *RegistrationComplete {
	return &RegistrationComplete{
		GetSORContent: msg.SORTransparentContainer,
	}
}
