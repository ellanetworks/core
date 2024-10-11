package consumer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/omec-project/openapi/Nnrf_NFDiscovery"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/udm/logger"
)

const (
	NFDiscoveryToUDRParamNone int = iota
	NFDiscoveryToUDRParamSupi
	NFDiscoveryToUDRParamExtGroupId
	NFDiscoveryToUDRParamGpsi
)

func SendNFIntances(nrfUri string, targetNfType, requestNfType models.NfType,
	param Nnrf_NFDiscovery.SearchNFInstancesParamOpts,
) (result models.SearchResult, err error) {
	configuration := Nnrf_NFDiscovery.NewConfiguration()
	configuration.SetBasePath(nrfUri) // addr
	clientNRF := Nnrf_NFDiscovery.NewAPIClient(configuration)

	result, res, err1 := clientNRF.NFInstancesStoreApi.SearchNFInstances(context.TODO(), targetNfType,
		requestNfType, &param)
	if err1 != nil {
		err = err1
		return
	}
	defer func() {
		if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
			logger.Handlelog.Errorf("SearchNFInstances response body cannot close: %+v", rspCloseErr)
		}
	}()

	if res != nil && res.StatusCode == http.StatusTemporaryRedirect {
		err = fmt.Errorf("Temporary Redirect For Non NRF Consumer")
	}
	return
}
