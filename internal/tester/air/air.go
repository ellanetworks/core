// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package air

import (
	"github.com/ellanetworks/core/ngap"
)

type DownlinkSender interface {
	SendDownlinkNAS(nasPDU []byte, amfUENGAPID int64, ranUENGAPID int64) error
	RRCRelease()
}

type UplinkSender interface {
	SendUplinkNAS(nasPDU []byte, amfUENGAPID int64, ranUENGAPID int64) error
	SendInitialUEMessage(nasPDU []byte, ranUENGAPID int64, guti5G []byte, cause ngap.RRCEstablishmentCause) error
}
