package ngap

import (
	"fmt"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func BuildPDUSessionResourceReleaseCommandTransfer() ([]byte, error) {
	resourceReleaseCommandTransfer := ngapType.PDUSessionResourceReleaseCommandTransfer{
		Cause: ngapType.Cause{
			Present: ngapType.CausePresentNas,
			Nas: &ngapType.CauseNas{
				Value: ngapType.CauseNasPresentNormalRelease,
			},
		},
	}

	buf, err := aper.MarshalWithParams(resourceReleaseCommandTransfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not encode pdu session resource release command transfer: %s", err)
	}

	return buf, nil
}
