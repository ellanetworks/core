// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// Downlink NAS Transport relays a NAS message to the UE (TS 38.413 §9.2.5.1).
// Only this AMF sends it, and only with the three mandatory IEs; the optional
// ones §9.2.5.1 also allows render as preserved-unmodeled if a capture from
// another core carries them.
func buildDownlinkNASTransport(value []byte) NGAPMessageValue {
	m, err := ngap.ParseDownlinkNASTransport(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Downlink NAS Transport: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDNASPDU, ngap.CriticalityReject, libNASPDU(m.NASPDU)),
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
