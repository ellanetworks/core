// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"errors"
	"time"
)

var ErrDatapathCapacity = errors.New("datapath session capacity reached")

const DatapathCapacityBackoff = 60 * time.Second
