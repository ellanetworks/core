// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/lpp/models"
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

	if pli.LocationEstimate != nil {
		t.Errorf("LocationEstimate = %+v, want none", pli.LocationEstimate)
	}

	if pli.LocationFailureCause == nil || *pli.LocationFailureCause != 1 {
		t.Errorf("LocationFailureCause = %v, want 1", pli.LocationFailureCause)
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

	if decoded.ProvideLocationInformation.LocationEstimate == nil || decoded.ProvideLocationInformation.LocationFailureCause != nil || decoded.ProvideLocationInformation.GNSSErrorCause != nil {
		t.Errorf("ProvideLocationInformation = %+v, want an estimate and no error", decoded.ProvideLocationInformation)
	}
}

func TestSessionRejectsLocationWithoutEstimate(t *testing.T) {
	s := NewSession("imsi-001010000000001", "session-1", "agnss")
	s.state = LocationRequested

	cause := int64(2)

	err := s.HandleResponse(&models.ProvideLocationInformation{LocationFailureCause: &cause})
	if !errors.Is(err, ErrUENoLocationEstimate) {
		t.Fatalf("HandleResponse = %v, want ErrUENoLocationEstimate", err)
	}

	if !strings.Contains(err.Error(), "positionMethodFailure") {
		t.Errorf("error %q does not name the location failure cause", err)
	}

	if s.State() == LocationReceived || s.LocationResult() != nil {
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

	e := decoded.ProvideLocationInformation.LocationEstimate
	if e == nil {
		t.Fatal("no estimate for an ellipsoid point with uncertainty ellipse")
	}

	if math.Abs(e.LatitudeDegrees-45) > 1e-4 || math.Abs(e.LongitudeDegrees-21.45) > 1e-4 {
		t.Errorf("position = %f, %f, want ~45, ~21.45", e.LatitudeDegrees, e.LongitudeDegrees)
	}

	if e.UncertaintyEllipse == nil || math.Abs(e.UncertaintyEllipse.SemiMajorMeters-20) > 2 || math.Abs(e.UncertaintyEllipse.SemiMinorMeters-10) > 1.5 {
		t.Errorf("uncertainty ellipse = %+v, want ~20 m by ~10 m", e.UncertaintyEllipse)
	}

	if e.ConfidencePercent == nil || *e.ConfidencePercent != 68 {
		t.Errorf("confidence = %v, want 68", e.ConfidencePercent)
	}

	if e.AltitudeMeters != nil {
		t.Errorf("altitude = %v, want none for a 2D ellipse", *e.AltitudeMeters)
	}
}

func TestDecodeProvideLocationInformationEstimateWithGNSSError(t *testing.T) {
	decoded, err := DecodeLPPMessage(encodeProvideLocationInformationR9(t, &lpptype.ProvideLocationInformationR9IEs{
		CommonIEsProvideLocationInformation: &lpptype.CommonIEsProvideLocationInformation{LocationEstimate: ellipseEstimate()},
		AGNSSProvideLocationInformation: &lpptype.AGNSSProvideLocationInformation{
			GNSSError: &lpptype.AGNSSError{
				TargetDeviceErrorCauses: &lpptype.GNSSTargetDeviceErrorCauses{
					Cause:                      lpptype.GNSSTargetDeviceErrorCauseNotAllRequestedMeasurementsPossible,
					ADRMeasurementsNotPossible: &per.Null{},
				},
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	pli := decoded.ProvideLocationInformation
	if pli.LocationEstimate == nil || pli.GNSSErrorCause == nil || *pli.GNSSErrorCause != int64(lpptype.GNSSTargetDeviceErrorCauseNotAllRequestedMeasurementsPossible) {
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

func TestSessionFailWithRecordsTheFirstFailure(t *testing.T) {
	s := NewSession("imsi-001010000000001", "session-3", "agnss")
	s.state = LocationRequested

	first := errors.New("first")
	s.FailWith(first)
	s.FailWith(errors.New("second"))

	if s.State() != SessionFailed {
		t.Fatalf("state = %s, want %s", s.State(), SessionFailed)
	}

	if !errors.Is(s.Failure(), first) {
		t.Errorf("Failure = %v, want %v", s.Failure(), first)
	}
}

func TestRequestLocationInformationCarriesResponseTime(t *testing.T) {
	b, err := EncodeRequestLocationInformation(0x01, 0, 17)
	if err != nil {
		t.Fatal(err)
	}

	msg, err := Decoder(b)
	if err != nil {
		t.Fatal(err)
	}

	qos := msg.LPPMessageBody.C1.RequestLocationInformation.CriticalExtensions.C1.RequestLocationInformationR9.CommonIEsRequestLocationInformation.QoS
	if qos == nil || qos.ResponseTime == nil || qos.ResponseTime.Time != 17 {
		t.Fatalf("QoS = %+v, want a response time of 17 s", qos)
	}
}

func TestLocationResponseTime(t *testing.T) {
	now := time.Now()

	for _, tc := range []struct {
		name     string
		deadline time.Time
		want     int64
		wantErr  bool
	}{
		{"no deadline", time.Time{}, 25, false},
		{"plenty of time", now.Add(30 * time.Second), 25, false},
		{"after paging", now.Add(12 * time.Second), 10, false},
		{"almost expired", now.Add(2500 * time.Millisecond), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := LocationResponseTime(tc.deadline, now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %t", err, tc.wantErr)
			}

			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("err = %v, want it to wrap context.DeadlineExceeded", err)
			}

			if got != tc.want {
				t.Errorf("response time = %d s, want %d s", got, tc.want)
			}
		})
	}
}

func TestSessionRejectsUnsupportedLocationShape(t *testing.T) {
	point := lpptype.PolygonPoints{DegreesLatitude: 1, DegreesLongitude: 1}

	decoded, err := DecodeLPPMessage(encodeProvideLocationInformationR9(t, &lpptype.ProvideLocationInformationR9IEs{
		CommonIEsProvideLocationInformation: &lpptype.CommonIEsProvideLocationInformation{
			LocationEstimate: &lpptype.LocationCoordinates{Polygon: &lpptype.Polygon{List: []lpptype.PolygonPoints{point, point, point}}},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	pli := decoded.ProvideLocationInformation
	if pli.LocationEstimate != nil || !pli.UnsupportedLocationShape {
		t.Fatalf("ProvideLocationInformation = %+v, want an unsupported shape", pli)
	}

	s := NewSession("imsi-001010000000001", "session-4", "agnss")
	s.state = LocationRequested

	if err := s.HandleResponse(pli); !errors.Is(err, ErrUnsupportedLocationShape) {
		t.Fatalf("HandleResponse = %v, want ErrUnsupportedLocationShape", err)
	}
}
