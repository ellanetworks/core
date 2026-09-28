// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/lpp/models"
	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
	coremodels "github.com/ellanetworks/core/internal/models"
)

const (
	pycrateRequestECIDCapabilities         = "d000000088050180"
	pycrateRequestECIDLocationInformation  = "d0020120c820881c5c0a050380"
	pycrateProvideECIDCapabilities         = "f0010342224e0140a09e00"
	pycrateProvideECIDLocationInformation  = "f003044a2240064a122f2b640a3909545a0a1c40999380000007e0f2608207260893c0042d650dc04c80"
	pycrateProvideECIDLocationInformErrors = "90032888a040a070522000"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func float(v float64) *float64 { return &v }

func integer(v int64) *int64 { return &v }

func TestEncodeRequestECIDMatchesPycrate(t *testing.T) {
	caps, err := EncodeRequestECIDCapabilities(0, 0, true)
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(caps); got != pycrateRequestECIDCapabilities {
		t.Errorf("RequestCapabilities = %s, want %s", got, pycrateRequestECIDCapabilities)
	}

	loc, err := EncodeRequestECIDLocationInformation(1, 1, 8,
		&models.ECIDMeasurements{RSRP: true, RSRQ: true, UERxTx: true},
		&models.NRECIDMeasurements{SSRSRP: true, SSRSRQ: true})
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(loc); got != pycrateRequestECIDLocationInformation {
		t.Errorf("RequestLocationInformation = %s, want %s", got, pycrateRequestECIDLocationInformation)
	}
}

func TestEncodeRequestECIDCapabilitiesOmitsNRECIDOnEUTRA(t *testing.T) {
	msg, err := EncodeRequestECIDCapabilities(0, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeLPPMessage(msg)
	if err != nil {
		t.Fatal(err)
	}

	if decoded.RequestCapabilities.ECID == nil || decoded.RequestCapabilities.NRECID != nil || decoded.RequestCapabilities.AGNSS {
		t.Fatalf("request = %+v, want E-CID capabilities only", decoded.RequestCapabilities)
	}
}

func TestDecodeRequestECIDLocationInformation(t *testing.T) {
	decoded, err := DecodeLPPMessage(mustHex(t, pycrateRequestECIDLocationInformation))
	if err != nil {
		t.Fatal(err)
	}

	req := decoded.RequestLocationInformation
	if req.AGNSS || !reflect.DeepEqual(req.ECID, &models.ECIDMeasurements{RSRP: true, RSRQ: true, UERxTx: true}) ||
		!reflect.DeepEqual(req.NRECID, &models.NRECIDMeasurements{SSRSRP: true, SSRSRQ: true}) {
		t.Fatalf("request = %+v (ECID %+v, NR %+v)", req, req.ECID, req.NRECID)
	}
}

func TestDecodeProvideECIDCapabilities(t *testing.T) {
	decoded, err := DecodeLPPMessage(mustHex(t, pycrateProvideECIDCapabilities))
	if err != nil {
		t.Fatal(err)
	}

	caps := decoded.ProvideCapabilities
	if !reflect.DeepEqual(caps.ECID, &models.ECIDMeasurements{RSRP: true, RSRQ: true, UERxTx: true}) {
		t.Errorf("E-CID = %+v", caps.ECID)
	}

	if !reflect.DeepEqual(caps.NRECID, &models.NRECIDMeasurements{SSRSRP: true, SSRSRQ: true}) {
		t.Errorf("NR E-CID = %+v", caps.NRECID)
	}
}

func TestDecodeProvideECIDLocationInformation(t *testing.T) {
	decoded, err := DecodeLPPMessage(mustHex(t, pycrateProvideECIDLocationInformation))
	if err != nil {
		t.Fatal(err)
	}

	plmn := &coremodels.PlmnID{Mcc: "999", Mnc: "70"}
	want := []lmfmodels.CellMeasurement{
		{
			Source: lmfmodels.MeasurementSourceUE, RAT: lmfmodels.RATEUTRA,
			PCI: integer(148), ARFCN: integer(9310), RSRP: float(-98), RSRQ: float(-7.5),
		},
		{
			Source: lmfmodels.MeasurementSourceUE, RAT: lmfmodels.RATNR, Serving: true,
			PCI: integer(180), ARFCN: integer(662592), ARFCNType: lmfmodels.ARFCNTypeSSB,
			NCGI:   &coremodels.Ncgi{PlmnID: plmn, NrCellID: "000000fc1"},
			SSRSRP: float(-84), SSRSRQ: float(-11),
			SSBBeams: []lmfmodels.BeamMeasurement{
				{Index: 0, RSRP: float(-84), RSRQ: float(-11)},
				{Index: 4, RSRP: float(-97)},
			},
		},
		{
			Source: lmfmodels.MeasurementSourceUE, RAT: lmfmodels.RATNR,
			PCI: integer(181), ARFCN: integer(662400), ARFCNType: lmfmodels.ARFCNTypeCSIRSPointA,
			CSIRSRP: float(-107),
		},
	}

	if got := decoded.ProvideLocationInformation.Measurements; !reflect.DeepEqual(got, want) {
		t.Fatalf("measurements =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDecodeProvideECIDLocationInformationErrors(t *testing.T) {
	decoded, err := DecodeLPPMessage(mustHex(t, pycrateProvideECIDLocationInformErrors))
	if err != nil {
		t.Fatal(err)
	}

	pli := decoded.ProvideLocationInformation
	if len(pli.Measurements) != 0 {
		t.Errorf("measurements = %+v, want none", pli.Measurements)
	}

	if pli.ECIDErrorCause == nil || *pli.ECIDErrorCause != int64(lpptype.ECIDTargetDeviceErrorCauseRequestedMeasurementNotAvailable) {
		t.Errorf("E-CID error cause = %v", pli.ECIDErrorCause)
	}

	if pli.NRECIDErrorCause == nil || *pli.NRECIDErrorCause != int64(lpptype.NRECIDTargetDeviceErrorCauseNotAllRequestedMeasurementsPossible) {
		t.Errorf("NR E-CID error cause = %v", pli.NRECIDErrorCause)
	}
}

func TestProvideECIDEncodersRoundTrip(t *testing.T) {
	caps, err := EncodeProvideECIDCapabilities(0, 1, &models.ECIDMeasurements{RSRP: true}, &models.NRECIDMeasurements{SSRSRP: true, CSIRSRQ: true})
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeLPPMessage(caps)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(decoded.ProvideCapabilities.ECID, &models.ECIDMeasurements{RSRP: true}) ||
		!reflect.DeepEqual(decoded.ProvideCapabilities.NRECID, &models.NRECIDMeasurements{SSRSRP: true, CSIRSRQ: true}) {
		t.Fatalf("capabilities = %+v", decoded.ProvideCapabilities)
	}

	rsrp := int64(73)
	ssb := int64(662592)

	loc, err := EncodeProvideECIDLocationInformation(1, 2, nil, &lpptype.NRECIDSignalMeasurementInformation{
		NRPrimaryCellMeasuredResults: lpptype.NRMeasuredResultsElement{
			NRPhysCellID:   1,
			NRARFCN:        lpptype.NRMeasuredResultsElementARFCN{SSBARFCN: &ssb},
			ResultsSSBCell: &lpptype.MeasQuantityResults{NRRSRP: &rsrp},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	decoded, err = DecodeLPPMessage(loc)
	if err != nil {
		t.Fatal(err)
	}

	cells := decoded.ProvideLocationInformation.Measurements
	if len(cells) != 1 || !cells[0].Serving || *cells[0].SSRSRP != -84 {
		t.Fatalf("measurements = %+v", cells)
	}
}

type recordingTransport struct {
	sent      [][]byte
	completed *lmfmodels.LocationResult
}

func newECIDSession(t *testing.T, nr bool) (*Session, *recordingTransport) {
	t.Helper()

	rec := &recordingTransport{}
	s := NewSession("imsi-001010000000001", "session-ecid", MethodECID)
	s.SetNRAccess(nr)
	s.SetTransport(
		func(msg []byte) error {
			rec.sent = append(rec.sent, msg)
			return nil
		},
		func(result *lmfmodels.LocationResult) error {
			rec.completed = result
			return nil
		},
		nil, nil, nil,
	)

	if err := s.StartSession(); err != nil {
		t.Fatal(err)
	}

	return s, rec
}

func TestECIDSessionRequestsSupportedMeasurements(t *testing.T) {
	s, rec := newECIDSession(t, true)

	first, err := DecodeLPPMessage(rec.sent[0])
	if err != nil {
		t.Fatal(err)
	}

	if first.RequestCapabilities.ECID == nil || first.RequestCapabilities.NRECID == nil {
		t.Fatalf("capability request = %+v, want E-CID and NR E-CID", first.RequestCapabilities)
	}

	err = s.HandleResponse(&models.ProvideLocationCapabilities{
		ECID:   &models.ECIDMeasurements{RSRP: true, RSRQ: true},
		NRECID: &models.NRECIDMeasurements{SSRSRP: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	second, err := DecodeLPPMessage(rec.sent[1])
	if err != nil {
		t.Fatal(err)
	}

	req := second.RequestLocationInformation
	if !reflect.DeepEqual(req.ECID, &models.ECIDMeasurements{RSRP: true, RSRQ: true}) ||
		!reflect.DeepEqual(req.NRECID, &models.NRECIDMeasurements{SSRSRP: true}) || req.AGNSS {
		t.Fatalf("location request = %+v (ECID %+v, NR %+v)", req, req.ECID, req.NRECID)
	}

	cells := []lmfmodels.CellMeasurement{{Source: lmfmodels.MeasurementSourceUE, RAT: lmfmodels.RATNR, Serving: true}}
	if err := s.HandleResponse(&models.ProvideLocationInformation{Measurements: cells}); err != nil {
		t.Fatal(err)
	}

	if s.State() != LocationReceived || rec.completed == nil || !reflect.DeepEqual(rec.completed.Measurements, cells) {
		t.Fatalf("state = %s, completed = %+v", s.State(), rec.completed)
	}
}

func TestECIDSessionSkipsNRECIDOnEUTRA(t *testing.T) {
	s, rec := newECIDSession(t, false)

	err := s.HandleResponse(&models.ProvideLocationCapabilities{
		ECID:   &models.ECIDMeasurements{RSRP: true},
		NRECID: &models.NRECIDMeasurements{SSRSRP: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	req, err := DecodeLPPMessage(rec.sent[1])
	if err != nil {
		t.Fatal(err)
	}

	if req.RequestLocationInformation.ECID == nil || req.RequestLocationInformation.NRECID != nil {
		t.Fatalf("location request = %+v, want E-CID only", req.RequestLocationInformation)
	}
}

func TestECIDSessionFailsWithoutCapability(t *testing.T) {
	s, _ := newECIDSession(t, true)

	err := s.HandleResponse(&models.ProvideLocationCapabilities{NRECID: &models.NRECIDMeasurements{}})
	if !errors.Is(err, ErrUEECIDNotSupported) {
		t.Fatalf("HandleResponse = %v, want ErrUEECIDNotSupported", err)
	}
}

func TestECIDSessionFailsWithoutMeasurements(t *testing.T) {
	s, _ := newECIDSession(t, true)

	if err := s.HandleResponse(&models.ProvideLocationCapabilities{ECID: &models.ECIDMeasurements{RSRP: true}}); err != nil {
		t.Fatal(err)
	}

	cause := int64(lpptype.ECIDTargetDeviceErrorCauseRequestedMeasurementNotAvailable)

	err := s.HandleResponse(&models.ProvideLocationInformation{ECIDErrorCause: &cause})
	if !errors.Is(err, ErrUENoMeasurements) {
		t.Fatalf("HandleResponse = %v, want ErrUENoMeasurements", err)
	}

	if !strings.Contains(err.Error(), "requestedMeasurementNotAvailable") {
		t.Errorf("error %q does not name the E-CID error cause", err)
	}
}
