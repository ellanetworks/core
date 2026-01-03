// Copyright 2024 Ella Networks

package pfcp

import (
	"fmt"

	"github.com/wmnsk/go-pfcp/ie"
)

func FindFTEID(createdPDRIEs []*ie.IE) (*ie.FTEIDFields, error) {
	for _, createdPDRIE := range createdPDRIEs {
		teid, err := createdPDRIE.FTEID()
		if err == nil {
			return teid, nil
		}
	}

	return nil, fmt.Errorf("FTEID not found in CreatedPDR")
}
