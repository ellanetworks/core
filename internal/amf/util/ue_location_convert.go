package util

import (
	coreModels "github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/openapi/models"
)

// ConvertUeLocation is a function that converts a UserLocation from models.UserLocation to coreModels.UserLocation
// This method should be deleted when we fully migrate to the new models
func ConvertUeLocation(uL *models.UserLocation) *coreModels.UserLocation {
	return &coreModels.UserLocation{
		EutraLocation: &coreModels.EutraLocation{
			Tai: &coreModels.Tai{
				PlmnId: &coreModels.PlmnId{
					Mcc: uL.EutraLocation.Tai.PlmnId.Mcc,
					Mnc: uL.EutraLocation.Tai.PlmnId.Mnc,
				},
				Tac: uL.EutraLocation.Tai.Tac,
			},
			Ecgi: &coreModels.Ecgi{
				PlmnId: &coreModels.PlmnId{
					Mcc: uL.EutraLocation.Ecgi.PlmnId.Mcc,
					Mnc: uL.EutraLocation.Ecgi.PlmnId.Mnc,
				},
				EutraCellId: uL.EutraLocation.Ecgi.EutraCellId,
			},
			AgeOfLocationInformation: uL.EutraLocation.AgeOfLocationInformation,
			UeLocationTimestamp:      uL.EutraLocation.UeLocationTimestamp,
			GeographicalInformation:  uL.EutraLocation.GeographicalInformation,
			GeodeticInformation:      uL.EutraLocation.GeodeticInformation,
			GlobalNgenbId: &coreModels.GlobalRanNodeId{
				PlmnId: &coreModels.PlmnId{
					Mcc: uL.EutraLocation.GlobalNgenbId.PlmnId.Mcc,
					Mnc: uL.EutraLocation.GlobalNgenbId.PlmnId.Mnc,
				},
				N3IwfId: uL.EutraLocation.GlobalNgenbId.N3IwfId,
				GNbId: &coreModels.GNbId{
					BitLength: uL.EutraLocation.GlobalNgenbId.GNbId.BitLength,
					GNBValue:  uL.EutraLocation.GlobalNgenbId.GNbId.GNBValue,
				},
				NgeNbId: uL.EutraLocation.GlobalNgenbId.NgeNbId,
			},
		},
		NrLocation: &coreModels.NrLocation{
			Tai: &coreModels.Tai{
				PlmnId: &coreModels.PlmnId{
					Mcc: uL.NrLocation.Tai.PlmnId.Mcc,
					Mnc: uL.NrLocation.Tai.PlmnId.Mnc,
				},
				Tac: uL.NrLocation.Tai.Tac,
			},
			Ncgi: &coreModels.Ncgi{
				PlmnId: &coreModels.PlmnId{
					Mcc: uL.NrLocation.Ncgi.PlmnId.Mcc,
					Mnc: uL.NrLocation.Ncgi.PlmnId.Mnc,
				},
				NrCellId: uL.NrLocation.Ncgi.NrCellId,
			},
			AgeOfLocationInformation: uL.NrLocation.AgeOfLocationInformation,
			UeLocationTimestamp:      uL.NrLocation.UeLocationTimestamp,
			GeographicalInformation:  uL.NrLocation.GeographicalInformation,
			GeodeticInformation:      uL.NrLocation.GeodeticInformation,
			GlobalGnbId: &coreModels.GlobalRanNodeId{
				PlmnId: &coreModels.PlmnId{
					Mcc: uL.NrLocation.GlobalGnbId.PlmnId.Mcc,
					Mnc: uL.NrLocation.GlobalGnbId.PlmnId.Mnc,
				},
				N3IwfId: uL.NrLocation.GlobalGnbId.N3IwfId,
				GNbId: &coreModels.GNbId{
					BitLength: uL.NrLocation.GlobalGnbId.GNbId.BitLength,
					GNBValue:  uL.NrLocation.GlobalGnbId.GNbId.GNBValue,
				},
				NgeNbId: uL.NrLocation.GlobalGnbId.NgeNbId,
			},
		},
	}
}
