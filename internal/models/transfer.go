// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "errors"

var (
	ErrSessionNotTransferable = errors.New("session does not exist on the other access")

	ErrSessionOnOtherDNN = errors.New("session is on another data network")

	ErrSessionNotMovable = errors.New("session cannot move as described")
)
