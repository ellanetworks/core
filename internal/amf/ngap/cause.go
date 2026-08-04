// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/ngap"

	"github.com/ellanetworks/core/internal/ngapcause"
	"github.com/free5gc/ngap/ngapType"
)

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
