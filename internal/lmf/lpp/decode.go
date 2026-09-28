// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"fmt"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/lpp/models"
	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/per"
)

// Decoder parses unaligned-PER bytes into an LPP-Message.
func Decoder(data []byte) (*lpptype.LPPMessage, error) {
	msg := &lpptype.LPPMessage{}
	if err := per.Unmarshal(data, msg, per.Unaligned); err != nil {
		return nil, fmt.Errorf("decode LPP-Message: %w", err)
	}

	return msg, nil
}

// DecodeLPPMessage decodes unaligned-PER-encoded LPP bytes and returns
// the transaction ID, initiator, and the discriminated message body.
type DecodedMessage struct {
	TransactionID        byte
	TransactionIDPresent bool
	Initiator            lpptype.Initiator
	EndTransaction       bool
	SequenceNumber       *int64 // nil when absent (TS 37.355 §4.3.2)
	AckRequested         bool   // the UE asked us to acknowledge this message
	BodyKind             int    // LPPMessageBodyC1Present*
	// Typed payloads (only one is non-nil depending on BodyKind):
	ProvideCapabilities        *models.ProvideLocationCapabilities
	ProvideLocationInformation *models.ProvideLocationInformation
	RequestCapabilities        *models.RequestLocationInformation
	RequestLocationInformation *models.RequestLocationInformation
	ProvideAssistanceData      *models.ProvideAssistanceData
	RequestAssistanceData      *models.RequestAssistanceData
	Abort                      *models.Abort
	Error                      *models.Error
}

// Payload returns the typed model for the decoded body, or an error if the body
// kind is not one the LMF handles.
func (d *DecodedMessage) Payload() (any, error) {
	switch d.BodyKind {
	case lpptype.LPPMessageBodyC1PresentProvideCapabilities:
		return d.ProvideCapabilities, nil
	case lpptype.LPPMessageBodyC1PresentProvideLocationInformation:
		return d.ProvideLocationInformation, nil
	case lpptype.LPPMessageBodyC1PresentRequestCapabilities:
		return d.RequestCapabilities, nil
	case lpptype.LPPMessageBodyC1PresentRequestLocationInformation:
		return d.RequestLocationInformation, nil
	case lpptype.LPPMessageBodyC1PresentProvideAssistanceData:
		return d.ProvideAssistanceData, nil
	case lpptype.LPPMessageBodyC1PresentRequestAssistanceData:
		return d.RequestAssistanceData, nil
	case lpptype.LPPMessageBodyC1PresentAbort:
		return d.Abort, nil
	case lpptype.LPPMessageBodyC1PresentError:
		return d.Error, nil
	case 0:
		return nil, fmt.Errorf("LPP message has no body")
	default:
		return nil, fmt.Errorf("unsupported LPP message body kind: %d", d.BodyKind)
	}
}

// DecodeLPPMessage decodes PER-encoded LPP bytes and returns a DecodedMessage.
func DecodeLPPMessage(data []byte) (*DecodedMessage, error) {
	msg, err := Decoder(data)
	if err != nil {
		return nil, err
	}

	out := &DecodedMessage{
		EndTransaction: msg.EndTransaction,
		SequenceNumber: msg.SequenceNumber,
	}

	if msg.Acknowledgement != nil {
		out.AckRequested = msg.Acknowledgement.AckRequested
	}

	if msg.TransactionID != nil {
		out.TransactionID = byte(msg.TransactionID.TransactionNumber)
		out.TransactionIDPresent = true
		out.Initiator = msg.TransactionID.Initiator
	}

	if msg.LPPMessageBody == nil || msg.LPPMessageBody.C1 == nil {
		return out, nil
	}

	c1 := msg.LPPMessageBody.C1

	switch {
	case c1.RequestCapabilities != nil:
		out.BodyKind = lpptype.LPPMessageBodyC1PresentRequestCapabilities
		out.RequestCapabilities = decodeRequestCapabilities(c1.RequestCapabilities)

	case c1.ProvideCapabilities != nil:
		out.BodyKind = lpptype.LPPMessageBodyC1PresentProvideCapabilities
		out.ProvideCapabilities = decodeProvideCapabilities(c1.ProvideCapabilities)

	case c1.ProvideAssistanceData != nil:
		out.BodyKind = lpptype.LPPMessageBodyC1PresentProvideAssistanceData
		out.ProvideAssistanceData = decodeProvideAssistanceData(c1.ProvideAssistanceData)

	case c1.RequestLocationInformation != nil:
		out.BodyKind = lpptype.LPPMessageBodyC1PresentRequestLocationInformation
		out.RequestLocationInformation = decodeRequestLocationInformation(c1.RequestLocationInformation)

	case c1.ProvideLocationInformation != nil:
		out.BodyKind = lpptype.LPPMessageBodyC1PresentProvideLocationInformation
		out.ProvideLocationInformation = decodeProvideLocationInformation(c1.ProvideLocationInformation)

	case c1.RequestAssistanceData != nil:
		out.BodyKind = lpptype.LPPMessageBodyC1PresentRequestAssistanceData
		out.RequestAssistanceData = decodeRequestAssistanceData(c1.RequestAssistanceData, out.TransactionID)

	case c1.Abort != nil:
		out.BodyKind = lpptype.LPPMessageBodyC1PresentAbort
		out.Abort = decodeAbort(c1.Abort, out.TransactionID)

	case c1.Error != nil:
		out.BodyKind = lpptype.LPPMessageBodyC1PresentError
		out.Error = decodeError(c1.Error, out.TransactionID)

	default:
		// Spare — not decoded into a model.
	}

	return out, nil
}

// decodeProvideCapabilities extracts GNSS capability info from a ProvideCapabilities message.
func decodeProvideCapabilities(pc *lpptype.ProvideCapabilities) *models.ProvideLocationCapabilities {
	out := &models.ProvideLocationCapabilities{}

	if pc == nil {
		return out
	}

	ce := pc.CriticalExtensions
	if ce.C1 == nil {
		return out
	}

	c1 := ce.C1
	if c1.ProvideCapabilitiesR9 == nil {
		return out
	}

	r9 := c1.ProvideCapabilitiesR9
	if r9.AGNSSProvideCapabilities == nil || r9.AGNSSProvideCapabilities.GNSSSupportList == nil {
		return out
	}

	for _, elem := range r9.AGNSSProvideCapabilities.GNSSSupportList.List {
		var gnssID models.GnssID

		switch elem.GNSSID.GNSSID {
		case lpptype.GNSSIDGPS:
			gnssID = models.GnssIDGps
		case lpptype.GNSSIDSBAS:
			gnssID = models.GnssIDSbas
		case lpptype.GNSSIDQZSS:
			gnssID = models.GnssIDQzss
		case lpptype.GNSSIDGalileo:
			gnssID = models.GnssIDGalileo
		case lpptype.GNSSIDGLONASS:
			gnssID = models.GnssIDGlonass
		case lpptype.GNSSIDBDS:
			gnssID = models.GnssIDBds
		case lpptype.GNSSIDNavIC:
			gnssID = models.GnssIDNavic
		default:
			continue
		}

		out.GNSSCapability.AddSupported(gnssID)
	}

	return out
}

// decodeProvideLocationInformation extracts the location estimate from a ProvideLocationInformation message.
func decodeProvideLocationInformation(pli *lpptype.ProvideLocationInformation) *models.ProvideLocationInformation {
	out := &models.ProvideLocationInformation{}

	if pli == nil {
		return out
	}

	ce := pli.CriticalExtensions
	if ce.C1 == nil {
		return out
	}

	c1 := ce.C1
	if c1.ProvideLocationInformationR9 == nil {
		return out
	}

	r9 := c1.ProvideLocationInformationR9
	if a := r9.AGNSSProvideLocationInformation; a != nil && a.GNSSError != nil && a.GNSSError.TargetDeviceErrorCauses != nil {
		cause := int64(a.GNSSError.TargetDeviceErrorCauses.Cause)
		out.GNSSErrorCause = &cause
	}

	if r9.CommonIEsProvideLocationInformation == nil {
		return out
	}

	common := r9.CommonIEsProvideLocationInformation
	if common.LocationError != nil {
		cause := int64(common.LocationError.LocationFailureCause)
		out.LocationFailureCause = &cause
	}

	if common.LocationEstimate == nil {
		return out
	}

	out.LocationEstimate, out.UnsupportedLocationShape = decodeLocationEstimate(common.LocationEstimate)

	return out
}

// decodeRequestCapabilities extracts capability request info.
func decodeRequestCapabilities(_ *lpptype.RequestCapabilities) *models.RequestLocationInformation {
	return &models.RequestLocationInformation{PositioningMethod: PosMethodGNSS}
}

// decodeRequestLocationInformation extracts location request info.
func decodeRequestLocationInformation(_ *lpptype.RequestLocationInformation) *models.RequestLocationInformation {
	return &models.RequestLocationInformation{PositioningMethod: PosMethodGNSS}
}

// decodeProvideAssistanceData extracts assistance data.
func decodeProvideAssistanceData(pad *lpptype.ProvideAssistanceData) *models.ProvideAssistanceData {
	return &models.ProvideAssistanceData{}
}

func decodeRequestAssistanceData(rad *lpptype.RequestAssistanceData, transactionID byte) *models.RequestAssistanceData {
	out := &models.RequestAssistanceData{TransactionID: transactionID}

	if c1 := rad.CriticalExtensions.C1; c1 != nil && c1.RequestAssistanceDataR9 != nil {
		out.AGNSS = c1.RequestAssistanceDataR9.AGNSSRequestAssistanceData != nil
	}

	return out
}

func decodeAbort(a *lpptype.Abort, transactionID byte) *models.Abort {
	out := &models.Abort{TransactionID: transactionID}

	if c1 := a.CriticalExtensions.C1; c1 != nil && c1.AbortR9 != nil && c1.AbortR9.CommonIEsAbort != nil {
		cause := int64(c1.AbortR9.CommonIEsAbort.AbortCause)
		out.Cause = &cause
	}

	return out
}

func decodeError(e *lpptype.Error, transactionID byte) *models.Error {
	out := &models.Error{TransactionID: transactionID}

	if e.ErrorR9 != nil && e.ErrorR9.CommonIEsError != nil {
		cause := int64(e.ErrorR9.CommonIEsError.ErrorCause)
		out.Cause = &cause
	}

	return out
}

func DecodeLPPHeader(data []byte) (*DecodedMessage, error) {
	r := per.NewReader(data)
	enc := per.Unaligned

	var present [4]bool

	for i := range present {
		b, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("decode LPP-Message header: %w", err)
		}

		present[i] = b
	}

	out := &DecodedMessage{}

	if present[0] {
		var tid lpptype.LPPTransactionID
		if err := tid.UnmarshalPER(r, enc); err != nil {
			return nil, fmt.Errorf("decode LPP-TransactionID: %w", err)
		}

		out.TransactionID = byte(tid.TransactionNumber)
		out.TransactionIDPresent = true
		out.Initiator = tid.Initiator
	}

	end, err := per.DecodeBoolean(r, enc)
	if err != nil {
		return nil, fmt.Errorf("decode endTransaction: %w", err)
	}

	out.EndTransaction = end

	if present[1] {
		seq, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
		if err != nil {
			return nil, fmt.Errorf("decode sequenceNumber: %w", err)
		}

		out.SequenceNumber = &seq
	}

	if present[2] {
		var ack lpptype.Acknowledgement
		if err := ack.UnmarshalPER(r, enc); err != nil {
			return nil, fmt.Errorf("decode acknowledgement: %w", err)
		}

		out.AckRequested = ack.AckRequested
	}

	if !present[3] {
		return out, nil
	}

	class, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("decode LPP-MessageBody: %w", err)
	}

	if class {
		return out, nil
	}

	idx, err := r.ReadBits(4)
	if err != nil {
		return nil, fmt.Errorf("decode LPP-MessageBody c1: %w", err)
	}

	out.BodyKind = int(idx) + lpptype.LPPMessageBodyC1PresentRequestCapabilities

	return out, nil
}

func decodeLocationEstimate(lc *lpptype.LocationCoordinates) (*lmfmodels.GeographicEstimate, bool) {
	switch {
	case lc.EllipsoidPoint != nil:
		ep := lc.EllipsoidPoint

		return &lmfmodels.GeographicEstimate{
			LatitudeDegrees:  latitudeDegrees(ep.LatitudeSign, ep.DegreesLatitude),
			LongitudeDegrees: longitudeDegrees(ep.DegreesLongitude),
		}, false

	case lc.EllipsoidPointWithUncertaintyCircle != nil:
		ep := lc.EllipsoidPointWithUncertaintyCircle
		radius := lmfmodels.HorizontalUncertaintyMeters(ep.Uncertainty)

		return &lmfmodels.GeographicEstimate{
			LatitudeDegrees:         latitudeDegrees(ep.LatitudeSign, ep.DegreesLatitude),
			LongitudeDegrees:        longitudeDegrees(ep.DegreesLongitude),
			UncertaintyRadiusMeters: &radius,
		}, false

	case lc.EllipsoidPointWithUncertaintyEllipse != nil:
		ep := lc.EllipsoidPointWithUncertaintyEllipse
		confidence := int32(ep.Confidence)

		return &lmfmodels.GeographicEstimate{
			LatitudeDegrees:  latitudeDegrees(ep.LatitudeSign, ep.DegreesLatitude),
			LongitudeDegrees: longitudeDegrees(ep.DegreesLongitude),
			UncertaintyEllipse: &lmfmodels.UncertaintyEllipse{
				SemiMajorMeters:         lmfmodels.HorizontalUncertaintyMeters(ep.UncertaintySemiMajor),
				SemiMinorMeters:         lmfmodels.HorizontalUncertaintyMeters(ep.UncertaintySemiMinor),
				OrientationMajorDegrees: int32(ep.OrientationMajorAxis),
			},
			ConfidencePercent: &confidence,
		}, false

	case lc.EllipsoidPointWithAltitude != nil:
		ep := lc.EllipsoidPointWithAltitude
		altitude := altitudeMeters(ep.AltitudeDirection, ep.Altitude)

		return &lmfmodels.GeographicEstimate{
			LatitudeDegrees:  latitudeDegrees(ep.LatitudeSign, ep.DegreesLatitude),
			LongitudeDegrees: longitudeDegrees(ep.DegreesLongitude),
			AltitudeMeters:   &altitude,
		}, false

	case lc.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid != nil:
		ep := lc.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid

		return lmfmodels.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid{
			LatitudeDegrees:      latitudeDegrees(ep.LatitudeSign, ep.DegreesLatitude),
			LongitudeDegrees:     longitudeDegrees(ep.DegreesLongitude),
			AltitudeMeters:       altitudeMeters(ep.AltitudeDirection, ep.Altitude),
			UncertaintySemiMajor: ep.UncertaintySemiMajor,
			UncertaintySemiMinor: ep.UncertaintySemiMinor,
			OrientationMajor:     ep.OrientationMajorAxis,
			UncertaintyAltitude:  ep.UncertaintyAltitude,
			Confidence:           ep.Confidence,
		}.Estimate(), false

	default:
		return nil, true
	}
}

func latitudeDegrees(sign lpptype.LatitudeSign, encoded int64) float64 {
	degrees := float64(encoded) * 90 / latitudeResolution
	if sign == lpptype.LatitudeSignSouth {
		return -degrees
	}

	return degrees
}

func longitudeDegrees(encoded int64) float64 {
	return float64(encoded) * 360 / longitudeResolution
}

func altitudeMeters(direction lpptype.AltitudeDirection, encoded int64) float64 {
	if direction == lpptype.AltitudeDirectionDepth {
		return -float64(encoded)
	}

	return float64(encoded)
}
