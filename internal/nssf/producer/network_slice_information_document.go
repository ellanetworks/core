package producer

import (
	"fmt"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/nssf/plugin"
)

// Check if the NF service consumer is authorized
func checkNfServiceConsumer(nfType models.NfType) error {
	if nfType != models.NfType_AMF && nfType != models.NfType_NSSF {
		return fmt.Errorf("`nf-type`:'%s' is not authorized to retrieve the slice selection information", string(nfType))
	}

	return nil
}

func GetNSSelection(param plugin.NsselectionQueryParameter) (*models.AuthorizedNetworkSliceInfo, error) {
	response := &models.AuthorizedNetworkSliceInfo{}
	err := checkNfServiceConsumer(*param.NfType)
	if err != nil {
		return nil, fmt.Errorf("NSSF No Response")
	}
	if param.SliceInfoRequestForRegistration != nil {
		err := nsselectionForRegistration(param, response, nil)
		if err != nil {
			return nil, err
		}
	}

	if param.SliceInfoRequestForPduSession != nil {
		err := nsselectionForPduSession(param, response, nil)
		if err != nil {
			return nil, err
		}
	}

	return response, nil
}
