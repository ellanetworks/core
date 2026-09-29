// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"errors"
	"fmt"
)

var (
	ErrUENoLocationEstimate     = errors.New("UE provided no location estimate")
	ErrUnsupportedLocationShape = errors.New("UE location estimate uses a shape that cannot be reported as a point")
	ErrUEAborted                = errors.New("UE aborted the LPP procedure")
	ErrUEReportedError          = errors.New("UE reported an LPP error")
	ErrUndecodableMessage       = errors.New("UE sent an LPP message that could not be decoded")
	ErrUEECIDNotSupported       = errors.New("UE does not support E-CID measurements")
	ErrUENoMeasurements         = errors.New("UE provided no E-CID measurements")
)

var locationFailureCauseNames = map[int64]string{
	0: "undefined",
	1: "requestedMethodNotSupported",
	2: "positionMethodFailure",
	3: "periodicLocationMeasurementsNotAvailable",
}

var gnssErrorCauseNames = map[int64]string{
	0: "undefined",
	1: "thereWereNotEnoughSatellitesReceived",
	2: "assistanceDataMissing",
	3: "notAllRequestedMeasurementsPossible",
}

var ecidErrorCauseNames = map[int64]string{
	0: "undefined",
	1: "requestedMeasurementNotAvailable",
	2: "notAllrequestedMeasurementsPossible",
}

var abortCauseNames = map[int64]string{
	0: "undefined",
	1: "stopPeriodicReporting",
	2: "targetDeviceAbort",
	3: "networkAbort",
	4: "stopPeriodicAssistanceDataDelivery",
}

var errorCauseNames = map[int64]string{
	0: "undefined",
	1: "lppMessageHeaderError",
	2: "lppMessageBodyError",
	3: "epduError",
	4: "incorrectDataValue",
	5: "lppSegmentationError",
}

func causeName(names map[int64]string, cause *int64) string {
	if cause == nil {
		return "absent"
	}

	if name, ok := names[*cause]; ok {
		return name
	}

	return fmt.Sprintf("unknown (%d)", *cause)
}
