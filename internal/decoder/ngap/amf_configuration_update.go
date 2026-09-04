// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

func buildAMFConfigurationUpdate(value []byte) NGAPMessageValue {
	m, err := ngap.ParseAMFConfigurationUpdate(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse AMF Configuration Update: %v", err)}
	}

	var ies []IE

	if m.AMFName != nil {
		ies = append(ies, ie(ngap.IDAMFName, ngap.CriticalityReject, *m.AMFName))
	}

	if len(m.ServedGUAMIList) > 0 {
		ies = append(ies, ie(ngap.IDServedGUAMIList, ngap.CriticalityReject, buildServedGUAMIList(m.ServedGUAMIList)))
	}

	if m.RelativeAMFCapacity != nil {
		ies = append(ies, ie(ngap.IDRelativeAMFCapacity, ngap.CriticalityIgnore, *m.RelativeAMFCapacity))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildAMFConfigurationUpdateAcknowledge(value []byte) NGAPMessageValue {
	m, err := ngap.ParseAMFConfigurationUpdateAcknowledge(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse AMF Configuration Update Acknowledge: %v", err)}
	}

	var ies []IE

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildAMFConfigurationUpdateFailure(value []byte) NGAPMessageValue {
	m, err := ngap.ParseAMFConfigurationUpdateFailure(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse AMF Configuration Update Failure: %v", err)}
	}

	var ies []IE

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.TimeToWait != nil {
		ies = append(ies, ie(ngap.IDTimeToWait, ngap.CriticalityIgnore, buildTimeToWait(*m.TimeToWait)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
