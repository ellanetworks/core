// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

func buildUEContextModificationRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUEContextModificationRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE Context Modification Request: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if ambr := m.UEAggregateMaximumBitRate; ambr != nil {
		ies = append(ies, ie(ngap.IDUEAggregateMaximumBitRate, ngap.CriticalityIgnore, UEAggregateMaximumBitRate{
			Downlink: int64(ambr.DL),
			Uplink:   int64(ambr.UL),
			Unit:     "bps",
		}))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildUEContextModificationResponse(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUEContextModificationResponse(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE Context Modification Response: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildUEContextModificationFailure(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUEContextModificationFailure(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE Context Modification Failure: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
