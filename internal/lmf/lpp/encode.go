// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"math"
	"time"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/per"
)

// Encoder serialises an LPP-Message to unaligned-PER bytes (TS 37.355 §7).
func Encoder(msg *lpptype.LPPMessage) ([]byte, error) {
	return per.Marshal(msg, per.Unaligned)
}

// encodeLPPMessage is a convenience wrapper that builds and encodes an LPP-Message.
// Every server message carries a sequenceNumber: a UE that uses the acknowledgement
// mechanism expects all peer messages to be sequenced (TS 37.355 §4.3.2).
func encodeLPPMessage(transactionID byte, initiator lpptype.Initiator, body *lpptype.LPPMessageBody, endTransaction bool, sequenceNumber byte) ([]byte, error) {
	seq := int64(sequenceNumber)

	msg := &lpptype.LPPMessage{
		TransactionID: &lpptype.LPPTransactionID{
			Initiator:         initiator,
			TransactionNumber: int64(transactionID),
		},
		EndTransaction: endTransaction,
		SequenceNumber: &seq,
		LPPMessageBody: body,
	}

	return Encoder(msg)
}

// EncodeRequestCapabilities encodes an LPP RequestCapabilities message.
func EncodeRequestCapabilities(transactionID, sequenceNumber byte) ([]byte, error) {
	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			RequestCapabilities: &lpptype.RequestCapabilities{
				CriticalExtensions: lpptype.RequestCapabilitiesCriticalExtensions{
					C1: &lpptype.RequestCapabilitiesCriticalExtensionsC1{
						RequestCapabilitiesR9: &lpptype.RequestCapabilitiesR9IEs{
							AGNSSRequestCapabilities: &lpptype.AGNSSRequestCapabilities{
								GNSSSupportListReq:           true,
								AssistanceDataSupportListReq: true,
								LocationVelocityTypesReq:     true,
							},
						},
					},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorLocationServer, body, false, sequenceNumber)
}

// EncodeRequestLocationInformation encodes an LPP RequestLocationInformation message
// requesting a GNSS location estimate.
func EncodeRequestLocationInformation(transactionID, sequenceNumber byte, responseTimeSeconds int64) ([]byte, error) {
	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			RequestLocationInformation: &lpptype.RequestLocationInformation{
				CriticalExtensions: lpptype.RequestLocationInformationCriticalExtensions{
					C1: &lpptype.RequestLocationInformationCriticalExtensionsC1{
						RequestLocationInformationR9: &lpptype.RequestLocationInformationR9IEs{
							CommonIEsRequestLocationInformation: &lpptype.CommonIEsRequestLocationInformation{
								LocationInformationType: lpptype.LocationInformationTypeLocationEstimateRequired,
								QoS: &lpptype.QoS{
									VerticalCoordinateRequest: false,
									ResponseTime:              &lpptype.ResponseTime{Time: responseTimeSeconds},
									VelocityRequest:           false,
								},
								LocationCoordinateTypes: &lpptype.LocationCoordinateTypes{
									EllipsoidPoint:                                    true,
									EllipsoidPointWithUncertaintyCircle:               true,
									EllipsoidPointWithUncertaintyEllipse:              true,
									Polygon:                                           false,
									EllipsoidPointWithAltitude:                        true,
									EllipsoidPointWithAltitudeAndUncertaintyEllipsoid: true,
									EllipsoidArc:                                      false,
								},
							},
							AGNSSRequestLocationInformation: &lpptype.AGNSSRequestLocationInformation{
								GNSSPositioningInstructions: lpptype.GNSSPositioningInstructions{
									GNSSMethods: lpptype.GNSSIDBitmap{
										GNSSIDs: makeGnssIdBitmap(lpptype.GNSSIDBitmapGPS), // GPS only
									},
									FineTimeAssistanceMeasReq: false,
									ADRMeasReq:                false,
									MultiFreqMeasReq:          false,
									AssistanceAvailability:    false,
								},
							},
						},
					},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorLocationServer, body, false, sequenceNumber)
}

// EncodeProvideCapabilities encodes an LPP ProvideCapabilities message
// indicating support for the given GNSS constellations.
func EncodeProvideCapabilities(transactionID byte, gnssIDs []lpptype.GNSSIDValue) ([]byte, error) {
	supportList := &lpptype.GNSSSupportList{}

	for _, id := range gnssIDs {
		supportList.List = append(supportList.List, lpptype.GNSSSupportElement{
			GNSSID:     lpptype.GNSSID{GNSSID: id},
			AGNSSModes: lpptype.PositioningModes{PosModes: makePosModes(true, false, true)}, // standalone + ue-assisted
			GNSSSignals: lpptype.GNSSSignalIDs{
				GNSSSignalIDs: makeGnssSignalBitmap(0), // first signal type
			},
			ADRSupport:                 false,
			VelocityMeasurementSupport: false,
		})
	}

	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			ProvideCapabilities: &lpptype.ProvideCapabilities{
				CriticalExtensions: lpptype.ProvideCapabilitiesCriticalExtensions{
					C1: &lpptype.ProvideCapabilitiesCriticalExtensionsC1{
						ProvideCapabilitiesR9: &lpptype.ProvideCapabilitiesR9IEs{
							AGNSSProvideCapabilities: &lpptype.AGNSSProvideCapabilities{
								GNSSSupportList: supportList,
							},
						},
					},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorTargetDevice, body, true, 0)
}

func EncodeAssistanceDataNotSupported(transactionID byte, sequenceNumber byte) ([]byte, error) {
	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			ProvideAssistanceData: &lpptype.ProvideAssistanceData{
				CriticalExtensions: lpptype.ProvideAssistanceDataCriticalExtensions{
					C1: &lpptype.ProvideAssistanceDataCriticalExtensionsC1{
						ProvideAssistanceDataR9: &lpptype.ProvideAssistanceDataR9IEs{
							AGNSSProvideAssistanceData: &lpptype.AGNSSProvideAssistanceData{
								GNSSError: &lpptype.AGNSSError{
									LocationServerErrorCauses: &lpptype.GNSSLocationServerErrorCauses{
										Cause: lpptype.GNSSLocationServerErrorCauseUndeliveredAssistanceDataIsNotSupportedByServer,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorTargetDevice, body, true, sequenceNumber)
}

func EncodeError(sequenceNumber byte, received *DecodedMessage, cause lpptype.ErrorCause) ([]byte, error) {
	seq := int64(sequenceNumber)

	msg := &lpptype.LPPMessage{
		EndTransaction: true,
		SequenceNumber: &seq,
		LPPMessageBody: &lpptype.LPPMessageBody{
			C1: &lpptype.LPPMessageBodyC1{
				Error: &lpptype.Error{
					ErrorR9: &lpptype.ErrorR9IEs{
						CommonIEsError: &lpptype.CommonIEsError{ErrorCause: cause},
					},
				},
			},
		},
	}

	if received != nil && received.TransactionIDPresent {
		msg.TransactionID = &lpptype.LPPTransactionID{
			Initiator:         received.Initiator,
			TransactionNumber: int64(received.TransactionID),
		}
	}

	return Encoder(msg)
}

// EncodeProvideLocationInformation encodes an LPP ProvideLocationInformation
// message carrying a GNSS-derived location estimate as an ellipsoid point with
// altitude and uncertainty ellipsoid (TS 23.032).
// hAcc and vAcc are the horizontal and vertical accuracy in meters.
func EncodeProvideLocationInformation(transactionID byte, lat int32, lon int32, alt int32, hAcc, vAcc uint32) ([]byte, error) {
	latSign, latAbs := encodeLatitude(lat)
	altDir, altAbs := encodeAltitude(alt)
	lonEncoded := encodeLongitude(lon)

	uncSemiMajor := encodeUncertainty(hAcc)
	uncSemiMinor := uncSemiMajor
	uncAltitude := encodeAltitudeUncertainty(vAcc)

	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			ProvideLocationInformation: &lpptype.ProvideLocationInformation{
				CriticalExtensions: lpptype.ProvideLocationInformationCriticalExtensions{
					C1: &lpptype.ProvideLocationInformationCriticalExtensionsC1{
						ProvideLocationInformationR9: &lpptype.ProvideLocationInformationR9IEs{
							CommonIEsProvideLocationInformation: &lpptype.CommonIEsProvideLocationInformation{
								LocationEstimate: &lpptype.LocationCoordinates{
									EllipsoidPointWithAltitudeAndUncertaintyEllipsoid: &lpptype.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid{
										LatitudeSign:         latSign,
										DegreesLatitude:      latAbs,
										DegreesLongitude:     lonEncoded,
										AltitudeDirection:    altDir,
										Altitude:             altAbs,
										UncertaintySemiMajor: uncSemiMajor,
										UncertaintySemiMinor: uncSemiMinor,
										OrientationMajorAxis: 0,
										UncertaintyAltitude:  uncAltitude,
										Confidence:           defaultConfidence,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorTargetDevice, body, true, 0)
}

// =====================================================================
// Helpers
// =====================================================================

// makeGnssIdBitmap creates a GNSS-ID-Bitmap with the given bit positions set.
// Bit 0 = GPS, 1 = SBAS, 2 = QZSS, 3 = Galileo, 4 = GLONASS, 5 = BDS, 6 = NavIC.
func makeGnssIdBitmap(gnssBits ...int) []bool {
	bs := make([]bool, gnssIdBitmapBitLength)

	for _, bit := range gnssBits {
		if bit >= 0 && bit < 7 {
			bs[bit] = true
		}
	}

	return namedBits(bs...)
}

// makeGnssSignalBitmap creates a GNSS-SignalIDs bitmap (8 bits, MSB = signal 0).
func makeGnssSignalBitmap(signalBits ...int) []bool {
	bs := make([]bool, gnssSignalIDsBitLength)

	for _, bit := range signalBits {
		if bit >= 0 && bit < 8 {
			bs[bit] = true
		}
	}

	return bs
}

const (
	posModesBitLength      = 3
	gnssIdBitmapBitLength  = 7
	gnssSignalIDsBitLength = 8

	maxLocationResponseTime    = 25 * time.Second
	locationResponseTimeMargin = 2 * time.Second
	minLocationResponseTime    = time.Second
)

// makePosModes creates a PositioningModes bitmap.
func makePosModes(standalone, ueBased, ueAssisted bool) []bool {
	bs := make([]bool, posModesBitLength)

	if standalone {
		bs[lpptype.PosModesStandalone] = true
	}

	if ueBased {
		bs[lpptype.PosModesUEBased] = true
	}

	if ueAssisted {
		bs[lpptype.PosModesUEAssisted] = true
	}

	return namedBits(bs...)
}

// encodeLatitude converts a signed 1e-7-degree latitude to TS 23.032 encoding.
// Returns (latitudeSign, degreesLatitude) where degreesLatitude is 0..maxDegreesLatitude.
// TS 23.032: latitude = N * 90 / 2^23, so N = lat_deg * 2^23 / 90.
func encodeLatitude(latE7 int32) (lpptype.LatitudeSign, int64) {
	latAbs := int64(latE7)
	if latAbs < 0 {
		latAbs = -latAbs
	}

	encoded := latAbs * latitudeResolution / maxLatitudeE7
	if encoded > maxDegreesLatitude {
		encoded = maxDegreesLatitude
	}

	if latE7 < 0 {
		return lpptype.LatitudeSignSouth, encoded
	}

	return lpptype.LatitudeSignNorth, encoded
}

// encodeLongitude converts a signed 1e-7-degree longitude to TS 23.032 encoding.
// TS 23.032: N <= lon_deg * 2^24 / 360 < N+1, with N in range
// minDegreesLongitude..maxDegreesLongitude.
func encodeLongitude(lonE7 int32) int64 {
	scaled := int64(lonE7) * longitudeResolution

	encoded := scaled / maxLongitudeE7
	if scaled%maxLongitudeE7 != 0 && scaled < 0 {
		encoded--
	}

	if encoded > maxDegreesLongitude {
		encoded = maxDegreesLongitude
	}

	if encoded < minDegreesLongitude {
		encoded = minDegreesLongitude
	}

	return encoded
}

// encodeAltitude converts a signed centimetre altitude to TS 23.032 encoding.
// Returns (altitudeDirection, altitude) where altitude is 0..maxAltitude.
func encodeAltitude(altCm int32) (lpptype.AltitudeDirection, int64) {
	if altCm < 0 {
		altM := int64(-altCm) / centimetresPerMetre
		if altM > maxAltitude {
			altM = maxAltitude
		}

		return lpptype.AltitudeDirectionDepth, altM
	}

	altM := int64(altCm) / centimetresPerMetre
	if altM > maxAltitude {
		altM = maxAltitude
	}

	return lpptype.AltitudeDirectionHeight, altM
}

// encodeUncertainty converts a distance in metres to a TS 23.032 uncertainty
// code (0..maxUncertaintyCode). It is the inverse of models.HorizontalUncertaintyMeters: given r
// metres, find k such that r = C * ((1+x)^k - 1) with C = uncertaintyConstantC
// and x = uncertaintyFactorX.
func encodeUncertainty(meters uint32) int64 {
	if meters == 0 {
		return 0
	}

	r := float64(meters)
	k := math.Log(r/uncertaintyConstantC+1.0) / math.Log(uncertaintyBase)

	code := int64(math.Round(k))
	if code < 0 {
		return 0
	}

	if code > maxUncertaintyCode {
		return maxUncertaintyCode
	}

	return code
}

func encodeAltitudeUncertainty(meters uint32) int64 {
	if meters == 0 {
		return 0
	}

	k := math.Log(float64(meters)/altitudeUncertaintyConstantC+1.0) / math.Log(altitudeUncertaintyBase)

	return min(max(int64(math.Round(k)), 0), maxUncertaintyCode)
}
