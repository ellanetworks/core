// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"math"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/lpp/models"
	"github.com/free5gc/aper"
)

// Decoder parses an LPP-Message. LPP is carried in the unaligned variant of
// PER (TS 37.355 §5), so it is decoded by the hand-written codec rather than
// the aligned reflection codec the other 3GPP protocols use.
func Decoder(data []byte) (*lpptype.LPPMessage, error) {
	return DecodeMessage(data)
}

// DecodeLPPMessage decodes an LPP-Message from APER bytes and returns
// the transaction ID, initiator, and the discriminated message body.
type DecodedMessage struct {
	TransactionID  byte
	Initiator      aper.Enumerated
	EndTransaction bool
	BodyKind       int // LPPMessageBodyC1Present*
	// Typed payloads (only one is non-nil depending on BodyKind):
	ProvideCapabilities        *models.ProvideLocationCapabilities
	ProvideLocationInformation *models.ProvideLocationInformation
	RequestCapabilities        *models.RequestLocationInformation
	RequestLocationInformation *models.RequestLocationInformation
	ProvideAssistanceData      *models.ProvideAssistanceData
	Abort                      *models.Abort
}

// DecodeLPPMessage decodes APER-encoded LPP bytes and returns a DecodedMessage.
func DecodeLPPMessage(data []byte) (*DecodedMessage, error) {
	msg, err := Decoder(data)
	if err != nil {
		return nil, err
	}

	out := &DecodedMessage{
		EndTransaction: msg.EndTransaction,
	}

	if msg.TransactionID != nil {
		out.TransactionID = byte(msg.TransactionID.TransactionNumber)
		out.Initiator = msg.TransactionID.Initiator.Value
	}

	if msg.LppMessageBody == nil || msg.LppMessageBody.Present != 1 || msg.LppMessageBody.C1 == nil {
		return out, nil
	}

	c1 := msg.LppMessageBody.C1
	out.BodyKind = c1.Present

	switch c1.Present {
	case lpptype.LPPMessageBodyC1PresentProvideCapabilities:
		out.ProvideCapabilities = decodeProvideCapabilities(c1.ProvideCapabilities)

	case lpptype.LPPMessageBodyC1PresentProvideLocationInformation:
		out.ProvideLocationInformation = decodeProvideLocationInformation(c1.ProvideLocationInformation)

	case lpptype.LPPMessageBodyC1PresentRequestCapabilities:
		out.RequestCapabilities = decodeRequestCapabilities(c1.RequestCapabilities)

	case lpptype.LPPMessageBodyC1PresentRequestLocationInformation:
		out.RequestLocationInformation = decodeRequestLocationInformation(c1.RequestLocationInformation)

	case lpptype.LPPMessageBodyC1PresentProvideAssistanceData:
		out.ProvideAssistanceData = decodeProvideAssistanceData(c1.ProvideAssistanceData)

	case lpptype.LPPMessageBodyC1PresentAbort:
		out.Abort = decodeAbort(c1.Abort, out.TransactionID)

	default:
		// Error and the spares are reported by body kind alone.
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
	if ce.Present != 1 || ce.C1 == nil {
		return out
	}

	c1 := ce.C1
	if c1.Present != 1 || c1.ProvideCapabilitiesR9 == nil {
		return out
	}

	r9 := c1.ProvideCapabilitiesR9
	if r9.AGNSSProvideCapabilities == nil || r9.AGNSSProvideCapabilities.GnssSupportList == nil {
		return out
	}

	for _, elem := range r9.AGNSSProvideCapabilities.GnssSupportList.List {
		var gnssID models.GnssID

		switch elem.GnssID.Value {
		case lpptype.GnssIDGps:
			gnssID = models.GnssIDGps
		case lpptype.GnssIDSbas:
			gnssID = models.GnssIDSbas
		case lpptype.GnssIDQzss:
			gnssID = models.GnssIDQzss
		case lpptype.GnssIDGalileo:
			gnssID = models.GnssIDGalileo
		case lpptype.GnssIDGlonass:
			gnssID = models.GnssIDGlonass
		case lpptype.GnssIDBds:
			gnssID = models.GnssIDBds
		case lpptype.GnssIDNavic:
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
	if ce.Present != 1 || ce.C1 == nil {
		return out
	}

	c1 := ce.C1
	if c1.Present != 1 || c1.ProvideLocationInformationR9 == nil {
		return out
	}

	r9 := c1.ProvideLocationInformationR9

	out.FailureCause = locationFailureCause(r9)

	common := r9.CommonIEsProvideLocationInformation
	if common == nil || common.LocationEstimate == nil {
		return out
	}

	out.HasEstimate = true

	lc := common.LocationEstimate
	switch lc.Present {
	case lpptype.LocationCoordinatesPresentEllipsoidPointWithAltitude:
		if lc.EllipsoidPointWithAltitude != nil {
			ep := lc.EllipsoidPointWithAltitude
			out.GNSSPositionResult.Latitude = decodeLatitude(ep.LatitudeSign, ep.DegreesLatitude)
			out.GNSSPositionResult.Longitude = decodeLongitude(ep.DegreesLongitude)
			out.GNSSPositionResult.Altitude = decodeAltitude(ep.AltitudeDirection, ep.Altitude)
		}

	case lpptype.LocationCoordinatesPresentEllipsoidPoint:
		if lc.EllipsoidPoint != nil {
			ep := lc.EllipsoidPoint
			out.GNSSPositionResult.Latitude = decodeLatitude(ep.LatitudeSign, ep.DegreesLatitude)
			out.GNSSPositionResult.Longitude = decodeLongitude(ep.DegreesLongitude)
		}

	case lpptype.LocationCoordinatesPresentEllipsoidPointWithUncertaintyCircle:
		if lc.EllipsoidPointWithUncertaintyCircle != nil {
			ep := lc.EllipsoidPointWithUncertaintyCircle
			out.GNSSPositionResult.Latitude = decodeLatitude(ep.LatitudeSign, ep.DegreesLatitude)
			out.GNSSPositionResult.Longitude = decodeLongitude(ep.DegreesLongitude)
			out.GNSSPositionResult.HorizontalAccuracy = uint32(decodeUncertainty(ep.Uncertainty))
		}

	case lpptype.LocationCoordinatesPresentEllipsoidPointWithAltitudeAndUncertaintyEllipsoid:
		if lc.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid != nil {
			ep := lc.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid
			out.GNSSPositionResult.Latitude = decodeLatitude(ep.LatitudeSign, ep.DegreesLatitude)
			out.GNSSPositionResult.Longitude = decodeLongitude(ep.DegreesLongitude)
			out.GNSSPositionResult.Altitude = decodeAltitude(ep.AltitudeDirection, ep.Altitude)
			uncMajor := decodeUncertainty(ep.UncertaintySemiMajor)

			uncMinor := decodeUncertainty(ep.UncertaintySemiMinor)
			if uncMinor > uncMajor {
				out.GNSSPositionResult.HorizontalAccuracy = uint32(uncMinor)
			} else {
				out.GNSSPositionResult.HorizontalAccuracy = uint32(uncMajor)
			}

			out.GNSSPositionResult.VerticalAccuracy = uint32(decodeUncertainty(ep.UncertaintyAltitude))
		}
	}

	return out
}

// locationFailureCause reports why a target sent no position, preferring the
// A-GNSS cause: it is the specific one, and the LMF only ever asks for A-GNSS.
func locationFailureCause(r9 *lpptype.ProvideLocationInformationR9IEs) string {
	if a := r9.AGNSSProvideLocationInformation; a != nil && a.GnssError != nil {
		if td := a.GnssError.TargetDeviceErrorCauses; td != nil {
			return lpptype.GNSSTargetDeviceErrorCauseString(td.Cause)
		}

		return "gnss-Error from location server"
	}

	if c := r9.CommonIEsProvideLocationInformation; c != nil && c.LocationError != nil {
		return lpptype.LocationFailureCauseString(c.LocationError.LocationFailureCause.Value)
	}

	return ""
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

// decodeAbort extracts the cause a peer gave for abandoning the procedure.
func decodeAbort(abort *lpptype.Abort, transactionID byte) *models.Abort {
	out := &models.Abort{TransactionID: transactionID, Cause: "absent"}

	if abort == nil || abort.CriticalExtensions.C1 == nil || abort.CriticalExtensions.C1.AbortR9 == nil {
		return out
	}

	if common := abort.CriticalExtensions.C1.AbortR9.CommonIEsAbort; common != nil {
		out.Cause = lpptype.AbortCauseString(common.AbortCause.Value)
	}

	return out
}

// decodeUncertainty converts a TS 23.032 uncertainty code to metres.
// r = C * ((1+x)^k - 1), C = uncertaintyConstantC, x = uncertaintyFactorX,
// k = uncertainty value (0..maxUncertaintyCode).
func decodeUncertainty(k int64) int64 {
	if k <= 0 {
		return 0
	}

	if k > maxUncertaintyCode {
		k = maxUncertaintyCode
	}

	r := uncertaintyConstantC * (math.Pow(uncertaintyBase, float64(k)) - 1.0)

	return int64(math.Round(r))
}
