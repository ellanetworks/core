// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"testing"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/lpp/models"
	lmmodels "github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/per"
)

func encodeProvideLocationInformationR9(t *testing.T, r9 *lpptype.ProvideLocationInformationR9IEs) []byte {
	t.Helper()

	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			ProvideLocationInformation: &lpptype.ProvideLocationInformation{
				CriticalExtensions: lpptype.ProvideLocationInformationCriticalExtensions{
					C1: &lpptype.ProvideLocationInformationCriticalExtensionsC1{
						ProvideLocationInformationR9: r9,
					},
				},
			},
		},
	}

	b, err := encodeLPPMessage(0x02, lpptype.InitiatorTargetDevice, body, true, 0)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func encodeProvideLocationInformationWithoutEstimate(t *testing.T) []byte {
	t.Helper()

	return encodeProvideLocationInformationR9(t, &lpptype.ProvideLocationInformationR9IEs{
		CommonIEsProvideLocationInformation: &lpptype.CommonIEsProvideLocationInformation{
			LocationError: &lpptype.LocationError{LocationFailureCause: 1},
		},
	})
}

func ellipseEstimate() *lpptype.LocationCoordinates {
	latSign, latAbs := encodeLatitude(450000000)

	return &lpptype.LocationCoordinates{
		EllipsoidPointWithUncertaintyEllipse: &lpptype.EllipsoidPointWithUncertaintyEllipse{
			LatitudeSign:         latSign,
			DegreesLatitude:      latAbs,
			DegreesLongitude:     encodeLongitude(214500000),
			UncertaintySemiMajor: encodeUncertainty(20),
			UncertaintySemiMinor: encodeUncertainty(10),
			Confidence:           68,
		},
	}
}

func TestDecodeProvideLocationInformationWithoutEstimate(t *testing.T) {
	decoded, err := DecodeLPPMessage(encodeProvideLocationInformationWithoutEstimate(t))
	if err != nil {
		t.Fatal(err)
	}

	pli := decoded.ProvideLocationInformation
	if pli == nil {
		t.Fatal("ProvideLocationInformation not decoded")
	}

	if pli.HasLocationEstimate {
		t.Error("HasLocationEstimate = true, want false")
	}

	if !pli.LocationError {
		t.Error("LocationError = false, want true")
	}
}

func TestDecodeProvideLocationInformationWithEstimate(t *testing.T) {
	b, err := EncodeProvideLocationInformation(0x02, 450000000, 214500000, 10000, 10, 15)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeLPPMessage(b)
	if err != nil {
		t.Fatal(err)
	}

	if !decoded.ProvideLocationInformation.HasLocationEstimate || decoded.ProvideLocationInformation.LocationError {
		t.Errorf("ProvideLocationInformation = %+v, want an estimate and no error", decoded.ProvideLocationInformation)
	}
}

func TestSessionRejectsLocationWithoutEstimate(t *testing.T) {
	s := NewSession("imsi-001010000000001", "session-1", "agnss")
	s.state = LocationRequested

	completed := false

	s.SetTransport(nil, func(*lmmodels.LocationResult) error {
		completed = true
		return nil
	}, nil, nil, nil)

	err := s.HandleResponse(&models.ProvideLocationInformation{LocationError: true})
	if err == nil {
		t.Fatal("HandleResponse accepted a ProvideLocationInformation with no estimate")
	}

	if s.State() == LocationReceived || completed {
		t.Error("session completed without a location estimate")
	}
}

func TestDecodeProvideLocationInformationEllipse(t *testing.T) {
	decoded, err := DecodeLPPMessage(encodeProvideLocationInformationR9(t, &lpptype.ProvideLocationInformationR9IEs{
		CommonIEsProvideLocationInformation: &lpptype.CommonIEsProvideLocationInformation{LocationEstimate: ellipseEstimate()},
	}))
	if err != nil {
		t.Fatal(err)
	}

	pli := decoded.ProvideLocationInformation
	if !pli.HasLocationEstimate {
		t.Fatal("HasLocationEstimate = false for an ellipsoid point with uncertainty ellipse")
	}

	if abs(int(pli.GNSSPositionResult.Latitude)-450000000) > 1000000 || abs(int(pli.GNSSPositionResult.Longitude)-214500000) > 1000000 {
		t.Errorf("position = %d, %d, want ~450000000, ~214500000", pli.GNSSPositionResult.Latitude, pli.GNSSPositionResult.Longitude)
	}

	if pli.GNSSPositionResult.HorizontalAccuracy < 15 {
		t.Errorf("horizontal accuracy = %d, want the semi-major axis (~20 m)", pli.GNSSPositionResult.HorizontalAccuracy)
	}
}

func TestDecodeProvideLocationInformationEstimateWithGNSSError(t *testing.T) {
	decoded, err := DecodeLPPMessage(encodeProvideLocationInformationR9(t, &lpptype.ProvideLocationInformationR9IEs{
		CommonIEsProvideLocationInformation: &lpptype.CommonIEsProvideLocationInformation{LocationEstimate: ellipseEstimate()},
		AGNSSProvideLocationInformation: &lpptype.AGNSSProvideLocationInformation{
			GnssError: &lpptype.GNSSError{
				TargetDeviceErrorCauses: &lpptype.GNSSTargetDeviceErrorCauses{
					Cause:                      lpptype.GNSSTargetDeviceErrorCausesNotAllRequestedMeasurementsPossible,
					AdrMeasurementsNotPossible: &per.Null{},
				},
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	pli := decoded.ProvideLocationInformation
	if !pli.HasLocationEstimate || !pli.LocationError {
		t.Fatalf("ProvideLocationInformation = %+v, want an estimate and a GNSS error", pli)
	}

	s := NewSession("imsi-001010000000001", "session-2", "agnss")
	s.state = LocationRequested

	if err := s.HandleResponse(pli); err != nil {
		t.Fatalf("HandleResponse rejected an estimate reported with notAllRequestedMeasurementsPossible: %v", err)
	}

	if s.State() != LocationReceived {
		t.Errorf("state = %s, want %s", s.State(), LocationReceived)
	}
}
