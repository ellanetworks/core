// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"math"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/per"
)

// Encoder serialises an LPP-Message to unaligned-PER bytes (TS 37.355 §7).
func Encoder(msg *lpptype.LPPMessage) ([]byte, error) {
	return per.Marshal(msg, per.Unaligned)
}

// encodeLPPMessage is a convenience wrapper that builds and encodes an LPP-Message.
// Every server message carries a sequenceNumber: a UE that uses the acknowledgement
// mechanism expects all peer messages to be sequenced (TS 37.355 §6.1).
func encodeLPPMessage(transactionID byte, initiator int64, body *lpptype.LPPMessageBody, endTransaction bool, sequenceNumber byte) ([]byte, error) {
	seq := int64(sequenceNumber)

	msg := &lpptype.LPPMessage{
		TransactionID: &lpptype.LPPTransactionID{
			Initiator:         lpptype.Initiator{Value: initiator},
			TransactionNumber: int64(transactionID),
		},
		EndTransaction: endTransaction,
		SequenceNumber: &seq,
		LppMessageBody: body,
	}

	return Encoder(msg)
}

// EncodeAcknowledgement encodes a standalone LPP Acknowledgement (TS 37.355
// §6.1/§6.2): it confirms receipt of the UE message whose sequence number is
// ackIndicator, so a UE that set ackRequested stops retransmitting it.
// sequenceNumber is this acknowledgement's own number.
func EncodeAcknowledgement(sequenceNumber, ackIndicator byte) ([]byte, error) {
	seq := int64(sequenceNumber)
	ind := int64(ackIndicator)

	msg := &lpptype.LPPMessage{
		EndTransaction: false,
		SequenceNumber: &seq,
		Acknowledgement: &lpptype.Acknowledgement{
			AckRequested: false,
			AckIndicator: &ind,
		},
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
								GnssSupportListReq:           true,
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
func EncodeRequestLocationInformation(transactionID, sequenceNumber byte) ([]byte, error) {
	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			RequestLocationInformation: &lpptype.RequestLocationInformation{
				CriticalExtensions: lpptype.RequestLocationInformationCriticalExtensions{
					C1: &lpptype.RequestLocationInformationCriticalExtensionsC1{
						RequestLocationInformationR9: &lpptype.RequestLocationInformationR9IEs{
							CommonIEsRequestLocationInformation: &lpptype.CommonIEsRequestLocationInformation{
								LocationInformationType: lpptype.LocationInformationType{
									Value: lpptype.LocationInformationTypeLocationEstimateRequired,
								},
								QoS: &lpptype.QoS{
									VerticalCoordinateRequest: false,
									VelocityRequest:           false,
								},
							},
							AGNSSRequestLocationInformation: &lpptype.AGNSSRequestLocationInformation{
								GnssPositioningInstructions: lpptype.GNSSPositioningInstructions{
									GnssMethods: lpptype.GNSSIDBitmap{
										GnssIDs: makeGnssIdBitmap(0), // GPS only
									},
									FineTimeAssistanceMeasReq: false,
									AdrMeasReq:                false,
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
func EncodeProvideCapabilities(transactionID byte, gnssIDs []int64) ([]byte, error) {
	supportList := &lpptype.GNSSSupportList{}

	for _, id := range gnssIDs {
		supportList.List = append(supportList.List, lpptype.GNSSSupportElement{
			GnssID:     lpptype.GNSSID{Value: id},
			AGNSSModes: lpptype.PositioningModes{PosModes: makePosModes(true, false, true)}, // standalone + ue-assisted
			GnssSignals: lpptype.GNSSSignalIDs{
				GnssSignalIDs: makeGnssSignalBitmap(0), // first signal type
			},
			AdrSupport:                 false,
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
								GnssSupportList: supportList,
							},
						},
					},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorTargetDevice, body, true, 0)
}

// EncodeProvideAssistanceData encodes an LPP ProvideAssistanceData message.
// For the MVP, the assistance data payload is opaque (not fully encoded per ASN.1).
func EncodeProvideAssistanceData(transactionID byte, assistanceData []byte) ([]byte, error) {
	_ = assistanceData // MVP: assistance data encoding is Phase 3+

	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			ProvideAssistanceData: &lpptype.ProvideAssistanceData{
				CriticalExtensions: lpptype.ProvideAssistanceDataCriticalExtensions{
					C1: &lpptype.ProvideAssistanceDataCriticalExtensionsC1{
						ProvideAssistanceDataR9: &lpptype.ProvideAssistanceDataR9IEs{
							AGNSSProvideAssistanceData: &lpptype.AGNSSProvideAssistanceData{},
						},
					},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorLocationServer, body, true, 0)
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
	uncAltitude := encodeUncertainty(vAcc)

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

	return bs
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

// PositioningModes bitmap bit masks (TS 37.355 §6.4.1).
// Bit 0 = standalone, 1 = ue-based, 2 = ue-assisted.
const (
	posModeStandaloneBit = 0
	posModeUeBasedBit    = 1
	posModeUeAssistedBit = 2

	posModesBitLength      = 3
	gnssIdBitmapBitLength  = 7
	gnssSignalIDsBitLength = 8
)

// makePosModes creates a PositioningModes bitmap.
func makePosModes(standalone, ueBased, ueAssisted bool) []bool {
	bs := make([]bool, posModesBitLength)

	if standalone {
		bs[posModeStandaloneBit] = true
	}

	if ueBased {
		bs[posModeUeBasedBit] = true
	}

	if ueAssisted {
		bs[posModeUeAssistedBit] = true
	}

	return bs
}

// encodeLatitude converts a signed 1e-7-degree latitude to TS 23.032 encoding.
// Returns (latitudeSign, degreesLatitude) where degreesLatitude is 0..maxDegreesLatitude.
// TS 23.032: latitude = N * 90 / 2^23, so N = lat_deg * 2^23 / 90.
func encodeLatitude(latE7 int32) (int64, int64) {
	latAbs := int64(latE7)
	if latAbs < 0 {
		latAbs = -latAbs
	}

	encoded := latAbs * latitudeResolution / maxLatitudeE7
	if encoded > maxDegreesLatitude {
		encoded = maxDegreesLatitude
	}

	if latE7 < 0 {
		return lpptype.EllipsoidPointLatitudeSignSouth, encoded
	}

	return lpptype.EllipsoidPointLatitudeSignNorth, encoded
}

// encodeLongitude converts a signed 1e-7-degree longitude to TS 23.032 encoding.
// Returns the unsigned offset (value + longitudeOffset) in range 0..maxDegreesLongitude.
// TS 23.032: longitude = N * 360 / 2^24, so N = lon_deg * 2^24 / 360.
// The spec uses signed N in range -2^23..2^23-1, but we store the unsigned
// offset (N + longitudeOffset) for simpler handling of large negative bounds.
func encodeLongitude(lonE7 int32) int64 {
	encoded := int64(lonE7) * longitudeResolution / maxLongitudeE7
	// Shift to unsigned range: offset = N + longitudeOffset
	offset := encoded + longitudeOffset
	if offset > maxDegreesLongitude {
		offset = maxDegreesLongitude
	}

	if offset < 0 {
		offset = 0
	}

	return offset
}

// encodeAltitude converts a signed centimetre altitude to TS 23.032 encoding.
// Returns (altitudeDirection, altitude) where altitude is 0..maxAltitude.
func encodeAltitude(altCm int32) (int64, int64) {
	if altCm < 0 {
		altM := int64(-altCm) / centimetresPerMetre
		if altM > maxAltitude {
			altM = maxAltitude
		}

		return lpptype.EllipsoidPointWithAltitudeAltitudeDirectionDepth, altM
	}

	altM := int64(altCm) / centimetresPerMetre
	if altM > maxAltitude {
		altM = maxAltitude
	}

	return lpptype.EllipsoidPointWithAltitudeAltitudeDirectionHeight, altM
}

// decodeLatitude converts TS 23.032 encoded latitude back to 1e-7 degrees.
func decodeLatitude(sign, encoded int64) int32 {
	latE7 := encoded * maxLatitudeE7 / latitudeResolution
	if sign == lpptype.EllipsoidPointLatitudeSignSouth {
		return -int32(latE7)
	}

	return int32(latE7)
}

// decodeLongitude converts TS 23.032 encoded longitude (unsigned offset) back to 1e-7 degrees.
func decodeLongitude(encoded int64) int32 {
	// Convert unsigned offset back to signed value: N = offset - longitudeOffset
	signed := encoded - longitudeOffset
	return int32(signed * maxLongitudeE7 / longitudeResolution)
}

// decodeAltitude converts TS 23.032 encoded altitude back to centimetres.
func decodeAltitude(dir, encoded int64) int32 {
	altCm := encoded * centimetresPerMetre
	if dir == lpptype.EllipsoidPointWithAltitudeAltitudeDirectionDepth {
		return -int32(altCm)
	}

	return int32(altCm)
}

// encodeUncertainty converts a distance in metres to a TS 23.032 uncertainty
// code (0..maxUncertaintyCode). It is the inverse of decodeUncertainty: given r
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
