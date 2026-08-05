// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/ngap"
)

// libNASPDU wraps a NAS PDU relayed inside an NGAP message: the raw octets as
// hex, plus the decoded NAS tree.
func libNASPDU(pdu ngap.NASPDU) NASPDU {
	return NASPDU{
		Protocol: "NAS",
		RawHex:   hex.EncodeToString(pdu),
		Decoded:  nas.DecodeNASMessage(pdu),
	}
}

// Uplink NAS Transport relays a NAS message from the UE, with the location the
// NG-RAN node saw it at (TS 38.413 §9.2.5.3).
func buildUplinkNASTransport(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUplinkNASTransport(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Uplink NAS Transport: %v", err)}
	}

	ies := []IE{
		ie(idAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(idRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(idNASPDU, ngap.CriticalityReject, libNASPDU(m.NASPDU)),
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(idUserLocationInformation, ngap.CriticalityIgnore,
			userLocationInformation(*m.UserLocationInformation)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
