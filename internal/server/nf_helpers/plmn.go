package nf_helpers

import (
	"context"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/db/sql"
)

type SupportedNssaiInPlmn struct {
	PlmnId              *models.PlmnId  `yaml:"plmnId"`
	SupportedSnssaiList []models.Snssai `yaml:"supportedSnssaiList"`
}

func GetSupportedNssaiInPlmnList(queries *sql.Queries) ([]SupportedNssaiInPlmn, error) {
	networkSlices, err := queries.ListNetworkSlices(context.Background())
	if err != nil {
		return nil, err
	}
	supportedNssaiInPlmnList := make([]SupportedNssaiInPlmn, 0, len(networkSlices))
	for _, networkSlice := range networkSlices {
		nssai := models.Snssai{
			Sst: int32(networkSlice.Sst),
			Sd:  networkSlice.Sd,
		}
		supportedNssaiInPlmn := SupportedNssaiInPlmn{
			PlmnId:              &models.PlmnId{Mnc: networkSlice.Mnc, Mcc: networkSlice.Mcc},
			SupportedSnssaiList: []models.Snssai{nssai},
		}
		supportedNssaiInPlmnList = append(supportedNssaiInPlmnList, supportedNssaiInPlmn)
	}
	return supportedNssaiInPlmnList, nil
}
