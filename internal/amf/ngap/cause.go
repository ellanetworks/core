// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"
	"github.com/ellanetworks/core/ngap"

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

func causeToString(cause ngapType.Cause) string {
	return ngapcause.CauseToString(cause)
}

// libCause converts a Cause the reference decoder produced into the library's,
// for the send paths those procedures still feed. It goes when the last of them
// is migrated.
func libCause(cause *ngapType.Cause) ngap.Cause {
	group := ngap.CauseGroupRadioNetwork
	value := ngap.CauseRadioNetworkUnspecified

	switch cause.Present {
	case ngapType.CausePresentRadioNetwork:
		value = int(cause.RadioNetwork.Value)
	case ngapType.CausePresentTransport:
		group, value = ngap.CauseGroupTransport, int(cause.Transport.Value)
	case ngapType.CausePresentNas:
		group, value = ngap.CauseGroupNAS, int(cause.Nas.Value)
	case ngapType.CausePresentProtocol:
		group, value = ngap.CauseGroupProtocol, int(cause.Protocol.Value)
	case ngapType.CausePresentMisc:
		group, value = ngap.CauseGroupMisc, int(cause.Misc.Value)
	}

	return ngap.Cause{Group: group, Value: value}
}
