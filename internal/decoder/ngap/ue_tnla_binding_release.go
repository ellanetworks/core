// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

func buildUETNLABindingReleaseRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUETNLABindingReleaseRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE TNLA Binding Release Request: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
