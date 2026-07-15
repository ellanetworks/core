// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"math"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/free5gc/aper"
)

// Encoder serialises an LPP-Message. LPP is carried in the unaligned variant of
// PER (TS 37.355 §5), so it is encoded by the hand-written codec rather than the
// aligned reflection codec the other 3GPP protocols use.
func Encoder(msg *lpptype.LPPMessage) ([]byte, error) {
	return EncodeMessage(msg)
}

// encodeLPPMessage is a convenience wrapper that builds and encodes an LPP-Message.
//
// endTransaction marks the last message carrying a body in the transaction
// (TS 37.355 §4.2). A request is never last: the response that closes the
// transaction is, so a request that sets it leaves the peer with nothing to
// send.
func encodeLPPMessage(transactionID byte, initiator aper.Enumerated, endTransaction bool, body *lpptype.LPPMessageBody) ([]byte, error) {
	msg := &lpptype.LPPMessage{
		TransactionID: &lpptype.LPPTransactionID{
			Initiator:         lpptype.Initiator{Value: initiator},
			TransactionNumber: int64(transactionID),
		},
		EndTransaction: endTransaction,
		LppMessageBody: body,
	}

	return Encoder(msg)
}

// EncodeRequestCapabilities encodes an LPP RequestCapabilities message.
func EncodeRequestCapabilities(transactionID byte) ([]byte, error) {
	body := &lpptype.LPPMessageBody{
		Present: 1, // c1
		C1: &lpptype.LPPMessageBodyC1{
			Present: lpptype.LPPMessageBodyC1PresentRequestCapabilities,
			RequestCapabilities: &lpptype.RequestCapabilities{
				CriticalExtensions: lpptype.RequestCapabilitiesCriticalExtensions{
					Present: 1, // c1
					C1: &lpptype.RequestCapabilitiesCriticalExtensionsC1{
						Present: 1, // requestCapabilities-r9
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

	// §5.1.1: the target's ProvideCapabilities is what ends this transaction.
	return encodeLPPMessage(transactionID, lpptype.InitiatorLocationServer, false, body)
}

// EncodeRequestLocationInformation encodes an LPP RequestLocationInformation message
// requesting a GNSS location estimate.
func EncodeRequestLocationInformation(transactionID byte) ([]byte, error) {
	body := &lpptype.LPPMessageBody{
		Present: 1, // c1
		C1: &lpptype.LPPMessageBodyC1{
			Present: lpptype.LPPMessageBodyC1PresentRequestLocationInformation,
			RequestLocationInformation: &lpptype.RequestLocationInformation{
				CriticalExtensions: lpptype.RequestLocationInformationCriticalExtensions{
					Present: 1, // c1
					C1: &lpptype.RequestLocationInformationCriticalExtensionsC1{
						Present: 1, // requestLocationInformation-r9
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

	// §5.3.1: the target's ProvideLocationInformation is what ends this transaction.
	return encodeLPPMessage(transactionID, lpptype.InitiatorLocationServer, false, body)
}

// EncodeProvideCapabilities encodes an LPP ProvideCapabilities message
// indicating support for the given GNSS constellations.
func EncodeProvideCapabilities(transactionID byte, gnssIDs []aper.Enumerated) ([]byte, error) {
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
		Present: 1, // c1
		C1: &lpptype.LPPMessageBodyC1{
			Present: lpptype.LPPMessageBodyC1PresentProvideCapabilities,
			ProvideCapabilities: &lpptype.ProvideCapabilities{
				CriticalExtensions: lpptype.ProvideCapabilitiesCriticalExtensions{
					Present: 1, // c1
					C1: &lpptype.ProvideCapabilitiesCriticalExtensionsC1{
						Present: 1, // provideCapabilities-r9
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

	// §5.1.1 step 2: the target's response ends the transaction.
	return encodeLPPMessage(transactionID, lpptype.InitiatorTargetDevice, true, body)
}

// EncodeProvideAssistanceData encodes an LPP ProvideAssistanceData message.
// For the MVP, the assistance data payload is opaque (not fully encoded per ASN.1).
func EncodeProvideAssistanceData(transactionID byte, assistanceData []byte) ([]byte, error) {
	_ = assistanceData // MVP: assistance data encoding is Phase 3+

	body := &lpptype.LPPMessageBody{
		Present: 1, // c1
		C1: &lpptype.LPPMessageBodyC1{
			Present: lpptype.LPPMessageBodyC1PresentProvideAssistanceData,
			ProvideAssistanceData: &lpptype.ProvideAssistanceData{
				CriticalExtensions: lpptype.ProvideAssistanceDataCriticalExtensions{
					Present: 1, // c1
					C1: &lpptype.ProvideAssistanceDataCriticalExtensionsC1{
						Present: 1, // provideAssistanceData-r9
						ProvideAssistanceDataR9: &lpptype.ProvideAssistanceDataR9IEs{
							AGNSSProvideAssistanceData: &lpptype.AGNSSProvideAssistanceData{},
						},
					},
				},
			},
		},
	}

	// §5.2.1: a single delivery carries all the assistance data there is.
	return encodeLPPMessage(transactionID, lpptype.InitiatorLocationServer, true, body)
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
		Present: 1, // c1
		C1: &lpptype.LPPMessageBodyC1{
			Present: lpptype.LPPMessageBodyC1PresentProvideLocationInformation,
			ProvideLocationInformation: &lpptype.ProvideLocationInformation{
				CriticalExtensions: lpptype.ProvideLocationInformationCriticalExtensions{
					Present: 1, // c1
					C1: &lpptype.ProvideLocationInformationCriticalExtensionsC1{
						Present: 1, // provideLocationInformation-r9
						ProvideLocationInformationR9: &lpptype.ProvideLocationInformationR9IEs{
							CommonIEsProvideLocationInformation: &lpptype.CommonIEsProvideLocationInformation{
								LocationEstimate: &lpptype.LocationCoordinates{
									Present: lpptype.LocationCoordinatesPresentEllipsoidPointWithAltitudeAndUncertaintyEllipsoid,
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

	// §5.1.1 step 2: the target's response ends the transaction.
	return encodeLPPMessage(transactionID, lpptype.InitiatorTargetDevice, true, body)
}

// =====================================================================
// Helpers
// =====================================================================

// makeGnssIdBitmap creates a GNSS-ID-Bitmap with the given bit positions set.
// Bit 0 = GPS, 1 = SBAS, 2 = QZSS, 3 = Galileo, 4 = GLONASS, 5 = BDS, 6 = NavIC.
func makeGnssIdBitmap(gnssBits ...int) aper.BitString {
	bs := aper.BitString{
		Bytes:     make([]byte, 1),
		BitLength: gnssIdBitmapBitLength,
	}

	for _, bit := range gnssBits {
		if bit >= 0 && bit < 8 {
			bs.Bytes[0] |= 1 << (7 - bit)
		}
	}

	return bs
}

// makeGnssSignalBitmap creates a GNSS-SignalIDs bitmap (8 bits, MSB = signal 0).
func makeGnssSignalBitmap(signalBits ...int) aper.BitString {
	bs := aper.BitString{
		Bytes:     make([]byte, 1),
		BitLength: gnssSignalIDsBitLength,
	}

	for _, bit := range signalBits {
		if bit >= 0 && bit < 8 {
			bs.Bytes[0] |= 1 << (7 - bit)
		}
	}

	return bs
}

// PositioningModes bitmap bit masks (TS 37.355 §6.4.1).
// Bit 0 = standalone, 1 = ue-based, 2 = ue-assisted.
const (
	posModeStandaloneBit = 0x80
	posModeUeBasedBit    = 0x40
	posModeUeAssistedBit = 0x20

	posModesBitLength      = 3
	gnssIdBitmapBitLength  = 7
	gnssSignalIDsBitLength = 8
)

// makePosModes creates a PositioningModes bitmap.
func makePosModes(standalone, ueBased, ueAssisted bool) aper.BitString {
	bs := aper.BitString{
		Bytes:     make([]byte, 1),
		BitLength: posModesBitLength,
	}

	if standalone {
		bs.Bytes[0] |= posModeStandaloneBit
	}

	if ueBased {
		bs.Bytes[0] |= posModeUeBasedBit
	}

	if ueAssisted {
		bs.Bytes[0] |= posModeUeAssistedBit
	}

	return bs
}

// encodeLatitude converts a signed 1e-7-degree latitude to TS 23.032 encoding.
// Returns (latitudeSign, degreesLatitude) where degreesLatitude is 0..maxDegreesLatitude.
// TS 23.032: latitude = N * 90 / 2^23, so N = lat_deg * 2^23 / 90.
func encodeLatitude(latE7 int32) (aper.Enumerated, int64) {
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
// offset (N + longitudeOffset) to work around an aper library bug with signed
// INTEGERs that have a large negative lower bound.
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
func encodeAltitude(altCm int32) (aper.Enumerated, int64) {
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
func decodeLatitude(sign aper.Enumerated, encoded int64) int32 {
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
func decodeAltitude(dir aper.Enumerated, encoded int64) int32 {
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
