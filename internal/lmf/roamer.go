// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

var ErrRoamer = errors.New("roamers cannot be located")

func (l *LMF) refuseRoamer(supi etsi.SUPI) error {
	if (l.amf != nil && l.amf.ServesRoamer(supi)) || (l.mme != nil && l.mme.ServesRoamer(supi)) {
		return fmt.Errorf("%w: %s", ErrRoamer, supi)
	}

	return nil
}
