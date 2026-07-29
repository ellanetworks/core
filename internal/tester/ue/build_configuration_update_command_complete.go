// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import "github.com/ellanetworks/core/nas/fgs"

func BuildConfigurationUpdateComplete() ([]byte, error) {
	return (&fgs.ConfigurationUpdateComplete{}).MarshalBinary()
}
