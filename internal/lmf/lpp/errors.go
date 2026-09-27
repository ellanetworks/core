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

func causeName(names map[int64]string, cause *int64) string {
	if cause == nil {
		return "absent"
	}

	if name, ok := names[*cause]; ok {
		return name
	}

	return fmt.Sprintf("unknown (%d)", *cause)
}
