// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// Error Indication reports a failure that has no dedicated response message
// (TS 38.413 §8.7.5). Every IE is optional: the UE-NGAP-IDs are present only
// when the failure is associated with a UE, and either Cause or Criticality
// Diagnostics — or both — carry the reason.
func buildErrorIndication(value []byte) NGAPMessageValue {
	ind, err := ngap.ParseErrorIndication(value)
	if err != nil {
		return NGAPMessageValue{Error: err.Error()}
	}

	ies := make([]IE, 0, 5)

	if ind.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*ind.AMFUENGAPID)))
	}

	if ind.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*ind.RANUENGAPID)))
	}

	if ind.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*ind.Cause)))
	}

	if ind.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore,
			criticalityDiagnostics(*ind.CriticalityDiagnostics)))
	}

	if ind.FiveGSTMSI != nil {
		ies = append(ies, ie(ngap.IDFiveGSTMSI, ngap.CriticalityIgnore, buildFiveGSTMSI(*ind.FiveGSTMSI)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(ind.UnknownIEs())...)}
}

func buildFiveGSTMSI(s ngap.FiveGSTMSI) FiveGSTMSI {
	return FiveGSTMSI{
		AMFSetID:   bitsHex(uint64(s.AMFSetID), 10),
		AMFPointer: bitsHex(uint64(s.AMFPointer), 6),
		FiveGTMSI:  fmt.Sprintf("%08x", uint32(s.FiveGTMSI)),
	}
}
