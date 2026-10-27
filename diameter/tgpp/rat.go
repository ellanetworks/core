// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import "fmt"

type RATType uint32

const (
	RATWLAN                RATType = 0
	RATVirtual             RATType = 1
	RATTrustedN3GA         RATType = 2
	RATWireline            RATType = 3
	RATWirelineCable       RATType = 4
	RATWirelineBBF         RATType = 5
	RATUTRAN               RATType = 1000
	RATGERAN               RATType = 1001
	RATGAN                 RATType = 1002
	RATHSPAEvolution       RATType = 1003
	RATEUTRAN              RATType = 1004
	RATEUTRANNBIoT         RATType = 1005
	RATNR                  RATType = 1006
	RATLTEM                RATType = 1007
	RATNRU                 RATType = 1008
	RATEUTRANLEO           RATType = 1011
	RATEUTRANMEO           RATType = 1012
	RATEUTRANGEO           RATType = 1013
	RATEUTRANOtherSat      RATType = 1014
	RATEUTRANNBIoTLEO      RATType = 1021
	RATEUTRANNBIoTMEO      RATType = 1022
	RATEUTRANNBIoTGEO      RATType = 1023
	RATEUTRANNBIoTOtherSat RATType = 1024
	RATLTEMLEO             RATType = 1031
	RATLTEMMEO             RATType = 1032
	RATLTEMGEO             RATType = 1033
	RATLTEMOtherSat        RATType = 1034
	RATNRLEO               RATType = 1035
	RATNRMEO               RATType = 1036
	RATNRGEO               RATType = 1037
	RATNROtherSat          RATType = 1038
	RATNRRedCap            RATType = 1039
	RATNREnhancedRedCap    RATType = 1040
	RATCDMA20001X          RATType = 2000
	RATHRPD                RATType = 2001
	RATUMB                 RATType = 2002
	RATEHRPD               RATType = 2003
)

var ratTypeNames = map[RATType]string{
	RATWLAN: "WLAN", RATVirtual: "VIRTUAL", RATTrustedN3GA: "TRUSTED-N3GA", RATWireline: "WIRELINE",
	RATWirelineCable: "WIRELINE-CABLE", RATWirelineBBF: "WIRELINE-BBF", RATUTRAN: "UTRAN", RATGERAN: "GERAN", RATGAN: "GAN",
	RATHSPAEvolution: "HSPA_EVOLUTION", RATEUTRAN: "EUTRAN", RATEUTRANNBIoT: "EUTRAN-NB-IoT", RATNR: "NR", RATLTEM: "LTE-M",
	RATNRU: "NR-U", RATEUTRANLEO: "EUTRAN(LEO)", RATEUTRANMEO: "EUTRAN(MEO)", RATEUTRANGEO: "EUTRAN(GEO)",
	RATEUTRANOtherSat: "EUTRAN(OTHERSAT)", RATEUTRANNBIoTLEO: "EUTRAN-NB-IoT(LEO)", RATEUTRANNBIoTMEO: "EUTRAN-NB-IoT(MEO)",
	RATEUTRANNBIoTGEO: "EUTRAN-NB-IoT(GEO)", RATEUTRANNBIoTOtherSat: "EUTRAN-NB-IoT(OTHERSAT)", RATLTEMLEO: "LTE-M(LEO)",
	RATLTEMMEO: "LTE-M(MEO)", RATLTEMGEO: "LTE-M(GEO)", RATLTEMOtherSat: "LTE-M(OTHERSAT)", RATNRLEO: "NR(LEO)",
	RATNRMEO: "NR(MEO)", RATNRGEO: "NR(GEO)", RATNROtherSat: "NR(OTHERSAT)", RATNRRedCap: "NR-REDCAP",
	RATNREnhancedRedCap: "NR-EREDCAP", RATCDMA20001X: "CDMA2000_1X", RATHRPD: "HRPD", RATUMB: "UMB", RATEHRPD: "EHRPD",
}

func (t RATType) String() string {
	if name, ok := ratTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("RATType(%d)", uint32(t))
}
