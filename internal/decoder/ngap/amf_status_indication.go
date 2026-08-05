// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// AMF Status Indication tells the NG-RAN node which GUAMIs this AMF can no
// longer serve (TS 38.413 §8.7.6). TS 36.413 defines no counterpart.
func buildAMFStatusIndication(value []byte) NGAPMessageValue {
	m, err := ngap.ParseAMFStatusIndication(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse AMF Status Indication: %v", err)}
	}

	guamis := make([]Guami, 0, len(m.UnavailableGUAMIList))
	for _, item := range m.UnavailableGUAMIList {
		guamis = append(guamis, guami(item.GUAMI))
	}

	ies := []IE{ie(idUnavailableGUAMIList, ngap.CriticalityReject, guamis)}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
