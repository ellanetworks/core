// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

type RatType string

const (
	RatNR               RatType = "NR"
	RatEUTRA            RatType = "EUTRA"
	RatWLAN             RatType = "WLAN"
	RatVirtual          RatType = "VIRTUAL"
	RatNBIoT            RatType = "NBIOT"
	RatWireline         RatType = "WIRELINE"
	RatWirelineCable    RatType = "WIRELINE_CABLE"
	RatWirelineBBF      RatType = "WIRELINE_BBF"
	RatLTEM             RatType = "LTE-M"
	RatNRU              RatType = "NR_U"
	RatEUTRAU           RatType = "EUTRA_U"
	RatTrustedN3GA      RatType = "TRUSTED_N3GA"
	RatTrustedWLAN      RatType = "TRUSTED_WLAN"
	RatUTRA             RatType = "UTRA"
	RatGERA             RatType = "GERA"
	RatNRLEO            RatType = "NR_LEO"
	RatNRMEO            RatType = "NR_MEO"
	RatNRGEO            RatType = "NR_GEO"
	RatNROtherSat       RatType = "NR_OTHER_SAT"
	RatNRRedCap         RatType = "NR_REDCAP"
	RatWBEUTRANLEO      RatType = "WB_E_UTRAN_LEO"
	RatWBEUTRANMEO      RatType = "WB_E_UTRAN_MEO"
	RatWBEUTRANGEO      RatType = "WB_E_UTRAN_GEO"
	RatWBEUTRANOtherSat RatType = "WB_E_UTRAN_OTHERSAT"
	RatNBIoTLEO         RatType = "NB_IOT_LEO"
	RatNBIoTMEO         RatType = "NB_IOT_MEO"
	RatNBIoTGEO         RatType = "NB_IOT_GEO"
	RatNBIoTOtherSat    RatType = "NB_IOT_OTHERSAT"
	RatLTEMLEO          RatType = "LTE_M_LEO"
	RatLTEMMEO          RatType = "LTE_M_MEO"
	RatLTEMGEO          RatType = "LTE_M_GEO"
	RatLTEMOtherSat     RatType = "LTE_M_OTHERSAT"
	RatNREnhancedRedCap RatType = "NR_EREDCAP"
	RatHSPAEvolution    RatType = "HSPA_EVOLUTION"
)

func (r RatType) Known() bool {
	return oneOf(r, RatNR, RatEUTRA, RatWLAN, RatVirtual, RatNBIoT, RatWireline, RatWirelineCable,
		RatWirelineBBF, RatLTEM, RatNRU, RatEUTRAU, RatTrustedN3GA, RatTrustedWLAN, RatUTRA, RatGERA,
		RatNRLEO, RatNRMEO, RatNRGEO, RatNROtherSat, RatNRRedCap, RatWBEUTRANLEO, RatWBEUTRANMEO,
		RatWBEUTRANGEO, RatWBEUTRANOtherSat, RatNBIoTLEO, RatNBIoTMEO, RatNBIoTGEO, RatNBIoTOtherSat,
		RatLTEMLEO, RatLTEMMEO, RatLTEMGEO, RatLTEMOtherSat, RatNREnhancedRedCap, RatHSPAEvolution)
}

type AccessType string

const (
	Access3GPP    AccessType = "3GPP_ACCESS"
	AccessNon3GPP AccessType = "NON_3GPP_ACCESS"
)

func (a AccessType) Known() bool {
	return oneOf(a, Access3GPP, AccessNon3GPP)
}

type URIScheme string

const (
	SchemeHTTP  URIScheme = "http"
	SchemeHTTPS URIScheme = "https"
)

func (s URIScheme) Known() bool {
	return oneOf(s, SchemeHTTP, SchemeHTTPS)
}
