// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"fmt"

	uper "github.com/ellanetworks/core/aper"
	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
)

// Message bodies carried over the unaligned envelope in codec.go.
//
// The LMF reads only the leading fields it acts on and then stops: every field
// it needs is at the front of its SEQUENCE, so the trailing optionals and
// extension groups a handset may add never have to be modelled.
const (
	nRootLocationInformationType = 4 // locationEstimateRequired..locationEstimatePreferred
	nRootLocationCoordinates     = 7 // ellipsoidPoint..ellipsoidArc
	nRootLatitudeSign            = 2 // north, south
	nRootAltitudeDirection       = 2 // height, depth
	// GNSS-ID ::= SEQUENCE { gnss-id ENUMERATED{ gps, sbas, qzss, galileo, glonass,
	//     ..., bds, navic-v1610 }, ... } — the extension marker follows glonass, so
	// bds and navic are extension additions rather than root values.
	nRootGNSSID            = 5
	maxGNSSSupportElements = 16

	degreesLatitudeMax  = 8388607
	degreesLongitudeMax = 16777215 // signed -8388608..8388607 stored as an unsigned offset
	altitudeMax         = 32767
	uncertaintyMax      = 127
	orientationMax      = 179
	confidenceMax       = 100
	gnssSignalIDsBits   = 8
	positioningModeBits = 3
)

//	RequestLocationInformation ::= SEQUENCE {
//	    criticalExtensions CHOICE {
//	        c1 CHOICE { requestLocationInformation-r9 RequestLocationInformation-r9-IEs, spare3, spare2, spare1 },
//	        criticalExtensionsFuture SEQUENCE {} } }
func writeRequestLocationInformation(w *uper.Writer, req *lpptype.RequestLocationInformation) error {
	if req == nil || req.CriticalExtensions.Present != 1 || req.CriticalExtensions.C1 == nil {
		return fmt.Errorf("requestLocationInformation: only the c1 critical extension is supported")
	}

	if err := w.WriteChoiceIndex(0, nRootCriticalExt, false, false); err != nil {
		return fmt.Errorf("requestLocationInformation criticalExtensions: %w", err)
	}

	if err := w.WriteChoiceIndex(0, nRootCriticalExtC1, false, false); err != nil {
		return fmt.Errorf("requestLocationInformation c1: %w", err)
	}

	ies := req.CriticalExtensions.C1.RequestLocationInformationR9
	if ies == nil {
		return fmt.Errorf("requestLocationInformation: requestLocationInformation-r9 is required")
	}

	//	RequestLocationInformation-r9-IEs ::= SEQUENCE {
	//	    commonIEsRequestLocationInformation OPTIONAL, a-gnss-RequestLocationInformation OPTIONAL,
	//	    otdoa-RequestLocationInformation OPTIONAL, ecid-RequestLocationInformation OPTIONAL,
	//	    epdu-RequestLocationInformation OPTIONAL, ... }
	// The preamble covers all five root optionals even though the LMF only ever
	// populates the first two; the remaining positions must still be signalled.
	w.WriteSequencePreamble(true, false, []bool{
		ies.CommonIEsRequestLocationInformation != nil,
		ies.AGNSSRequestLocationInformation != nil,
		false, // otdoa-RequestLocationInformation
		false, // ecid-RequestLocationInformation
		false, // epdu-RequestLocationInformation
	})

	if c := ies.CommonIEsRequestLocationInformation; c != nil {
		if err := writeCommonIEsRequestLocationInformation(w, c); err != nil {
			return err
		}
	}

	if a := ies.AGNSSRequestLocationInformation; a != nil {
		if err := writeAGNSSRequestLocationInformation(w, a); err != nil {
			return err
		}
	}

	return nil
}

//	CommonIEsRequestLocationInformation ::= SEQUENCE {
//	    locationInformationType LocationInformationType,
//	    triggeredReporting OPTIONAL, periodicalReporting OPTIONAL,
//	    additionalInformation OPTIONAL, qos OPTIONAL, environment OPTIONAL,
//	    locationCoordinateTypes OPTIONAL, velocityTypes OPTIONAL, ... }
func writeCommonIEsRequestLocationInformation(w *uper.Writer, c *lpptype.CommonIEsRequestLocationInformation) error {
	w.WriteSequencePreamble(true, false, []bool{
		false,        // triggeredReporting
		false,        // periodicalReporting
		false,        // additionalInformation
		c.QoS != nil, // qos
		false,        // environment
		false,        // locationCoordinateTypes
		false,        // velocityTypes
	})

	if err := w.WriteEnum(int(c.LocationInformationType.Value), nRootLocationInformationType, true, false); err != nil {
		return fmt.Errorf("locationInformationType: %w", err)
	}

	if c.QoS != nil {
		//	QoS ::= SEQUENCE {
		//	    horizontalAccuracy OPTIONAL, verticalCoordinateRequest BOOLEAN,
		//	    verticalAccuracy OPTIONAL, responseTime OPTIONAL, velocityRequest BOOLEAN, ... }
		w.WriteSequencePreamble(true, false, []bool{false, false, false})
		w.WriteBool(c.QoS.VerticalCoordinateRequest)
		w.WriteBool(c.QoS.VelocityRequest)
	}

	return nil
}

//	A-GNSS-RequestLocationInformation ::= SEQUENCE {
//	    gnss-PositioningInstructions GNSS-PositioningInstructions, ... }
//
//	GNSS-PositioningInstructions ::= SEQUENCE {
//	    gnss-Methods GNSS-ID-Bitmap, fineTimeAssistanceMeasReq BOOLEAN,
//	    adrMeasReq BOOLEAN, multiFreqMeasReq BOOLEAN, assistanceAvailability BOOLEAN, ... }
func writeAGNSSRequestLocationInformation(w *uper.Writer, a *lpptype.AGNSSRequestLocationInformation) error {
	w.WriteSequencePreamble(true, false, nil)
	w.WriteSequencePreamble(true, false, nil)

	gnss := a.GnssPositioningInstructions

	//	GNSS-ID-Bitmap ::= SEQUENCE { gnss-ids BIT STRING (SIZE(1..16)), ... }
	w.WriteSequencePreamble(true, false, nil)

	if err := w.WriteBitString(gnss.GnssMethods.GnssIDs.Bytes, int(gnss.GnssMethods.GnssIDs.BitLength), 1, 16, false); err != nil {
		return fmt.Errorf("gnss-ids: %w", err)
	}

	w.WriteBool(gnss.FineTimeAssistanceMeasReq)
	w.WriteBool(gnss.AdrMeasReq)
	w.WriteBool(gnss.MultiFreqMeasReq)
	w.WriteBool(gnss.AssistanceAvailability)

	return nil
}

// writeProvideCapabilities encodes the capabilities a target device reports.
// The LMF never originates it; the in-repo tester UE does, and it gives the
// decoder a peer to round-trip against.
func writeProvideCapabilities(w *uper.Writer, p *lpptype.ProvideCapabilities) error {
	if p == nil || p.CriticalExtensions.Present != 1 || p.CriticalExtensions.C1 == nil {
		return fmt.Errorf("provideCapabilities: only the c1 critical extension is supported")
	}

	if err := writeCriticalExtensionC1(w, "provideCapabilities"); err != nil {
		return err
	}

	ies := p.CriticalExtensions.C1.ProvideCapabilitiesR9
	if ies == nil {
		return fmt.Errorf("provideCapabilities: provideCapabilities-r9 is required")
	}

	w.WriteSequencePreamble(true, false, []bool{
		false, // commonIEsProvideCapabilities
		ies.AGNSSProvideCapabilities != nil,
		false, // otdoa-ProvideCapabilities
		false, // ecid-ProvideCapabilities
		false, // epdu-ProvideCapabilities
	})

	agnss := ies.AGNSSProvideCapabilities
	if agnss == nil {
		return nil
	}

	w.WriteSequencePreamble(true, false, []bool{
		agnss.GnssSupportList != nil,
		false, // assistanceDataSupportList
		false, // locationCoordinateTypes
		false, // velocityTypes
	})

	if agnss.GnssSupportList == nil {
		return nil
	}

	if err := w.WriteConstrainedLength(len(agnss.GnssSupportList.List), 1, maxGNSSSupportElements); err != nil {
		return fmt.Errorf("gnss-SupportList length: %w", err)
	}

	for i := range agnss.GnssSupportList.List {
		if err := writeGNSSSupportElement(w, &agnss.GnssSupportList.List[i]); err != nil {
			return fmt.Errorf("gnss-SupportElement %d: %w", i, err)
		}
	}

	return nil
}

func writeGNSSSupportElement(w *uper.Writer, e *lpptype.GNSSSupportElement) error {
	w.WriteSequencePreamble(true, false, []bool{
		false, // sbas-IDs
		false, // fta-MeasSupport
	})

	w.WriteSequencePreamble(true, false, nil) // GNSS-ID

	if int(e.GnssID.Value) >= nRootGNSSID {
		return fmt.Errorf("gnss-id: %d is an extension addition and is not supported", e.GnssID.Value)
	}

	if err := w.WriteEnum(int(e.GnssID.Value), nRootGNSSID, true, false); err != nil {
		return fmt.Errorf("gnss-id: %w", err)
	}

	w.WriteSequencePreamble(true, false, nil) // PositioningModes

	if err := w.WriteBitString(e.AGNSSModes.PosModes.Bytes, int(e.AGNSSModes.PosModes.BitLength), 1, 8, false); err != nil {
		return fmt.Errorf("agnss-Modes: %w", err)
	}

	w.WriteSequencePreamble(true, false, nil) // GNSS-SignalIDs

	if err := w.WriteBitString(e.GnssSignals.GnssSignalIDs.Bytes, gnssSignalIDsBits, gnssSignalIDsBits, gnssSignalIDsBits, false); err != nil {
		return fmt.Errorf("gnss-Signals: %w", err)
	}

	w.WriteBool(e.AdrSupport)
	w.WriteBool(e.VelocityMeasurementSupport)

	return nil
}

// writeProvideLocationInformation encodes the location fix a target device
// reports. As with writeProvideCapabilities, the peer side is the tester UE.
func writeProvideLocationInformation(w *uper.Writer, p *lpptype.ProvideLocationInformation) error {
	if p == nil || p.CriticalExtensions.Present != 1 || p.CriticalExtensions.C1 == nil {
		return fmt.Errorf("provideLocationInformation: only the c1 critical extension is supported")
	}

	if err := writeCriticalExtensionC1(w, "provideLocationInformation"); err != nil {
		return err
	}

	ies := p.CriticalExtensions.C1.ProvideLocationInformationR9
	if ies == nil {
		return fmt.Errorf("provideLocationInformation: provideLocationInformation-r9 is required")
	}

	w.WriteSequencePreamble(true, false, []bool{
		ies.CommonIEsProvideLocationInformation != nil,
		false, // a-gnss-ProvideLocationInformation
		false, // otdoa-ProvideLocationInformation
		false, // ecid-ProvideLocationInformation
		false, // epdu-ProvideLocationInformation
	})

	common := ies.CommonIEsProvideLocationInformation
	if common == nil {
		return nil
	}

	w.WriteSequencePreamble(true, false, []bool{
		common.LocationEstimate != nil,
		false, // velocityEstimate
		false, // locationError
	})

	if common.LocationEstimate == nil {
		return nil
	}

	return writeLocationCoordinates(w, common.LocationEstimate)
}

func writeLocationCoordinates(w *uper.Writer, c *lpptype.LocationCoordinates) error {
	if err := w.WriteChoiceIndex(c.Present-1, nRootLocationCoordinates, true, false); err != nil {
		return fmt.Errorf("locationEstimate choice: %w", err)
	}

	switch c.Present {
	case lpptype.LocationCoordinatesPresentEllipsoidPoint:
		p := c.EllipsoidPoint
		return writeLatLon(w, p.LatitudeSign, p.DegreesLatitude, p.DegreesLongitude)

	case lpptype.LocationCoordinatesPresentEllipsoidPointWithUncertaintyCircle:
		p := c.EllipsoidPointWithUncertaintyCircle
		if err := writeLatLon(w, p.LatitudeSign, p.DegreesLatitude, p.DegreesLongitude); err != nil {
			return err
		}

		return w.WriteConstrainedInt(p.Uncertainty, 0, uncertaintyMax)

	case lpptype.LocationCoordinatesPresentEllipsoidPointWithAltitudeAndUncertaintyEllipsoid:
		p := c.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid
		if err := writeLatLon(w, p.LatitudeSign, p.DegreesLatitude, p.DegreesLongitude); err != nil {
			return err
		}

		if err := w.WriteEnum(int(p.AltitudeDirection), nRootAltitudeDirection, false, false); err != nil {
			return fmt.Errorf("altitudeDirection: %w", err)
		}

		for _, f := range []struct {
			v    int64
			ub   int64
			name string
		}{
			{p.Altitude, altitudeMax, "altitude"},
			{p.UncertaintySemiMajor, uncertaintyMax, "uncertaintySemiMajor"},
			{p.UncertaintySemiMinor, uncertaintyMax, "uncertaintySemiMinor"},
			{p.OrientationMajorAxis, orientationMax, "orientationMajorAxis"},
			{p.UncertaintyAltitude, uncertaintyMax, "uncertaintyAltitude"},
			{p.Confidence, confidenceMax, "confidence"},
		} {
			if err := w.WriteConstrainedInt(f.v, 0, f.ub); err != nil {
				return fmt.Errorf("%s: %w", f.name, err)
			}
		}

		return nil

	default:
		return fmt.Errorf("locationEstimate: encoding shape %d is not implemented", c.Present-1)
	}
}

func writeLatLon(w *uper.Writer, sign aperEnum, lat, lon int64) error {
	if err := w.WriteEnum(int(sign), nRootLatitudeSign, false, false); err != nil {
		return fmt.Errorf("latitudeSign: %w", err)
	}

	if err := w.WriteConstrainedInt(lat, 0, degreesLatitudeMax); err != nil {
		return fmt.Errorf("degreesLatitude: %w", err)
	}

	if err := w.WriteConstrainedInt(lon, 0, degreesLongitudeMax); err != nil {
		return fmt.Errorf("degreesLongitude: %w", err)
	}

	return nil
}

// writeCriticalExtensionC1 mirrors expectCriticalExtensionC1 on the encode side.
func writeCriticalExtensionC1(w *uper.Writer, what string) error {
	if err := w.WriteChoiceIndex(0, nRootCriticalExt, false, false); err != nil {
		return fmt.Errorf("%s criticalExtensions: %w", what, err)
	}

	if err := w.WriteChoiceIndex(0, nRootCriticalExtC1, false, false); err != nil {
		return fmt.Errorf("%s c1: %w", what, err)
	}

	return nil
}

//	ProvideCapabilities ::= SEQUENCE { criticalExtensions CHOICE { c1 CHOICE {
//	    provideCapabilities-r9 ProvideCapabilities-r9-IEs, spare3, spare2, spare1 }, ... } }
//
// Only gnss-SupportList is read: it is the first field of
// A-GNSS-ProvideCapabilities and the only one the session acts on.
func readProvideCapabilities(r *uper.Reader) (*lpptype.ProvideCapabilities, error) {
	if err := expectCriticalExtensionC1(r, "provideCapabilities"); err != nil {
		return nil, err
	}

	//	ProvideCapabilities-r9-IEs ::= SEQUENCE {
	//	    commonIEsProvideCapabilities OPTIONAL, a-gnss-ProvideCapabilities OPTIONAL,
	//	    otdoa-... OPTIONAL, ecid-... OPTIONAL, epdu-... OPTIONAL, ... }
	_, optionals, err := r.ReadSequencePreamble(true, 5)
	if err != nil {
		return nil, fmt.Errorf("provideCapabilities-r9-IEs preamble: %w", err)
	}

	out := &lpptype.ProvideCapabilities{
		CriticalExtensions: lpptype.ProvideCapabilitiesCriticalExtensions{
			Present: 1,
			C1: &lpptype.ProvideCapabilitiesCriticalExtensionsC1{
				Present:               1,
				ProvideCapabilitiesR9: &lpptype.ProvideCapabilitiesR9IEs{},
			},
		},
	}

	if optionals[0] {
		//	CommonIEsProvideCapabilities ::= SEQUENCE { ... } — no root fields the LMF reads.
		if _, _, err := r.ReadSequencePreamble(true, 0); err != nil {
			return nil, fmt.Errorf("commonIEsProvideCapabilities preamble: %w", err)
		}
	}

	if !optionals[1] {
		return out, nil // no A-GNSS capabilities to report
	}

	//	A-GNSS-ProvideCapabilities ::= SEQUENCE {
	//	    gnss-SupportList OPTIONAL, assistanceDataSupportList OPTIONAL,
	//	    locationCoordinateTypes OPTIONAL, velocityTypes OPTIONAL, ... }
	_, agnssOpt, err := r.ReadSequencePreamble(true, 4)
	if err != nil {
		return nil, fmt.Errorf("a-gnss-ProvideCapabilities preamble: %w", err)
	}

	agnss := &lpptype.AGNSSProvideCapabilities{}
	out.CriticalExtensions.C1.ProvideCapabilitiesR9.AGNSSProvideCapabilities = agnss

	if agnssOpt[0] {
		if agnss.GnssSupportList, err = readGNSSSupportList(r); err != nil {
			return nil, err
		}
	}

	// Everything after gnss-SupportList is left undecoded on purpose.
	return out, nil
}

// GNSS-SupportList ::= SEQUENCE (SIZE(1..16)) OF GNSS-SupportElement
func readGNSSSupportList(r *uper.Reader) (*lpptype.GNSSSupportList, error) {
	n, err := r.ReadConstrainedLength(1, maxGNSSSupportElements)
	if err != nil {
		return nil, fmt.Errorf("gnss-SupportList length: %w", err)
	}

	list := &lpptype.GNSSSupportList{}

	for i := 0; i < n; i++ {
		elem, err := readGNSSSupportElement(r)
		if err != nil {
			return nil, fmt.Errorf("gnss-SupportElement %d: %w", i, err)
		}

		list.List = append(list.List, *elem)
	}

	return list, nil
}

//	GNSS-SupportElement ::= SEQUENCE {
//	    gnss-ID GNSS-ID, sbas-IDs SBAS-IDs OPTIONAL, agnss-Modes PositioningModes,
//	    gnss-Signals GNSS-SignalIDs, fta-MeasSupport SEQUENCE {...} OPTIONAL,
//	    adr-Support BOOLEAN, velocityMeasurementSupport BOOLEAN, ... }
func readGNSSSupportElement(r *uper.Reader) (*lpptype.GNSSSupportElement, error) {
	// Two root optionals: sbas-IDs and fta-MeasSupport.
	extPresent, optionals, err := r.ReadSequencePreamble(true, 2)
	if err != nil {
		return nil, fmt.Errorf("preamble: %w", err)
	}

	elem := &lpptype.GNSSSupportElement{}

	//	GNSS-ID ::= SEQUENCE { gnss-id ENUMERATED {gps, sbas, qzss, galileo, glonass, bds, navic, ...}, ... }
	if _, _, err := r.ReadSequencePreamble(true, 0); err != nil {
		return nil, fmt.Errorf("gnss-ID preamble: %w", err)
	}

	id, isExt, err := r.ReadEnum(nRootGNSSID, true)
	if err != nil {
		return nil, fmt.Errorf("gnss-id: %w", err)
	}

	if !isExt {
		elem.GnssID.Value = enumValue(id)
	}

	if optionals[0] { // sbas-IDs ::= SEQUENCE { sbas-IDs BIT STRING (SIZE(1..8)), ... }
		if _, _, err := r.ReadSequencePreamble(true, 0); err != nil {
			return nil, fmt.Errorf("sbas-IDs preamble: %w", err)
		}

		if _, _, err := r.ReadBitString(1, 8, false); err != nil {
			return nil, fmt.Errorf("sbas-IDs: %w", err)
		}
	}

	//	PositioningModes ::= SEQUENCE { posModes BIT STRING (SIZE(1..8)), ... }
	if _, _, err := r.ReadSequencePreamble(true, 0); err != nil {
		return nil, fmt.Errorf("agnss-Modes preamble: %w", err)
	}

	modes, modeBits, err := r.ReadBitString(1, 8, false)
	if err != nil {
		return nil, fmt.Errorf("agnss-Modes: %w", err)
	}

	elem.AGNSSModes.PosModes = bitString(modes, modeBits)

	//	GNSS-SignalIDs ::= SEQUENCE { gnss-SignalIDs BIT STRING (SIZE(8)), ... }
	if _, _, err := r.ReadSequencePreamble(true, 0); err != nil {
		return nil, fmt.Errorf("gnss-Signals preamble: %w", err)
	}

	sigs, sigBits, err := r.ReadBitString(gnssSignalIDsBits, gnssSignalIDsBits, false)
	if err != nil {
		return nil, fmt.Errorf("gnss-Signals: %w", err)
	}

	elem.GnssSignals.GnssSignalIDs = bitString(sigs, sigBits)

	if optionals[1] { // fta-MeasSupport ::= SEQUENCE { cellTime AccessTypes, mode PositioningModes, ... }
		if _, _, err := r.ReadSequencePreamble(true, 0); err != nil {
			return nil, fmt.Errorf("fta-MeasSupport preamble: %w", err)
		}

		if _, _, err := r.ReadSequencePreamble(true, 0); err != nil { // AccessTypes
			return nil, fmt.Errorf("fta cellTime preamble: %w", err)
		}

		if _, _, err := r.ReadBitString(1, 8, false); err != nil {
			return nil, fmt.Errorf("fta cellTime: %w", err)
		}

		if _, _, err := r.ReadSequencePreamble(true, 0); err != nil { // PositioningModes
			return nil, fmt.Errorf("fta mode preamble: %w", err)
		}

		if _, _, err := r.ReadBitString(1, 8, false); err != nil {
			return nil, fmt.Errorf("fta mode: %w", err)
		}
	}

	if elem.AdrSupport, err = r.ReadBool(); err != nil {
		return nil, fmt.Errorf("adr-Support: %w", err)
	}

	if elem.VelocityMeasurementSupport, err = r.ReadBool(); err != nil {
		return nil, fmt.Errorf("velocityMeasurementSupport: %w", err)
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, fmt.Errorf("extensions: %w", err)
		}
	}

	return elem, nil
}

//	ProvideLocationInformation ::= SEQUENCE { criticalExtensions CHOICE { c1 CHOICE {
//	    provideLocationInformation-r9 ProvideLocationInformation-r9-IEs, spare3, spare2, spare1 }, ... } }
//
// Only commonIEsProvideLocationInformation.locationEstimate is read: both are
// the first field of their SEQUENCE, and the estimate is the fix the LMF wants.
func readProvideLocationInformation(r *uper.Reader) (*lpptype.ProvideLocationInformation, error) {
	if err := expectCriticalExtensionC1(r, "provideLocationInformation"); err != nil {
		return nil, err
	}

	//	ProvideLocationInformation-r9-IEs ::= SEQUENCE {
	//	    commonIEsProvideLocationInformation OPTIONAL, a-gnss-... OPTIONAL,
	//	    otdoa-... OPTIONAL, ecid-... OPTIONAL, epdu-... OPTIONAL, ... }
	_, optionals, err := r.ReadSequencePreamble(true, 5)
	if err != nil {
		return nil, fmt.Errorf("provideLocationInformation-r9-IEs preamble: %w", err)
	}

	out := &lpptype.ProvideLocationInformation{
		CriticalExtensions: lpptype.ProvideLocationInformationCriticalExtensions{
			Present: 1,
			C1: &lpptype.ProvideLocationInformationCriticalExtensionsC1{
				Present:                      1,
				ProvideLocationInformationR9: &lpptype.ProvideLocationInformationR9IEs{},
			},
		},
	}

	if !optionals[0] {
		return out, nil // no common IEs, so no location estimate
	}

	//	CommonIEsProvideLocationInformation ::= SEQUENCE {
	//	    locationEstimate OPTIONAL, velocityEstimate OPTIONAL, locationError OPTIONAL, ... }
	_, commonOpt, err := r.ReadSequencePreamble(true, 3)
	if err != nil {
		return nil, fmt.Errorf("commonIEsProvideLocationInformation preamble: %w", err)
	}

	common := &lpptype.CommonIEsProvideLocationInformation{}
	out.CriticalExtensions.C1.ProvideLocationInformationR9.CommonIEsProvideLocationInformation = common

	if commonOpt[0] {
		if common.LocationEstimate, err = readLocationCoordinates(r); err != nil {
			return nil, err
		}
	}

	// velocityEstimate, locationError and the extensions are left undecoded.
	return out, nil
}

// LocationCoordinates ::= CHOICE { ellipsoidPoint, ellipsoidPointWithUncertaintyCircle,
//
//	ellipsoidPointWithUncertaintyEllipse, polygon, ellipsoidPointWithAltitude,
//	ellipsoidPointWithAltitudeAndUncertaintyEllipsoid, ellipsoidArc, ... }
func readLocationCoordinates(r *uper.Reader) (*lpptype.LocationCoordinates, error) {
	choice, isExt, err := r.ReadChoiceIndex(nRootLocationCoordinates, true)
	if err != nil {
		return nil, fmt.Errorf("locationEstimate choice: %w", err)
	}

	if isExt {
		return nil, fmt.Errorf("locationEstimate: unsupported extension shape")
	}

	out := &lpptype.LocationCoordinates{Present: choice + 1}

	switch out.Present {
	case lpptype.LocationCoordinatesPresentEllipsoidPoint:
		p := &lpptype.EllipsoidPoint{}
		if p.LatitudeSign, p.DegreesLatitude, p.DegreesLongitude, err = readLatLon(r); err != nil {
			return nil, err
		}

		out.EllipsoidPoint = p

	case lpptype.LocationCoordinatesPresentEllipsoidPointWithUncertaintyCircle:
		p := &lpptype.EllipsoidPointWithUncertaintyCircle{}
		if p.LatitudeSign, p.DegreesLatitude, p.DegreesLongitude, err = readLatLon(r); err != nil {
			return nil, err
		}

		if p.Uncertainty, err = r.ReadConstrainedInt(0, uncertaintyMax); err != nil {
			return nil, fmt.Errorf("uncertainty: %w", err)
		}

		out.EllipsoidPointWithUncertaintyCircle = p

	case lpptype.LocationCoordinatesPresentEllipsoidPointWithAltitudeAndUncertaintyEllipsoid:
		p := &lpptype.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid{}
		if p.LatitudeSign, p.DegreesLatitude, p.DegreesLongitude, err = readLatLon(r); err != nil {
			return nil, err
		}

		dir, _, err := r.ReadEnum(nRootAltitudeDirection, false)
		if err != nil {
			return nil, fmt.Errorf("altitudeDirection: %w", err)
		}

		p.AltitudeDirection = enumValue(dir)

		for _, f := range []struct {
			dst  *int64
			ub   int64
			name string
		}{
			{&p.Altitude, altitudeMax, "altitude"},
			{&p.UncertaintySemiMajor, uncertaintyMax, "uncertaintySemiMajor"},
			{&p.UncertaintySemiMinor, uncertaintyMax, "uncertaintySemiMinor"},
			{&p.OrientationMajorAxis, orientationMax, "orientationMajorAxis"},
			{&p.UncertaintyAltitude, uncertaintyMax, "uncertaintyAltitude"},
			{&p.Confidence, confidenceMax, "confidence"},
		} {
			if *f.dst, err = r.ReadConstrainedInt(0, f.ub); err != nil {
				return nil, fmt.Errorf("%s: %w", f.name, err)
			}
		}

		out.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid = p

	default:
		return nil, fmt.Errorf("locationEstimate: decoding shape %d is not implemented", choice)
	}

	return out, nil
}

// readLatLon reads the latitudeSign/degreesLatitude/degreesLongitude triple
// every TS 23.032 shape opens with. degreesLongitude is INTEGER(-8388608..
// 8388607) on the wire; the model stores the unsigned offset, which PER encodes
// identically (the value less its lower bound).
func readLatLon(r *uper.Reader) (sign aperEnum, lat, lon int64, err error) {
	s, _, err := r.ReadEnum(nRootLatitudeSign, false)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("latitudeSign: %w", err)
	}

	if lat, err = r.ReadConstrainedInt(0, degreesLatitudeMax); err != nil {
		return 0, 0, 0, fmt.Errorf("degreesLatitude: %w", err)
	}

	if lon, err = r.ReadConstrainedInt(0, degreesLongitudeMax); err != nil {
		return 0, 0, 0, fmt.Errorf("degreesLongitude: %w", err)
	}

	return enumValue(s), lat, lon, nil
}

//	ProvideAssistanceData ::= SEQUENCE {
//	    criticalExtensions CHOICE {
//	        c1 CHOICE { provideAssistanceData-r9 ProvideAssistanceData-r9-IEs, spare3, spare2, spare1 },
//	        criticalExtensionsFuture SEQUENCE {} } }
//
// The assistance data itself carries no elements: the LMF holds no ephemeris to
// send, so an A-GNSS-ProvideAssistanceData with every field absent is the whole
// payload. Populated assistance data is rejected rather than encoded empty.
func writeProvideAssistanceData(w *uper.Writer, p *lpptype.ProvideAssistanceData) error {
	if p == nil || p.CriticalExtensions.Present != 1 || p.CriticalExtensions.C1 == nil {
		return fmt.Errorf("provideAssistanceData: only the c1 critical extension is supported")
	}

	if err := writeCriticalExtensionC1(w, "provideAssistanceData"); err != nil {
		return err
	}

	ies := p.CriticalExtensions.C1.ProvideAssistanceDataR9
	if ies == nil {
		return fmt.Errorf("provideAssistanceData: provideAssistanceData-r9 is required")
	}

	if ies.CommonIEsProvideAssistanceData != nil {
		return fmt.Errorf("provideAssistanceData: commonIEsProvideAssistanceData is not implemented")
	}

	//	ProvideAssistanceData-r9-IEs ::= SEQUENCE {
	//	    commonIEsProvideAssistanceData OPTIONAL, a-gnss-ProvideAssistanceData OPTIONAL,
	//	    otdoa-ProvideAssistanceData OPTIONAL, epdu-Provide-Assistance-Data OPTIONAL, ... }
	w.WriteSequencePreamble(true, false, []bool{
		false, // commonIEsProvideAssistanceData
		ies.AGNSSProvideAssistanceData != nil,
		false, // otdoa-ProvideAssistanceData
		false, // epdu-Provide-Assistance-Data
	})

	if a := ies.AGNSSProvideAssistanceData; a != nil {
		if a.GnssCommonAssistData != nil || a.GnssGenericAssistData != nil || a.GnssError != nil {
			return fmt.Errorf("provideAssistanceData: a-gnss assistance elements are not implemented")
		}

		//	A-GNSS-ProvideAssistanceData ::= SEQUENCE {
		//	    gnss-CommonAssistData OPTIONAL, gnss-GenericAssistData OPTIONAL,
		//	    gnss-Error OPTIONAL, ... }
		w.WriteSequencePreamble(true, false, []bool{false, false, false})
	}

	return nil
}

// readProvideAssistanceData mirrors writeProvideAssistanceData. It reads the
// structure and stops: no assistance element is decoded into a model, so the
// body kind is all the caller learns.
func readProvideAssistanceData(r *uper.Reader) (*lpptype.ProvideAssistanceData, error) {
	if err := expectCriticalExtensionC1(r, "provideAssistanceData"); err != nil {
		return nil, err
	}

	_, optionals, err := r.ReadSequencePreamble(true, 4)
	if err != nil {
		return nil, fmt.Errorf("provideAssistanceData-r9-IEs preamble: %w", err)
	}

	out := &lpptype.ProvideAssistanceData{
		CriticalExtensions: lpptype.ProvideAssistanceDataCriticalExtensions{
			Present: 1,
			C1: &lpptype.ProvideAssistanceDataCriticalExtensionsC1{
				Present:                 1,
				ProvideAssistanceDataR9: &lpptype.ProvideAssistanceDataR9IEs{},
			},
		},
	}

	if optionals[1] {
		if _, _, err := r.ReadSequencePreamble(true, 3); err != nil {
			return nil, fmt.Errorf("a-gnss-ProvideAssistanceData preamble: %w", err)
		}

		out.CriticalExtensions.C1.ProvideAssistanceDataR9.AGNSSProvideAssistanceData = &lpptype.AGNSSProvideAssistanceData{}
	}

	return out, nil
}

// readRequestCapabilities mirrors writeRequestCapabilities. The LMF never
// receives one and the tester UE answers every request the same way, so the
// structure is read to confirm the shape and nothing is decoded into a model.
func readRequestCapabilities(r *uper.Reader) (*lpptype.RequestCapabilities, error) {
	if err := expectCriticalExtensionC1(r, "requestCapabilities"); err != nil {
		return nil, err
	}

	if _, _, err := r.ReadSequencePreamble(true, 5); err != nil {
		return nil, fmt.Errorf("requestCapabilities-r9-IEs preamble: %w", err)
	}

	return &lpptype.RequestCapabilities{
		CriticalExtensions: lpptype.RequestCapabilitiesCriticalExtensions{
			Present: 1,
			C1: &lpptype.RequestCapabilitiesCriticalExtensionsC1{
				Present:               1,
				RequestCapabilitiesR9: &lpptype.RequestCapabilitiesR9IEs{},
			},
		},
	}, nil
}

// readRequestLocationInformation mirrors writeRequestLocationInformation, with
// the same reach as readRequestCapabilities: the tester UE acts on the body
// kind alone.
func readRequestLocationInformation(r *uper.Reader) (*lpptype.RequestLocationInformation, error) {
	if err := expectCriticalExtensionC1(r, "requestLocationInformation"); err != nil {
		return nil, err
	}

	if _, _, err := r.ReadSequencePreamble(true, 5); err != nil {
		return nil, fmt.Errorf("requestLocationInformation-r9-IEs preamble: %w", err)
	}

	return &lpptype.RequestLocationInformation{
		CriticalExtensions: lpptype.RequestLocationInformationCriticalExtensions{
			Present: 1,
			C1: &lpptype.RequestLocationInformationCriticalExtensionsC1{
				Present:                      1,
				RequestLocationInformationR9: &lpptype.RequestLocationInformationR9IEs{},
			},
		},
	}, nil
}

// expectCriticalExtensionC1 consumes the criticalExtensions/c1 CHOICE pair that
// every LPP message body opens with, rejecting the alternatives the LMF cannot
// act on rather than decoding past them.
func expectCriticalExtensionC1(r *uper.Reader, what string) error {
	ext, _, err := r.ReadChoiceIndex(nRootCriticalExt, false)
	if err != nil {
		return fmt.Errorf("%s criticalExtensions: %w", what, err)
	}

	if ext != 0 {
		return fmt.Errorf("%s: criticalExtensionsFuture is not supported", what)
	}

	c1, _, err := r.ReadChoiceIndex(nRootCriticalExtC1, false)
	if err != nil {
		return fmt.Errorf("%s c1: %w", what, err)
	}

	if c1 != 0 {
		return fmt.Errorf("%s: c1 alternative %d is not supported", what, c1)
	}

	return nil
}
