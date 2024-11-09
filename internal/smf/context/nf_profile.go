package context

import (
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/smf/logger"
)

var NFServices *[]models.NfService

var NfServiceVersion *[]models.NfServiceVersion

var SmfInfo *models.SmfInfo

type SmfSnssaiPlmnIdInfo map[string]models.PlmnId

var SmfPlmnInfo SmfSnssaiPlmnIdInfo

func SmfPlmnConfig() *[]models.PlmnId {
	plmns := make([]models.PlmnId, 0)
	for _, plmn := range SmfPlmnInfo {
		plmns = append(plmns, plmn)
	}

	if len(plmns) > 0 {
		logger.CfgLog.Debugf("plmnId configured [%v] ", plmns)
		return &plmns
	}
	return nil
}
