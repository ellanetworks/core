// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import (
	"errors"

	"github.com/ellanetworks/core/per"
)

var ErrUnmodelledExtension = errors.New("lpptype: extension addition is not modelled")

type UnmodelledExtension struct{}

func (*UnmodelledExtension) MarshalPER(*per.Writer, per.Encoding) error {
	return ErrUnmodelledExtension
}

func (*UnmodelledExtension) UnmarshalPER(*per.Reader, per.Encoding) error {
	return nil
}
