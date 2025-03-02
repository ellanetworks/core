package util

import (
	coreModels "github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/openapi/models"
)

// ConvertUeLocation safely converts a UserLocation from models.UserLocation to coreModels.UserLocation.
// It returns nil if the input is nil, and performs nil-checks for each nested field.
func ConvertUeLocation(uL *models.UserLocation) *coreModels.UserLocation {
	if uL == nil {
		return nil
	}

	converted := &coreModels.UserLocation{}

	if uL.EutraLocation != nil {
		converted.EutraLocation = &coreModels.EutraLocation{}

		if uL.EutraLocation.Tai != nil {
			converted.EutraLocation.Tai = &coreModels.Tai{}
			if uL.EutraLocation.Tai.PlmnId != nil {
				converted.EutraLocation.Tai.PlmnId = &coreModels.PlmnId{
					Mcc: uL.EutraLocation.Tai.PlmnId.Mcc,
					Mnc: uL.EutraLocation.Tai.PlmnId.Mnc,
				}
			}
			converted.EutraLocation.Tai.Tac = uL.EutraLocation.Tai.Tac
		}

		if uL.EutraLocation.Ecgi != nil {
			converted.EutraLocation.Ecgi = &coreModels.Ecgi{}
			if uL.EutraLocation.Ecgi.PlmnId != nil {
				converted.EutraLocation.Ecgi.PlmnId = &coreModels.PlmnId{
					Mcc: uL.EutraLocation.Ecgi.PlmnId.Mcc,
					Mnc: uL.EutraLocation.Ecgi.PlmnId.Mnc,
				}
			}
			converted.EutraLocation.Ecgi.EutraCellId = uL.EutraLocation.Ecgi.EutraCellId
		}

		converted.EutraLocation.AgeOfLocationInformation = uL.EutraLocation.AgeOfLocationInformation
		converted.EutraLocation.UeLocationTimestamp = uL.EutraLocation.UeLocationTimestamp
		converted.EutraLocation.GeographicalInformation = uL.EutraLocation.GeographicalInformation
		converted.EutraLocation.GeodeticInformation = uL.EutraLocation.GeodeticInformation

		if uL.EutraLocation.GlobalNgenbId != nil {
			converted.EutraLocation.GlobalNgenbId = &coreModels.GlobalRanNodeId{}
			if uL.EutraLocation.GlobalNgenbId.PlmnId != nil {
				converted.EutraLocation.GlobalNgenbId.PlmnId = &coreModels.PlmnId{
					Mcc: uL.EutraLocation.GlobalNgenbId.PlmnId.Mcc,
					Mnc: uL.EutraLocation.GlobalNgenbId.PlmnId.Mnc,
				}
			}
			converted.EutraLocation.GlobalNgenbId.N3IwfId = uL.EutraLocation.GlobalNgenbId.N3IwfId
			if uL.EutraLocation.GlobalNgenbId.GNbId != nil {
				converted.EutraLocation.GlobalNgenbId.GNbId = &coreModels.GNbId{
					BitLength: uL.EutraLocation.GlobalNgenbId.GNbId.BitLength,
					GNBValue:  uL.EutraLocation.GlobalNgenbId.GNbId.GNBValue,
				}
			}
			converted.EutraLocation.GlobalNgenbId.NgeNbId = uL.EutraLocation.GlobalNgenbId.NgeNbId
		}
	}

	if uL.NrLocation != nil {
		converted.NrLocation = &coreModels.NrLocation{}

		if uL.NrLocation.Tai != nil {
			converted.NrLocation.Tai = &coreModels.Tai{}
			if uL.NrLocation.Tai.PlmnId != nil {
				converted.NrLocation.Tai.PlmnId = &coreModels.PlmnId{
					Mcc: uL.NrLocation.Tai.PlmnId.Mcc,
					Mnc: uL.NrLocation.Tai.PlmnId.Mnc,
				}
			}
			converted.NrLocation.Tai.Tac = uL.NrLocation.Tai.Tac
		}

		if uL.NrLocation.Ncgi != nil {
			converted.NrLocation.Ncgi = &coreModels.Ncgi{}
			if uL.NrLocation.Ncgi.PlmnId != nil {
				converted.NrLocation.Ncgi.PlmnId = &coreModels.PlmnId{
					Mcc: uL.NrLocation.Ncgi.PlmnId.Mcc,
					Mnc: uL.NrLocation.Ncgi.PlmnId.Mnc,
				}
			}
			converted.NrLocation.Ncgi.NrCellId = uL.NrLocation.Ncgi.NrCellId
		}

		converted.NrLocation.AgeOfLocationInformation = uL.NrLocation.AgeOfLocationInformation
		converted.NrLocation.UeLocationTimestamp = uL.NrLocation.UeLocationTimestamp
		converted.NrLocation.GeographicalInformation = uL.NrLocation.GeographicalInformation
		converted.NrLocation.GeodeticInformation = uL.NrLocation.GeodeticInformation

		if uL.NrLocation.GlobalGnbId != nil {
			converted.NrLocation.GlobalGnbId = &coreModels.GlobalRanNodeId{}
			if uL.NrLocation.GlobalGnbId.PlmnId != nil {
				converted.NrLocation.GlobalGnbId.PlmnId = &coreModels.PlmnId{
					Mcc: uL.NrLocation.GlobalGnbId.PlmnId.Mcc,
					Mnc: uL.NrLocation.GlobalGnbId.PlmnId.Mnc,
				}
			}
			converted.NrLocation.GlobalGnbId.N3IwfId = uL.NrLocation.GlobalGnbId.N3IwfId
			if uL.NrLocation.GlobalGnbId.GNbId != nil {
				converted.NrLocation.GlobalGnbId.GNbId = &coreModels.GNbId{
					BitLength: uL.NrLocation.GlobalGnbId.GNbId.BitLength,
					GNBValue:  uL.NrLocation.GlobalGnbId.GNbId.GNBValue,
				}
			}
			converted.NrLocation.GlobalGnbId.NgeNbId = uL.NrLocation.GlobalGnbId.NgeNbId
		}
	}

	return converted
}
