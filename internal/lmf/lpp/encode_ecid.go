// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/lpp/models"
)

func namedBits(set ...bool) []bool {
	n := 1

	for i, b := range set {
		if b {
			n = i + 1
		}
	}

	return append([]bool(nil), set[:n]...)
}

func ecidBits(m models.ECIDMeasurements) []bool {
	return namedBits(m.RSRP, m.RSRQ, m.UERxTx)
}

func nrECIDBits(m models.NRECIDMeasurements) []bool {
	return namedBits(m.SSRSRP, m.SSRSRQ, m.CSIRSRP, m.CSIRSRQ)
}

func EncodeRequestECIDCapabilities(transactionID, sequenceNumber byte, nr bool) ([]byte, error) {
	r9 := &lpptype.RequestCapabilitiesR9IEs{
		ECIDRequestCapabilities: &lpptype.ECIDRequestCapabilities{},
	}

	if nr {
		r9.R16Additions = &lpptype.RequestCapabilitiesR16Additions{
			NRECIDRequestCapabilities: &lpptype.NRECIDRequestCapabilities{},
		}
	}

	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			RequestCapabilities: &lpptype.RequestCapabilities{
				CriticalExtensions: lpptype.RequestCapabilitiesCriticalExtensions{
					C1: &lpptype.RequestCapabilitiesCriticalExtensionsC1{RequestCapabilitiesR9: r9},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorLocationServer, body, false, sequenceNumber)
}

func EncodeRequestECIDLocationInformation(transactionID, sequenceNumber byte, responseTimeSeconds int64, ecid *models.ECIDMeasurements, nr *models.NRECIDMeasurements) ([]byte, error) {
	r9 := &lpptype.RequestLocationInformationR9IEs{
		CommonIEsRequestLocationInformation: &lpptype.CommonIEsRequestLocationInformation{
			LocationInformationType: lpptype.LocationInformationTypeLocationMeasurementsRequired,
			QoS: &lpptype.QoS{
				ResponseTime: &lpptype.ResponseTime{Time: responseTimeSeconds},
			},
		},
	}

	if ecid != nil {
		r9.ECIDRequestLocationInformation = &lpptype.ECIDRequestLocationInformation{RequestedMeasurements: ecidBits(*ecid)}
	}

	if nr != nil {
		r9.R16Additions = &lpptype.RequestLocationInformationR16Additions{
			NRECIDRequestLocationInformation: &lpptype.NRECIDRequestLocationInformation{RequestedMeasurements: nrECIDBits(*nr)},
		}
	}

	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			RequestLocationInformation: &lpptype.RequestLocationInformation{
				CriticalExtensions: lpptype.RequestLocationInformationCriticalExtensions{
					C1: &lpptype.RequestLocationInformationCriticalExtensionsC1{RequestLocationInformationR9: r9},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorLocationServer, body, false, sequenceNumber)
}

func EncodeProvideECIDCapabilities(transactionID, sequenceNumber byte, ecid *models.ECIDMeasurements, nr *models.NRECIDMeasurements) ([]byte, error) {
	r9 := &lpptype.ProvideCapabilitiesR9IEs{}

	if ecid != nil {
		r9.ECIDProvideCapabilities = &lpptype.ECIDProvideCapabilities{ECIDMeasSupported: ecidBits(*ecid)}
	}

	if nr != nil {
		r9.R16Additions = &lpptype.ProvideCapabilitiesR16Additions{
			NRECIDProvideCapabilities: &lpptype.NRECIDProvideCapabilities{NRECIDMeasSupported: nrECIDBits(*nr)},
		}
	}

	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			ProvideCapabilities: &lpptype.ProvideCapabilities{
				CriticalExtensions: lpptype.ProvideCapabilitiesCriticalExtensions{
					C1: &lpptype.ProvideCapabilitiesCriticalExtensionsC1{ProvideCapabilitiesR9: r9},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorLocationServer, body, true, sequenceNumber)
}

func EncodeProvideECIDLocationInformation(transactionID, sequenceNumber byte, ecid *lpptype.ECIDSignalMeasurementInformation, nr *lpptype.NRECIDSignalMeasurementInformation) ([]byte, error) {
	r9 := &lpptype.ProvideLocationInformationR9IEs{}

	if ecid != nil {
		r9.ECIDProvideLocationInformation = &lpptype.ECIDProvideLocationInformation{ECIDSignalMeasurementInformation: ecid}
	}

	if nr != nil {
		r9.R16Additions = &lpptype.ProvideLocationInformationR16Additions{
			NRECIDProvideLocationInformation: &lpptype.NRECIDProvideLocationInformation{NRECIDSignalMeasurementInformation: nr},
		}
	}

	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			ProvideLocationInformation: &lpptype.ProvideLocationInformation{
				CriticalExtensions: lpptype.ProvideLocationInformationCriticalExtensions{
					C1: &lpptype.ProvideLocationInformationCriticalExtensionsC1{ProvideLocationInformationR9: r9},
				},
			},
		},
	}

	return encodeLPPMessage(transactionID, lpptype.InitiatorLocationServer, body, true, sequenceNumber)
}
