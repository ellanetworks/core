// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/ngapcause"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func getCause(cause *ngapType.Cause) (int, aper.Enumerated, error) {
	switch cause.Present {
	case ngapType.CausePresentRadioNetwork:
		return cause.Present, cause.RadioNetwork.Value, nil
	case ngapType.CausePresentTransport:
		return cause.Present, cause.Transport.Value, nil
	case ngapType.CausePresentProtocol:
		return cause.Present, cause.Protocol.Value, nil
	case ngapType.CausePresentNas:
		return cause.Present, cause.Nas.Value, nil
	case ngapType.CausePresentMisc:
		return cause.Present, cause.Misc.Value, nil
	default:
		return cause.Present, 0, fmt.Errorf("invalid Cause group: %d", cause.Present)
	}
}

// causeToString renders an NGAP cause for logging (TS 38.413 §9.3.1.2).
func causeToString(cause ngapType.Cause) string {
	return ngapcause.CauseToString(cause)
}
